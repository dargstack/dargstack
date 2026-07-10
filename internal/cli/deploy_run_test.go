package cli

import "testing"

func TestUniqueSortedDomains(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: nil,
		},
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: []string{},
		},
		{
			name:     "a only",
			a:        []string{"b.com", "a.com"},
			b:        nil,
			expected: []string{"a.com", "b.com"},
		},
		{
			name:     "b only",
			a:        nil,
			b:        []string{"z.com", "a.com"},
			expected: []string{"a.com", "z.com"},
		},
		{
			name:     "merge no overlap",
			a:        []string{"a.com"},
			b:        []string{"b.com"},
			expected: []string{"a.com", "b.com"},
		},
		{
			name:     "merge with overlap",
			a:        []string{"a.com", "b.com"},
			b:        []string{"b.com", "c.com"},
			expected: []string{"a.com", "b.com", "c.com"},
		},
		{
			name:     "empty strings filtered",
			a:        []string{"a.com", ""},
			b:        []string{"", "b.com"},
			expected: []string{"a.com", "b.com"},
		},
		{
			name:     "all empty strings",
			a:        []string{"", ""},
			b:        []string{""},
			expected: []string{},
		},
		{
			name:     "identical slices",
			a:        []string{"x.com", "y.com"},
			b:        []string{"x.com", "y.com"},
			expected: []string{"x.com", "y.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueSortedDomains(tt.a, tt.b)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d elements, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestExtractBuildServices(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name:     "no services",
			yaml:     "version: '3'",
			expected: nil,
		},
		{
			name: "services without build label",
			yaml: `
services:
  web:
    image: nginx
  api:
    image: node
`,
			expected: nil,
		},
		{
			name: "single service with build label",
			yaml: `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.build: "./"
`,
			expected: []string{"web"},
		},
		{
			name: "multiple services mixed",
			yaml: `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.build: "./"
  api:
    image: node
  worker:
    image: python
    deploy:
      labels:
        dargstack.development.build: "../docker"
`,
			expected: []string{"web", "worker"},
		},
		{
			name: "labels as list format",
			yaml: `
services:
  app:
    image: ruby
    deploy:
      labels:
        - dargstack.development.build=./app
`,
			expected: []string{"app"},
		},
		{
			name:     "invalid yaml",
			yaml:     "{{invalid",
			expected: nil,
		},
		{
			name:     "empty yaml",
			yaml:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBuildServices([]byte(tt.yaml))
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestCountComposeServices(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected int
	}{
		{
			name:     "no services key",
			yaml:     "version: '3'",
			expected: 0,
		},
		{
			name: "single service",
			yaml: `
services:
  web:
    image: nginx
`,
			expected: 1,
		},
		{
			name: "multiple services",
			yaml: `
services:
  web:
    image: nginx
  api:
    image: node
  worker:
    image: python
`,
			expected: 3,
		},
		{
			name:     "empty services",
			yaml:     "services: {}",
			expected: 0,
		},
		{
			name:     "invalid yaml",
			yaml:     "{{invalid",
			expected: 0,
		},
		{
			name:     "empty yaml",
			yaml:     "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countComposeServices([]byte(tt.yaml))
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
