package resource

import "testing"

func TestCleanCommentStripsDevOnlyMarker(t *testing.T) {
	raw := "S3 console access for management\n# dargstack:dev-only"
	result := cleanComment(raw)
	if result != "S3 console access for management" {
		t.Errorf("expected dev-only marker line to be filtered, got %q", result)
	}
}

func TestCleanCommentPreservesNonMarkerLines(t *testing.T) {
	raw := "A service for handling requests\nAnother line of description"
	result := cleanComment(raw)
	if result != "A service for handling requests\nAnother line of description" {
		t.Errorf("expected all lines preserved, got %q", result)
	}
}

func TestCleanCommentEmptyInput(t *testing.T) {
	result := cleanComment("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCleanCommentOnlyMarker(t *testing.T) {
	raw := "# dargstack:dev-only"
	result := cleanComment(raw)
	if result != "" {
		t.Errorf("expected empty string when only marker present, got %q", result)
	}
}
