package docker

import (
	"fmt"
	"strings"
)

// PruneImagesKeeping removes every image that no container uses and that is not in keep, returning how many were removed.
// Unlike `docker image prune -af` it spares the images of a stack's services on nodes where no task runs, so a deploy's pre-pull stays effective.
// Removal failures (such as images a child image depends on) are ignored; err is only set when listing fails.
func PruneImagesKeeping(exec *Executor, keep []string) (removed int, err error) {
	protected := make(map[string]bool)
	for _, ref := range keep {
		if id, inspectErr := exec.Run("image", "inspect", "--format", "{{.Id}}", ref); inspectErr == nil && id != "" {
			protected[id] = true
		}
	}

	ids, err := exec.Run("ps", "-aq")
	if err != nil {
		return 0, fmt.Errorf("list containers: %w", err)
	}
	if containers := strings.Fields(ids); len(containers) > 0 {
		out, inspectErr := exec.Run(append([]string{"inspect", "--format", "{{.Image}}"}, containers...)...)
		if inspectErr != nil {
			return 0, fmt.Errorf("inspect containers: %w", inspectErr)
		}
		for _, id := range strings.Fields(out) {
			protected[id] = true
		}
	}

	out, err := exec.Run("image", "ls", "-aq", "--no-trunc")
	if err != nil {
		return 0, fmt.Errorf("list images: %w", err)
	}
	seen := make(map[string]bool)
	for _, id := range strings.Fields(out) {
		if protected[id] || seen[id] {
			continue
		}
		seen[id] = true
		if _, rmErr := exec.Run("image", "rm", "-f", id); rmErr == nil {
			removed++
		}
	}
	return removed, nil
}
