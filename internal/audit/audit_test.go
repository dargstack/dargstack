package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditLogDir(t *testing.T) {
	got := AuditLogDir("/stack")
	want := filepath.Join(string(filepath.Separator), "stack", "artifacts", "audit-log")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestSaveDeployment(t *testing.T) {
	dir := t.TempDir()
	data := []byte("services:\n  api:\n    image: api:latest\n")

	path, err := SaveDeployment(dir, "development", data)
	if err != nil {
		t.Fatal(err)
	}

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, data) {
		t.Errorf("saved content mismatch")
	}

	latestPath := filepath.Join(dir, "latest-development.yaml")
	latest, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal("latest copy should exist")
	}
	if !bytes.Equal(latest, data) {
		t.Error("latest copy content mismatch")
	}
}

func TestListDeployments(t *testing.T) {
	dir := t.TempDir()
	data := []byte("services: {}")

	_, err := SaveDeployment(dir, "development", data)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 10)
	_, err = SaveDeployment(dir, "production", data)
	if err != nil {
		t.Fatal(err)
	}

	deployments, err := ListDeployments(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(deployments))
	}

	if !deployments[0].Timestamp.After(deployments[1].Timestamp) {
		t.Error("expected newest first")
	}
}

func TestListDeploymentsEmpty(t *testing.T) {
	dir := t.TempDir()
	deployments, err := ListDeployments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(deployments) != 0 {
		t.Errorf("expected 0 deployments, got %d", len(deployments))
	}
}

func TestListDeploymentsNonexistent(t *testing.T) {
	deployments, err := ListDeployments("/nonexistent/path")
	if err != nil {
		t.Fatal("expected nil error for nonexistent dir")
	}
	if deployments != nil {
		t.Error("expected nil deployments for nonexistent dir")
	}
}

func TestLatestDeployment(t *testing.T) {
	dir := t.TempDir()
	data := []byte("services: {}")

	_, err := SaveDeployment(dir, "development", data)
	if err != nil {
		t.Fatal(err)
	}

	dep, err := LatestDeployment(dir, "development")
	if err != nil {
		t.Fatal(err)
	}

	if dep.Env != "development" {
		t.Errorf("expected env=development, got %s", dep.Env)
	}
}

func TestLatestDeploymentNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LatestDeployment(dir, "production")
	if err == nil {
		t.Error("expected error for missing deployment")
	}
}

func TestLoadDeployment(t *testing.T) {
	dir := t.TempDir()
	data := []byte("services:\n  api:\n    image: test\n")

	path, err := SaveDeployment(dir, "development", data)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadDeployment(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(loaded, data) {
		t.Error("loaded content mismatch")
	}
}
