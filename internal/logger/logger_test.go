package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/logger"
)

func TestNewDefaultsToJSONHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New(logger.Options{Writer: &buf})
	l.Info("hello", "k", "v")

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("default handler did not emit JSON: %v\nraw=%q", err, buf.String())
	}
	if got["msg"] != "hello" {
		t.Fatalf("msg = %v, want %q", got["msg"], "hello")
	}
	if got["k"] != "v" {
		t.Fatalf("k = %v, want %q", got["k"], "v")
	}
}

func TestNewTextFormatEmitsKeyValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New(logger.Options{Format: logger.FormatText, Writer: &buf})
	l.Info("hello", "k", "v")

	if !strings.Contains(buf.String(), "msg=hello") {
		t.Fatalf("text handler missing msg key: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "k=v") {
		t.Fatalf("text handler missing attribute: %q", buf.String())
	}
	if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Fatalf("text handler unexpectedly emitted JSON: %q", buf.String())
	}
}

func TestLevelFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    slog.Level
		emitFn   func(*slog.Logger)
		wantBody bool
	}{
		{
			name:     "debug record dropped at info level",
			level:    slog.LevelInfo,
			emitFn:   func(l *slog.Logger) { l.Debug("nope") },
			wantBody: false,
		},
		{
			name:     "info record kept at info level",
			level:    slog.LevelInfo,
			emitFn:   func(l *slog.Logger) { l.Info("yes") },
			wantBody: true,
		},
		{
			name:     "debug record kept at debug level",
			level:    slog.LevelDebug,
			emitFn:   func(l *slog.Logger) { l.Debug("yes") },
			wantBody: true,
		},
		{
			name:     "warn record dropped at error level",
			level:    slog.LevelError,
			emitFn:   func(l *slog.Logger) { l.Warn("nope") },
			wantBody: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			l := logger.New(logger.Options{Level: tc.level, Writer: &buf})
			tc.emitFn(l)
			got := buf.Len() > 0
			if got != tc.wantBody {
				t.Fatalf("body emitted = %v (raw=%q), want %v", got, buf.String(), tc.wantBody)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := logger.ParseLevel(tc.input); got != tc.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSetDefaultReplacesGlobalLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	got := logger.SetDefault(logger.Options{Writer: &buf})
	if got == nil {
		t.Fatalf("SetDefault returned nil")
	}
	slog.Default().Info("hello")
	if !strings.Contains(buf.String(), `"msg":"hello"`) {
		t.Fatalf("SetDefault did not install logger: %q", buf.String())
	}
}

func TestHandlerWithAttrsAndWithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := logger.New(logger.Options{Writer: &buf})
	enriched := base.With("component", "boot")
	enriched.Info("ready")

	if !strings.Contains(buf.String(), `"component":"boot"`) {
		t.Fatalf("WithAttrs did not propagate: %q", buf.String())
	}

	buf.Reset()
	grouped := base.WithGroup("svc")
	grouped.Info("hit", "code", 200)
	if !strings.Contains(buf.String(), `"svc":{"code":200}`) {
		t.Fatalf("WithGroup did not nest attrs: %q", buf.String())
	}
}
