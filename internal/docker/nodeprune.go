package docker

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	pruneLabel = "dargstack.prune"
	pruneImage = "docker:cli"
)

// PruneOtherNodes removes stopped containers and unused images on every ready swarm node except the one dargstack talks to.
// It does nothing when that node is no swarm manager, as `docker node ls` fails there.
func PruneOtherNodes(exec *Executor, stack string, timeout time.Duration) (pruned []string, failures []string, err error) {
	out, lsErr := exec.Run("node", "ls", "--format", "{{.ID}}\t{{.Self}}\t{{.Status}}\t{{.Availability}}")
	if lsErr != nil {
		return nil, nil, nil
	}

	var selfID string
	others := 0
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			continue
		}
		switch {
		case f[1] == "true":
			selfID = f[0]
		case f[2] == "Ready" && f[3] == "Active":
			others++
		}
	}
	if others == 0 {
		return nil, nil, nil
	}

	name := stack + "_prune"
	if len(name) > maxServiceNameLen {
		name = name[:maxServiceNameLen]
	}
	args := []string{
		"--mode", "global-job",
		"--restart-condition", "none",
		"--mount", "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
		"--entrypoint", "sh",
	}
	if selfID != "" {
		args = append(args, "--constraint", "node.id!="+selfID)
	}
	args = append(args, pruneImage, "-c", "docker container prune -f && docker image prune -af")

	results, err := runJobs(exec, pruneLabel, stack, []jobSpec{{name: name, createArgs: args}}, timeout)
	if err != nil {
		return nil, nil, err
	}

	r := results[0]
	switch {
	case r.noTasks:
		failures = append(failures, "no node took the prune job")
	case r.timedOut:
		failures = append(failures, "timed out pruning on node(s): "+strings.Join(r.running, ", "))
	default:
		for _, task := range r.tasks {
			if task.state == "complete" {
				pruned = append(pruned, task.node)
			} else {
				failures = append(failures, fmt.Sprintf("prune on node %s ended as %s: %s", task.node, task.state, task.errMsg))
			}
		}
	}
	sort.Strings(pruned)
	sort.Strings(failures)
	return pruned, failures, nil
}
