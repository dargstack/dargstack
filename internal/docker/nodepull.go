package docker

import (
	"fmt"
	"sort"
	"strings"
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

type pullJob struct {
	target  PullTarget
	name    string
	created time.Time
	done    bool
}

func pullJobName(stack string, i int) string {
	suffix := fmt.Sprintf("_pull-%d", i)
	if len(stack)+len(suffix) > maxServiceNameLen {
		stack = stack[:maxServiceNameLen-len(suffix)]
	}
	return stack + suffix
}

// PullOnNodes makes every swarm node eligible for a target pull its image, by running a one-off global-job service per target.
// A node that cannot pull makes Swarm reject the task while preparing it, which is reported as a failure.
// Pulling through jobs lets the check run before `docker stack deploy` stops any running task.
func PullOnNodes(exec *Executor, stack string, targets []PullTarget, timeout time.Duration) (failures []string, err error) {
	if len(targets) == 0 {
		return nil, nil
	}

	// Leftovers of an interrupted run would collide on the job names.
	filter := "label=" + pullLabel + "=" + stack
	if out, lsErr := exec.Run("service", "ls", "--filter", filter, "-q"); lsErr == nil && out != "" {
		if _, rmErr := exec.Run(append([]string{"service", "rm"}, strings.Fields(out)...)...); rmErr != nil {
			return nil, fmt.Errorf("remove leftover pull jobs: %w", rmErr)
		}
	}

	var jobs []*pullJob
	defer func() {
		for _, j := range jobs {
			_, _ = exec.Run("service", "rm", j.name)
		}
	}()

	for i, t := range targets {
		name := pullJobName(stack, i)
		args := []string{
			"service", "create", "--detach", "--quiet",
			"--name", name,
			"--label", pullLabel + "=" + stack,
			"--mode", "global-job",
			"--restart-condition", "none",
			"--with-registry-auth",
			"--entrypoint", "true",
		}
		for _, c := range t.Constraints {
			args = append(args, "--constraint", c)
		}
		args = append(args, t.Image)
		if _, err := exec.Run(args...); err != nil {
			return nil, fmt.Errorf("create pull job for %s: %w", t.Image, err)
		}
		jobs = append(jobs, &pullJob{target: t, name: name, created: time.Now()})
	}

	deadline := time.Now().Add(timeout)
	pending := len(jobs)
	notTerminal := make(map[*pullJob][]string)
	for {
		for _, j := range jobs {
			if j.done {
				continue
			}
			out, psErr := exec.Run("service", "ps", j.name, "--no-trunc", "--format", "{{.Node}}\t{{.CurrentState}}\t{{.Error}}")
			if psErr != nil {
				return nil, fmt.Errorf("inspect pull job %s: %w", j.name, psErr)
			}
			done, jobFailures, running := evaluatePullTasks(j, out)
			if done {
				j.done = true
				pending--
				failures = append(failures, jobFailures...)
			}
			notTerminal[j] = running
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			for _, j := range jobs {
				if j.done {
					continue
				}
				failures = append(failures, fmt.Sprintf("timed out pulling image %s for %s on node(s): %s",
					j.target.Image, strings.Join(j.target.Services, ", "), strings.Join(notTerminal[j], ", ")))
			}
			break
		}
		time.Sleep(pullPollInterval)
	}

	sort.Strings(failures)
	return failures, nil
}

// evaluatePullTasks classifies the `service ps` output of one job.
// running lists the nodes whose task is not terminal yet.
func evaluatePullTasks(j *pullJob, out string) (done bool, failures, running []string) {
	services := strings.Join(j.target.Services, ", ")
	if out == "" {
		if time.Since(j.created) < pullNoTaskGrace {
			return false, nil, nil
		}
		return true, []string{fmt.Sprintf("no node satisfies the placement constraints of %s", services)}, nil
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
		case "complete", "failed":
			// Failed after a successful pull happens for images without a `true` binary.
		case "rejected":
			failures = append(failures, fmt.Sprintf("image %s for %s is not available on node %s: %s", j.target.Image, services, node, errMsg))
		case "shutdown", "orphaned", "remove":
			failures = append(failures, fmt.Sprintf("image %s for %s: pull task on node %s ended as %s: %s", j.target.Image, services, node, word, errMsg))
		default:
			running = append(running, node)
		}
	}
	if len(running) > 0 {
		return false, nil, running
	}
	return true, failures, nil
}
