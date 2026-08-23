package cli

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/dargstack/dargstack/v4/internal/config"
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

func TestResolveVersionIncompatibility(t *testing.T) {
	incompatErr := &config.IncompatibleVersionError{CLIVersion: "1.0.0", Compatibility: ">=2.0.0 <3.0.0"}
	otherErr := errors.New("some other error")

	tests := []struct {
		name             string
		err              error
		offline          bool
		confirmResult    bool
		selfUpdateErr    error
		wantUpdateCalled bool
		wantErrIs        error // expect the returned error to wrap/equal this
		wantErrMsg       string
	}{
		{
			name:      "non-incompatibility error passes through untouched",
			err:       otherErr,
			wantErrIs: otherErr,
		},
		{
			name:      "offline skips prompt and self-update",
			err:       incompatErr,
			offline:   true,
			wantErrIs: incompatErr,
		},
		{
			name:          "user declines the prompt",
			err:           incompatErr,
			confirmResult: false,
			wantErrIs:     incompatErr,
		},
		{
			name:             "user accepts and self-update succeeds",
			err:              incompatErr,
			confirmResult:    true,
			selfUpdateErr:    nil,
			wantUpdateCalled: true,
			wantErrMsg:       "Please re-run the command",
		},
		{
			name:             "user accepts but self-update fails",
			err:              incompatErr,
			confirmResult:    true,
			selfUpdateErr:    errors.New("network unreachable"),
			wantUpdateCalled: true,
			wantErrIs:        incompatErr,
			wantErrMsg:       "network unreachable",
		},
	}

	oldOffline := offline
	oldConfirm := confirmFunc
	oldSelfUpdate := selfUpdateFunc
	defer func() {
		offline = oldOffline
		confirmFunc = oldConfirm
		selfUpdateFunc = oldSelfUpdate
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offline = tt.offline
			confirmFunc = func(string, bool) (bool, error) { return tt.confirmResult, nil }
			called := false
			selfUpdateFunc = func() error {
				called = true
				return tt.selfUpdateErr
			}

			got := resolveVersionIncompatibility(tt.err)

			if called != tt.wantUpdateCalled {
				t.Errorf("selfUpdateFunc called=%v, want %v", called, tt.wantUpdateCalled)
			}
			if tt.wantErrIs != nil && !errors.Is(got, tt.wantErrIs) {
				t.Errorf("expected error to wrap %v, got %v", tt.wantErrIs, got)
			}
			if tt.wantErrMsg != "" && (got == nil || !strings.Contains(got.Error(), tt.wantErrMsg)) {
				t.Errorf("expected error to contain %q, got %v", tt.wantErrMsg, got)
			}
		})
	}
}

func TestApplyStackDomainDefault(t *testing.T) {
	tests := []struct {
		name                string
		env                 string
		stackDomainExplicit bool
		wantDomain          string
	}{
		{
			name:       "development uses development domain",
			env:        "development",
			wantDomain: "dev.example.com",
		},
		{
			name:       "production uses production domain",
			env:        "production",
			wantDomain: "prod.example.com",
		},
		{
			name:                "explicit STACK_DOMAIN is preserved",
			env:                 "production",
			stackDomainExplicit: true,
			wantDomain:          "user-provided.example.com",
		},
	}

	oldCfg := cfg
	oldEnv := env
	oldStackDomainExplicit := stackDomainExplicit
	defer func() {
		cfg = oldCfg
		env = oldEnv
		stackDomainExplicit = oldStackDomainExplicit
	}()

	cfg = &config.Config{
		Environment: config.EnvironmentConfig{
			Development: config.DevConfig{Domain: "dev.example.com"},
			Production:  config.ProdConfig{Domain: "prod.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env = tt.env
			stackDomainExplicit = tt.stackDomainExplicit
			if tt.stackDomainExplicit {
				t.Setenv("STACK_DOMAIN", "user-provided.example.com")
			} else {
				t.Setenv("STACK_DOMAIN", "")
				_ = os.Unsetenv("STACK_DOMAIN")
			}

			applyStackDomainDefault()

			if got := os.Getenv("STACK_DOMAIN"); got != tt.wantDomain {
				t.Errorf("STACK_DOMAIN: got %q, want %q", got, tt.wantDomain)
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
