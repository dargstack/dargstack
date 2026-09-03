package docker

import "sync"

// CheckImagesAccessible probes the registry for each image by inspecting its manifest without pulling any layers.
// Images already present locally are treated as accessible without any registry call: Swarm's default pull policy only pulls an image when it is missing locally, and a manifest inspect counts against a registry's pull rate limit the same as an actual pull, so skipping it for images we already have avoids needless rate-limit pressure.
// Returns a map from image name to error for every image that is not reachable (wrong credentials, missing tag, etc.).
//
// The first registry check runs synchronously before the rest are fanned out in parallel.
// If sudo credentials have expired, Executor.Run's refresh may prompt interactively; running one check first ensures that prompt happens once, rather than from multiple goroutines contending for stdin.
func CheckImagesAccessible(exec *Executor, images []string) map[string]error {
	failed := make(map[string]error)
	if len(images) == 0 {
		return failed
	}

	var toCheck []string
	for _, img := range images {
		if !ImageExistsLocally(exec, img) {
			toCheck = append(toCheck, img)
		}
	}
	if len(toCheck) == 0 {
		return failed
	}

	first, rest := toCheck[0], toCheck[1:]
	if _, err := exec.Run("manifest", "inspect", first); err != nil {
		failed[first] = err
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, img := range rest {
		wg.Add(1)
		go func(img string) {
			defer wg.Done()
			if _, err := exec.Run("manifest", "inspect", img); err != nil {
				mu.Lock()
				failed[img] = err
				mu.Unlock()
			}
		}(img)
	}
	wg.Wait()

	return failed
}

// ImageExistsLocally reports whether an image is already present in the local Docker image store.
func ImageExistsLocally(exec *Executor, image string) bool {
	_, err := exec.Run("image", "inspect", "--format", "{{.ID}}", image)
	return err == nil
}
