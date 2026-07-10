// Package logger configures a slog logger for the github-metrics project.
//
// The exported [New] function returns a slog.Logger backed by a JSON
// handler by default; switch to text via [Options.Format].
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects the underlying slog handler.
type Format string

const (
	// FormatJSON is the production default; emits one JSON object per record.
	FormatJSON Format = "json"
	// FormatText is intended for local development; emits key=value lines.
	FormatText Format = "text"
)

// Options configures a logger constructed by [New].
type Options struct {
	// Format selects JSON (default) or text output.
	Format Format
	// Level sets the minimum record level. Zero value is slog.LevelInfo.
	Level slog.Level
	// Writer is the destination. Defaults to os.Stderr when nil.
	Writer io.Writer
}

// New constructs a slog.Logger with a context-aware handler.
func New(opts Options) *slog.Logger {
	if opts.Writer == nil {
		opts.Writer = os.Stderr
	}
	handlerOpts := &slog.HandlerOptions{Level: opts.Level}
	var inner slog.Handler
	switch strings.ToLower(string(opts.Format)) {
	case string(FormatText):
		inner = slog.NewTextHandler(opts.Writer, handlerOpts)
	default:
		inner = slog.NewJSONHandler(opts.Writer, handlerOpts)
	}
	return slog.New(inner)
}

// SetDefault installs a logger built from opts as the global slog default.
// It returns the installed logger for convenience.
func SetDefault(opts Options) *slog.Logger {
	l := New(opts)
	slog.SetDefault(l)
	return l
}

// ParseLevel maps a string ("debug" / "info" / "warn" / "error") to the
// corresponding slog.Level. Unknown values fall back to slog.LevelInfo.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
