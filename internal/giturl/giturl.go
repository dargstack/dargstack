package giturl

import (
	"fmt"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

// GitURL holds SSH and HTTPS clone URLs for a service's repository.
type GitURL struct {
	SSH   string
	HTTPS string
}

// Primary returns the preferred clone URL (SSH if set, else HTTPS).
func (g GitURL) Primary() string {
	if g.SSH != "" {
		return g.SSH
	}
	return g.HTTPS
}

// Fallback returns the alternate clone URL, or "" if none is set.
func (g GitURL) Fallback() string {
	if g.SSH != "" && g.HTTPS != "" {
		return g.HTTPS
	}
	return ""
}

// IsSet returns true if at least one URL is configured.
func (g GitURL) IsSet() bool {
	return g.SSH != "" || g.HTTPS != ""
}

// RepoNameFromURL extracts the repository directory name from a git URL.
// It handles SSH (git@host:user/repo.git), HTTPS (https://host/user/repo.git),
// and git:// formats. The .git suffix is stripped if present.
func RepoNameFromURL(url string) string {
	name := url
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".git")
	return name
}

const (
	labelGit      = "dargstack.development.git"
	labelGitSSH   = "dargstack.development.git.ssh"
	labelGitHTTPS = "dargstack.development.git.https"
)

func extractLabel(svc map[string]interface{}, key string) string {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return ""
	}
	labels, ok := deploy["labels"]
	if !ok {
		return ""
	}
	switch v := labels.(type) {
	case map[string]interface{}:
		if val, ok := v[key].(string); ok {
			return val
		}
	case []interface{}:
		prefix := key + "="
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(s, prefix) {
				return strings.TrimPrefix(s, prefix)
			}
		}
	}
	return ""
}

func ExtractFromService(svc map[string]interface{}, serviceName string) GitURL {
	legacyGit := extractLabel(svc, labelGit)
	sshGit := extractLabel(svc, labelGitSSH)
	httpsGit := extractLabel(svc, labelGitHTTPS)

	var g GitURL

	if sshGit != "" {
		g.SSH = sshGit
	} else if legacyGit != "" {
		g.SSH = legacyGit
	}

	if httpsGit != "" {
		g.HTTPS = httpsGit
	}

	if legacyGit != "" && serviceName != "" {
		logger.L.Warn(fmt.Sprintf("service %q: dargstack.development.git is deprecated, use dargstack.development.git.ssh and dargstack.development.git.https", serviceName))
	}

	return g
}
