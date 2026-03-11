package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AuditLogDir returns the path to the audit trail directory within a stack.
func AuditLogDir(stackDir string) string {
	return filepath.Join(stackDir, "artifacts", "audit-log")
}

// SaveDeployment saves a deployment snapshot to the audit-log directory.
// Returns the path to the saved file.
func SaveDeployment(auditDir, env string, composeData []byte) (string, error) {
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		return "", fmt.Errorf("create audit-log directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102T150405.000Z")
	filename := fmt.Sprintf("%s-%s.yaml", timestamp, env)
	path := filepath.Join(auditDir, filename)

	if err := os.WriteFile(path, composeData, 0o644); err != nil {
		return "", fmt.Errorf("save deployment snapshot: %w", err)
	}

	// Write latest symlink (as a copy for cross-platform compat)
	latestPath := filepath.Join(auditDir, fmt.Sprintf("latest-%s.yaml", env))
	_ = os.WriteFile(latestPath, composeData, 0o644)

	return path, nil
}

// Deployment represents a single deployment snapshot.
type Deployment struct {
	Timestamp time.Time
	Env       string
	Path      string
}

// ListDeployments returns all deployment snapshots in the audit-log directory, sorted newest first.
func ListDeployments(auditDir string) ([]Deployment, error) {
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read audit-log directory: %w", err)
	}

	var deployments []Deployment
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "latest-") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		parts := strings.SplitN(name, "-", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := time.Parse("20060102T150405.000Z", parts[0])
		if err != nil {
			continue
		}
		deployments = append(deployments, Deployment{
			Timestamp: ts,
			Env:       parts[1],
			Path:      filepath.Join(auditDir, e.Name()),
		})
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Timestamp.After(deployments[j].Timestamp)
	})

	return deployments, nil
}

// LatestDeployment returns the most recent deployment for the given environment.
func LatestDeployment(auditDir, env string) (*Deployment, error) {
	latestPath := filepath.Join(auditDir, fmt.Sprintf("latest-%s.yaml", env))
	if _, err := os.Stat(latestPath); err != nil {
		return nil, fmt.Errorf("no previous %s deployment found", env)
	}

	info, err := os.Stat(latestPath)
	if err != nil {
		return nil, err
	}

	return &Deployment{
		Timestamp: info.ModTime(),
		Env:       env,
		Path:      latestPath,
	}, nil
}

// LoadDeployment reads a deployment snapshot.
func LoadDeployment(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read deployment: %w", err)
	}
	return data, nil
}
