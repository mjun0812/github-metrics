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

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"unknown", slog.LevelInfo},
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
