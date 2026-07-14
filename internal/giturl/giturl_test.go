package giturl

import "testing"

func TestGitURLPrimary(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   GitURL
		expected string
	}{
		{"ssh only", GitURL{SSH: "git@gh.com:a/b.git"}, "git@gh.com:a/b.git"},
		{"https only", GitURL{HTTPS: "https://gh.com/a/b.git"}, "https://gh.com/a/b.git"},
		{"both prefers ssh", GitURL{SSH: "git@gh.com:a/b.git", HTTPS: "https://gh.com/a/b.git"}, "git@gh.com:a/b.git"},
		{"empty", GitURL{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gitURL.Primary(); got != tt.expected {
				t.Errorf("Primary() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGitURLFallback(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   GitURL
		expected string
	}{
		{"ssh only", GitURL{SSH: "git@gh.com:a/b.git"}, ""},
		{"https only", GitURL{HTTPS: "https://gh.com/a/b.git"}, ""},
		{"both returns https", GitURL{SSH: "git@gh.com:a/b.git", HTTPS: "https://gh.com/a/b.git"}, "https://gh.com/a/b.git"},
		{"empty", GitURL{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gitURL.Fallback(); got != tt.expected {
				t.Errorf("Fallback() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGitURLIsSet(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   GitURL
		expected bool
	}{
		{"ssh only", GitURL{SSH: "git@gh.com:a/b.git"}, true},
		{"https only", GitURL{HTTPS: "https://gh.com/a/b.git"}, true},
		{"both", GitURL{SSH: "git@gh.com:a/b.git", HTTPS: "https://gh.com/a/b.git"}, true},
		{"empty", GitURL{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gitURL.IsSet(); got != tt.expected {
				t.Errorf("IsSet() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"ssh format", "git@github.com:organization/repository.git", "repository"},
		{"ssh format no .git", "git@github.com:organization/repository", "repository"},
		{"https format", "https://github.com/organization/repository.git", "repository"},
		{"https format no .git", "https://github.com/organization/repository", "repository"},
		{"git protocol", "git://github.com/organization/repository.git", "repository"},
		{"bitbucket ssh", "git@bitbucket.org:team/project-repo.git", "project-repo"},
		{"self-hosted with port", "git@192.168.1.1:22/path/to/repo.git", "repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RepoNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("RepoNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestSSHAgentAvailable_NoEnvVar(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	if SSHAgentAvailable() {
		t.Error("SSHAgentAvailable() should be false when SSH_AUTH_SOCK is not set")
	}
}

func TestExtractFromService(t *testing.T) {
	tests := []struct {
		name          string
		svc           map[string]interface{}
		serviceName   string
		expectedSSH   string
		expectedHTTPS string
	}{
		{
			name: "legacy git label only",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git": "git@github.com:org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "",
		},
		{
			name: "explicit ssh and https",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git.ssh":   "git@github.com:org/repo.git",
						"dargstack.development.git.https": "https://github.com/org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "https://github.com/org/repo.git",
		},
		{
			name: "legacy git with https fallback",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git":       "git@github.com:org/repo.git",
						"dargstack.development.git.https": "https://github.com/org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "https://github.com/org/repo.git",
		},
		{
			name: "all three set - explicit wins over legacy",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git":       "git@github.com:org/legacy.git",
						"dargstack.development.git.ssh":   "git@github.com:org/repo.git",
						"dargstack.development.git.https": "https://github.com/org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "https://github.com/org/repo.git",
		},
		{
			name: "ssh only",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git.ssh": "git@github.com:org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "",
		},
		{
			name: "https only",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git.https": "https://github.com/org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "",
			expectedHTTPS: "https://github.com/org/repo.git",
		},
		{
			name: "labels as list format",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						"dargstack.development.git.ssh=git@github.com:org/repo.git",
						"dargstack.development.git.https=https://github.com/org/repo.git",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "git@github.com:org/repo.git",
			expectedHTTPS: "https://github.com/org/repo.git",
		},
		{
			name: "no git labels",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"other.label": "value",
					},
				},
			},
			serviceName:   "web",
			expectedSSH:   "",
			expectedHTTPS: "",
		},
		{
			name:          "no deploy key",
			svc:           map[string]interface{}{"image": "nginx"},
			serviceName:   "web",
			expectedSSH:   "",
			expectedHTTPS: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFromService(tt.svc, tt.serviceName)
			if result.SSH != tt.expectedSSH {
				t.Errorf("SSH = %q, want %q", result.SSH, tt.expectedSSH)
			}
			if result.HTTPS != tt.expectedHTTPS {
				t.Errorf("HTTPS = %q, want %q", result.HTTPS, tt.expectedHTTPS)
			}
		})
	}
}
