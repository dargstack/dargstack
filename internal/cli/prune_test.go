package cli

import "testing"

func TestRunPruneSkipsDockerWhenNotConfirmed(t *testing.T) {
	oldDryRun, oldNoInteraction, oldConfirm := dryRun, noInteraction, confirmFunc
	defer func() { dryRun, noInteraction, confirmFunc = oldDryRun, oldNoInteraction, oldConfirm }()

	// cfg is nil here, so reaching docker.NewExecutor(cfg.Runtime.Sudo) would panic.
	tests := []struct {
		name    string
		dryRun  bool
		confirm bool
	}{
		{"dry run", true, true},
		{"declined", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dryRun, noInteraction = tt.dryRun, false
			asked := false
			confirmFunc = func(string, bool) (bool, error) { asked = true; return tt.confirm, nil }

			if err := runPrune(nil, nil); err != nil {
				t.Fatalf("runPrune() = %v, want nil", err)
			}
			if tt.dryRun && asked {
				t.Error("dry run must not prompt")
			}
		})
	}
}
