package docker

import "sync"

// CheckImagesAccessible probes the registry for each image by inspecting its
// manifest without pulling any layers. Returns a map from image name to
// error for every image that is not reachable (wrong credentials, missing
// tag, etc.).
//
// The first check runs synchronously before the rest are fanned out in
// parallel. If sudo credentials have expired, Executor.Run's refresh may
// prompt interactively; running one check first ensures that prompt happens
// once, rather than from multiple goroutines contending for stdin.
func CheckImagesAccessible(exec *Executor, images []string) map[string]error {
	failed := make(map[string]error)
	if len(images) == 0 {
		return failed
	}

	first, rest := images[0], images[1:]
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
