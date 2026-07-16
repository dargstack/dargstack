package platform

import (
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	got := Detect()
	if got != runtime.GOOS {
		t.Errorf("expected %q, got %q", runtime.GOOS, got)
	}
}

func TestGetWithOverride(t *testing.T) {
	got := Get("darwin")
	if got != "darwin" {
		t.Errorf("expected darwin, got %q", got)
	}
}

func TestGetNoOverride(t *testing.T) {
	got := Get("")
	if got != runtime.GOOS {
		t.Errorf("expected %q, got %q", runtime.GOOS, got)
	}
}
