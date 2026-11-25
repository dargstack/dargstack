package docker

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// PullTarget is one image that must be pullable on every node matching Constraints.
type PullTarget struct {
	Image       string
	Constraints []string
	Services    []string
}

var (
	pullPollInterval = 2 * time.Second
	// pullNoTaskGrace is how long a job may stay without tasks before no node is assumed to match its constraints.
	pullNoTaskGrace = 15 * time.Second
)

// maxServiceNameLen is Swarm's limit for service names.
const maxServiceNameLen = 63

const pullLabel = "dargstack.pull"

func pullJobName(stack string, i int) string {
	suffix := fmt.Sprintf("_pull-%d", i)
	if len(stack)+len(suffix) > maxServiceNameLen {
		stack = stack[:maxServiceNameLen-len(suffix)]
	}
	return stack + suffix
}

// jobSpec is one global-job service, with createArgs holding everything after `service create`.
type jobSpec struct {
	name       string
	createArgs []string
}

type jobTask struct {
	node, state, errMsg string
}

// jobResult is what a job's tasks ended up as.
// A job that is neither noTasks nor timedOut has only terminal tasks.
type jobResult struct {
	tasks    []jobTask
	noTasks  bool
	timedOut bool
	// running lists the nodes whose task was not terminal when the job timed out.
	running []string
	created time.Time
	done    bool
}

// runJobs creates the jobs, waits until each has only terminal tasks, no task after pullNoTaskGrace, or the timeout hits, and removes them again.
// Jobs carrying label=stack left over from an interrupted run are removed first, as they would collide on the names.
func runJobs(exec *Executor, label, stack string, specs []jobSpec, timeout time.Duration) (results []*jobResult, err error) {
	kind := strings.TrimPrefix(label, "dargstack.")
	filter := "label=" + label + "=" + stack
	if out, lsErr := exec.Run("service", "ls", "--filter", filter, "-q"); lsErr == nil && out != "" {
		if _, rmErr := exec.Run(append([]string{"service", "rm"}, strings.Fields(out)...)...); rmErr != nil {
			return nil, fmt.Errorf("remove leftover %s jobs: %w", kind, rmErr)
		}
	}

	// Creating jobs one by one made each wait for the previous registry round trip. The leftover check above
	// already ran any sudo prompt, so the parallel runs below cannot contend for stdin.
	results = make([]*jobResult, len(specs))
	createErrs := make([]error, len(specs))
	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Go(func() {
			args := append([]string{"service", "create", "--detach", "--quiet", "--name", spec.name, "--label", label + "=" + stack}, spec.createArgs...)
			_, createErrs[i] = exec.Run(args...)
			results[i] = &jobResult{created: time.Now()}
		})
	}
	wg.Wait()

	var created []string
	for i, spec := range specs {
		if createErrs[i] == nil {
			created = append(created, spec.name)
		}
	}
	defer func() {
		if len(created) > 0 {
			_, _ = exec.Run(append([]string{"service", "rm"}, created...)...)
		}
	}()
	for i, spec := range specs {
		if createErrs[i] != nil {
			return nil, fmt.Errorf("create %s job %s: %w", kind, spec.name, createErrs[i])
		}
	}

	deadline := time.Now().Add(timeout)
	pending := len(specs)
	for {
		for i, spec := range specs {
			r := results[i]
			if r.done {
				continue
			}
			out, psErr := exec.Run("service", "ps", spec.name, "--no-trunc", "--format", "{{.Node}}\t{{.CurrentState}}\t{{.Error}}")
			if psErr != nil {
				return nil, fmt.Errorf("inspect %s job %s: %w", kind, spec.name, psErr)
			}
			r.tasks, r.running = parseTasks(out)
			switch {
			case len(r.tasks) == 0:
				if time.Since(r.created) >= pullNoTaskGrace {
					r.noTasks, r.done = true, true
				}
			case len(r.running) == 0:
				r.done = true
			}
			if r.done {
				pending--
			}
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			for _, r := range results {
				r.timedOut = !r.done
			}
			break
		}
		time.Sleep(pullPollInterval)
	}
	return results, nil
}

// parseTasks parses `service ps` output into tasks, with the state reduced to its lowercase first word.
// running lists the nodes whose task is not terminal yet.
func parseTasks(out string) (tasks []jobTask, running []string) {
	if out == "" {
		return nil, nil
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		node, state, errMsg := parts[0], strings.Fields(parts[1]), strings.TrimSpace(parts[2])
		word := ""
		if len(state) > 0 {
			word = strings.ToLower(state[0])
		}
		switch word {
		case "complete", "failed", "rejected", "shutdown", "orphaned", "remove":
		default:
			running = append(running, node)
		}
		tasks = append(tasks, jobTask{node: node, state: word, errMsg: errMsg})
	}
	return tasks, running
}

// PullOnNodes makes every swarm node eligible for a target pull its image, by running a one-off global-job service per target.
// A node that cannot pull makes Swarm reject the task while preparing it, which is reported as a failure.
// Pulling through jobs lets the check run before `docker stack deploy` stops any running task.
func PullOnNodes(exec *Executor, stack string, targets []PullTarget, timeout time.Duration) (failures []string, err error) {
	if len(targets) == 0 {
		return nil, nil
	}

	specs := make([]jobSpec, len(targets))
	for i, t := range targets {
		args := []string{
			"--mode", "global-job",
			"--restart-condition", "none",
			"--with-registry-auth",
			"--entrypoint", "true",
		}
		// A digest-pinned reference needs no tag lookup at the registry, and nodes that already hold the digest
		// skip the pull for it, which is what keeps repeated deploys fast.
		if strings.Contains(t.Image, "@") {
			args = append(args, "--no-resolve-image")
		}
		for _, c := range t.Constraints {
			args = append(args, "--constraint", c)
		}
		specs[i] = jobSpec{name: pullJobName(stack, i), createArgs: append(args, t.Image)}
	}

	results, err := runJobs(exec, pullLabel, stack, specs, timeout)
	if err != nil {
		return nil, err
	}

	for i, r := range results {
		t := targets[i]
		services := strings.Join(t.Services, ", ")
		switch {
		case r.noTasks:
			failures = append(failures, fmt.Sprintf("no node satisfies the placement constraints of %s", services))
		case r.timedOut:
			failures = append(failures, fmt.Sprintf("timed out pulling image %s for %s on node(s): %s",
				t.Image, services, strings.Join(r.running, ", ")))
		default:
			for _, task := range r.tasks {
				switch task.state {
				case "rejected":
					failures = append(failures, fmt.Sprintf("image %s for %s is not available on node %s: %s", t.Image, services, task.node, task.errMsg))
				case "shutdown", "orphaned", "remove":
					failures = append(failures, fmt.Sprintf("image %s for %s: pull task on node %s ended as %s: %s", t.Image, services, task.node, task.state, task.errMsg))
				}
				// Failed after a successful pull happens for images without a `true` binary.
			}
		}
	}

	sort.Strings(failures)
	return failures, nil
}
