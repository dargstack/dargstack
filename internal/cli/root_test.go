package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestResolveLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		want    int
		wantErr bool
	}{
		{"error level", "error", levelError, false},
		{"warn level", "warn", levelWarn, false},
		{"info level", "info", levelInfo, false},
		{"debug level", "debug", levelDebug, false},
		{"invalid level", "trace", 0, true},
		{"empty level", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveLogLevel(%q) error = %v, wantErr %v", tt.level, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("resolveLogLevel(%q) = %d, want %d", tt.level, got, tt.want)
			}
		})
	}
}

func TestPrintHelpersRespectLogLevel(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     string
		call         func()
		expectStdout bool
		expectStderr bool
	}{
		{"error at error level prints", "error", func() { printError("test") }, false, true},
		{"error at info level prints", "info", func() { printError("test") }, false, true},
		{"warn at warn level prints", "warn", func() { printWarning("test") }, false, true},
		{"warn at error level suppressed", "error", func() { printWarning("test") }, false, false},
		{"info at info level prints", "info", func() { printInfo("test") }, true, false},
		{"info at warn level suppressed", "warn", func() { printInfo("test") }, false, false},
		{"success at info level prints", "info", func() { printSuccess("test") }, true, false},
		{"success at warn level suppressed", "warn", func() { printSuccess("test") }, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			oldLogLevel := logLevelValue

			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			level, _ := resolveLogLevel(tt.logLevel)
			logLevelValue = level

			defer func() {
				os.Stdout = oldStdout
				os.Stderr = oldStderr
				logLevelValue = oldLogLevel
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
