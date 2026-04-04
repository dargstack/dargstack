package compose

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// ExtractServiceImages returns the unique sorted list of image names referenced
// by all services in the compose document. Services without an image field (e.g.
// build-only services) are omitted.
func ExtractServiceImages(data []byte) []string {
	var doc struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var images []string
	for _, svc := range doc.Services {
		if svc.Image != "" && !seen[svc.Image] {
			seen[svc.Image] = true
			images = append(images, svc.Image)
		}
	}
	sort.Strings(images)
	return images
}
