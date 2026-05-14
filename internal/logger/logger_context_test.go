package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/ctxutil"
	"github.com/mjun0812/github-metrics/internal/logger"
)

// US1 AS4 (specs/001-project-foundation/spec.md): when a context carries a
// login via ctxutil.WithLogin and a record is emitted through slog.Default
// (or any logger built by New), the record MUST include the "login"
// attribute. This file isolates that contract from the broader logger
// behavior tests so the AS4 link is obvious at a glance.

// installDefaultMu serializes access to slog.SetDefault across the cases
// below; the global default is process-wide state.
var installDefaultMu sync.Mutex

func installDefault(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	installDefaultMu.Lock()
	prev := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(prev)
		installDefaultMu.Unlock()
	})
	logger.SetDefault(logger.Options{Writer: buf})
}

func TestUS1AS4_DefaultLoggerEmitsLoginAttribute(t *testing.T) {
	var buf bytes.Buffer
	installDefault(t, &buf)

	ctx := ctxutil.WithLogin(context.Background(), "octocat")
	slog.Default().InfoContext(ctx, "wired")

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("default logger did not emit JSON record: %v\nraw=%q", err, buf.String())
	}
	if got["login"] != "octocat" {
		t.Fatalf("login attribute missing or wrong: got %v, want %q", got["login"], "octocat")
	}
	if got["msg"] != "wired" {
		t.Fatalf("msg = %v, want %q", got["msg"], "wired")
	}
}

func TestUS1AS4_DefaultLoggerOmitsLoginWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	installDefault(t, &buf)

	slog.Default().InfoContext(context.Background(), "wired")

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("default logger did not emit JSON record: %v\nraw=%q", err, buf.String())
	}
	if _, present := got["login"]; present {
		t.Fatalf("login attribute present when context did not carry one: %v", got["login"])
	}
}

func TestUS1AS4_LoginOverwritesAcrossNestedScopes(t *testing.T) {
	var buf bytes.Buffer
	installDefault(t, &buf)

	ctx := ctxutil.WithLogin(context.Background(), "first")
	ctx = ctxutil.WithLogin(ctx, "second")
	slog.Default().InfoContext(ctx, "scoped")

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("default logger did not emit JSON: %v\nraw=%q", err, buf.String())
	}
	if got["login"] != "second" {
		t.Fatalf("nested WithLogin should win: got %v, want %q", got["login"], "second")
	}
}
