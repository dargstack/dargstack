package logger

import (
	"context"
	"log/slog"
	"os"

	"charm.land/lipgloss/v2"
)

// Styles for log output.
// Exported so other packages can use them directly.
var (
	StyleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	StyleInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	StyleOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

var levelPrefix = map[slog.Level]string{
	slog.LevelError: "Error: ",
	slog.LevelWarn:  "Warning: ",
}

var levelStyle = map[slog.Level]lipgloss.Style{
	slog.LevelError: StyleErr,
	slog.LevelWarn:  StyleWarn,
	slog.LevelInfo:  StyleInfo,
	slog.LevelDebug: StyleInfo,
}

// levelWrite routes output to the appropriate stream.
// Each call creates a fresh colorprofile wrapper around os.Stdout/os.Stderr so that tests redirecting those globals still see the output, and non-TTY output (pipelines, CI logs) gets ANSI codes stripped automatically.
var levelWrite = map[slog.Level]func(string){
	slog.LevelError: func(s string) { _, _ = lipgloss.Fprintln(os.Stderr, s) },
	slog.LevelWarn:  func(s string) { _, _ = lipgloss.Fprintln(os.Stderr, s) },
	slog.LevelInfo:  func(s string) { _, _ = lipgloss.Fprintln(os.Stdout, s) },
	slog.LevelDebug: func(s string) { _, _ = lipgloss.Fprintln(os.Stdout, s) },
}

// Level is the mutable log level.
// Defaults to slog.LevelInfo.
var Level = new(slog.LevelVar)

// L is the package-level logger used throughout dargstack.
var L = slog.New(&styledHandler{level: Level})

// styledHandler implements slog.Handler with lipgloss styling and stdout/stderr routing based on log level.
type styledHandler struct {
	attrs []slog.Attr
	level *slog.LevelVar
}

func (h *styledHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *styledHandler) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver
	prefix := levelPrefix[r.Level]
	write := levelWrite[r.Level]

	if prefix != "" {
		write(levelStyle[r.Level].Render(prefix + r.Message))
	} else {
		write(levelStyle[r.Level].Render(r.Message))
	}
	return nil
}

func (h *styledHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &styledHandler{
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
		level: h.level,
	}
}

func (h *styledHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &styledHandler{
		attrs: append([]slog.Attr{}, h.attrs...),
		level: h.level,
	}
}

// Success logs a success message at INFO level with the OK style.
func Success(msg string) {
	if slog.LevelInfo >= Level.Level() {
		write := levelWrite[slog.LevelInfo]
		write(StyleOK.Render(msg))
	}
}
