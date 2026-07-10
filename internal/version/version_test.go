package version

import (
	"testing"
)

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestCommitAndDate(t *testing.T) {
	// Commit and Date may be empty when not built with ldflags.
	// This test documents that behavior and passes either way.
	t.Logf("Version=%q Commit=%q Date=%q", Version, Commit, Date)
}
