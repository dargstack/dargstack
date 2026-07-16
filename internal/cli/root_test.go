package cli

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

func TestLoggerRespectsLogLevel(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     slog.Level
		call         func()
		expectStdout bool
		expectStderr bool
	}{
		{"error at error level prints", slog.LevelError, func() { logger.L.Error("test") }, false, true},
		{"error at info level prints", slog.LevelInfo, func() { logger.L.Error("test") }, false, true},
		{"warn at warn level prints", slog.LevelWarn, func() { logger.L.Warn("test") }, false, true},
		{"warn at error level suppressed", slog.LevelError, func() { logger.L.Warn("test") }, false, false},
		{"info at info level prints", slog.LevelInfo, func() { logger.L.Info("test") }, true, false},
		{"info at warn level suppressed", slog.LevelWarn, func() { logger.L.Info("test") }, false, false},
		{"success at info level prints", slog.LevelInfo, func() { logger.Success("test") }, true, false},
		{"success at warn level suppressed", slog.LevelWarn, func() { logger.Success("test") }, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			oldLevel := logger.Level.Level()

			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			logger.Level.Set(tt.logLevel)

			defer func() {
				os.Stdout = oldStdout
				os.Stderr = oldStderr
				logger.Level.Set(oldLevel)
			}()

			tt.call()
			_ = wOut.Close()
			_ = wErr.Close()

			var outBuf, errBuf bytes.Buffer
			_, _ = outBuf.ReadFrom(rOut)
			_, _ = errBuf.ReadFrom(rErr)

			gotStdout := outBuf.Len() > 0
			gotStderr := errBuf.Len() > 0

			if gotStdout != tt.expectStdout {
				t.Errorf("stdout: got %v, want %v (buf: %q)", gotStdout, tt.expectStdout, outBuf.String())
			}
			if gotStderr != tt.expectStderr {
				t.Errorf("stderr: got %v, want %v (buf: %q)", gotStderr, tt.expectStderr, errBuf.String())
			}
		})
	}
}

func TestResolveProfiles(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		flagSet    bool
		flagValue  []string
		wantNil    bool
		wantValues []string
	}{
		{
			name:       "env var populates profiles when flag not set",
			envVar:     "db,monitoring",
			flagSet:    false,
			wantNil:    false,
			wantValues: []string{"db", "monitoring"},
		},
		{
			name:       "flag overrides env var",
			envVar:     "db,monitoring",
			flagSet:    true,
			flagValue:  []string{"foo"},
			wantNil:    false,
			wantValues: []string{"foo"},
		},
		{
			name:       "whitespace and empty entries are trimmed",
			envVar:     " db , ,monitoring ",
			flagSet:    false,
			wantNil:    false,
			wantValues: []string{"db", "monitoring"},
		},
		{
			name:    "empty env var leaves profiles nil",
			envVar:  "",
			flagSet: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldProfiles := profiles
			defer func() { profiles = oldProfiles }()

			if tt.envVar != "" {
				t.Setenv("COMPOSE_PROFILES", tt.envVar)
			} else {
				t.Setenv("COMPOSE_PROFILES", "")
			}

			profiles = nil
			if tt.flagSet {
				profiles = tt.flagValue
			}

			resolveProfiles()

			if tt.wantNil {
				if profiles != nil {
					t.Errorf("expected profiles to be nil, got %v", profiles)
				}
				return
			}

			if profiles == nil {
				t.Fatal("expected profiles to be non-nil")
			}

			got := strings.Join(profiles, ",")
			want := strings.Join(tt.wantValues, ",")
			if got != want {
				t.Errorf("profiles: got %q, want %q", got, want)
			}
		})
	}
}
