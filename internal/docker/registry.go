package docker

// CheckImagesAccessible probes the registry for each image by inspecting its
// manifest without pulling any layers. Returns a map from image name to error
// for every image that is not reachable (wrong credentials, missing tag, etc.).
// Checks run sequentially to avoid concurrent sudo credential prompts.
func CheckImagesAccessible(exec *Executor, images []string) map[string]error {
	failed := make(map[string]error)
	for _, img := range images {
		if _, err := exec.Run("manifest", "inspect", img); err != nil {
			failed[img] = err
		}
	}
	return failed
}
