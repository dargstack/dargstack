package cli

import (
	"bytes"
	"log/slog"
	"os"
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
