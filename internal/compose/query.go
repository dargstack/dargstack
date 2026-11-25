package compose

import (
	"sort"

	"go.yaml.in/yaml/v4"
)

// ExtractServiceImages returns the unique sorted list of image names referenced by all services in the compose document.
// Services without an image field (e.g. build-only services) are omitted.
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

// ServicePlacement is a service's image together with its Swarm placement constraints.
type ServicePlacement struct {
	Service     string
	Image       string
	Constraints []string
}

// ExtractServicePlacements returns the image and deploy.placement.constraints of every service that has an image, sorted by service name.
// Constraints are sorted so that equal sets compare equal.
func ExtractServicePlacements(data []byte) []ServicePlacement {
	var doc struct {
		Services map[string]struct {
			Image  string `yaml:"image"`
			Deploy struct {
				Placement struct {
					Constraints []string `yaml:"constraints"`
				} `yaml:"placement"`
			} `yaml:"deploy"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var out []ServicePlacement
	for name, svc := range doc.Services {
		if svc.Image == "" {
			continue
		}
		constraints := append([]string(nil), svc.Deploy.Placement.Constraints...)
		sort.Strings(constraints)
		out = append(out, ServicePlacement{Service: name, Image: svc.Image, Constraints: constraints})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}
