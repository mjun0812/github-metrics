package plugins

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestRequireUserMode_RepoModeSkips covers the primary contract: when
// Data.Repo is non-nil (repository template) the helper must report
// skip=true and return a descriptive reason that includes the plugin
// slug + "user mode" + "repository".
func TestRequireUserMode_RepoModeSkips(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	pc := &PluginContext{
		Logger: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Data:   NewData(),
	}
	pc.Data.SetRepo(&Repo{Owner: "o", Name: "r"})

	reason, skip := RequireUserMode(pc, "achievements")
	if !skip {
		t.Fatalf("expected skip=true under repo mode, got false")
	}
	if !strings.Contains(reason, "achievements") {
		t.Fatalf("reason must mention plugin slug, got %q", reason)
	}
	if !strings.Contains(reason, "user mode") {
		t.Fatalf("reason must mention supported mode (user), got %q", reason)
	}
	if !strings.Contains(reason, "repository") {
		t.Fatalf("reason must mention current mode (repository), got %q", reason)
	}
	if !strings.Contains(buf.String(), "achievements") {
		t.Fatalf("expected WARN log to mention plugin slug, got %q", buf.String())
	}
}

// TestRequireUserMode_UserModePasses verifies the helper is a no-op
// when Data.Repo is nil — plugins must continue to their normal path.
func TestRequireUserMode_UserModePasses(t *testing.T) {
	t.Parallel()
	pc := &PluginContext{
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Data:   NewData(),
	}
	if reason, skip := RequireUserMode(pc, "achievements"); skip || reason != "" {
		t.Fatalf("expected pass-through in user mode, got reason=%q skip=%v", reason, skip)
	}
}

// TestRequireRepoMode_UserModeSkips mirrors TestRequireUserMode_RepoModeSkips
// for the inverse direction (contributors plugin).
func TestRequireRepoMode_UserModeSkips(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	pc := &PluginContext{
		Logger: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Data:   NewData(),
	}
	reason, skip := RequireRepoMode(pc, "contributors")
	if !skip {
		t.Fatalf("expected skip=true under user mode, got false")
	}
	if !strings.Contains(reason, "contributors") || !strings.Contains(reason, "repository mode") {
		t.Fatalf("reason must mention plugin + repository mode, got %q", reason)
	}
	if !strings.Contains(buf.String(), "contributors") {
		t.Fatalf("expected WARN log entry, got %q", buf.String())
	}
}

// TestRequireRepoMode_RepoModePasses verifies the helper is a no-op
// when Data.Repo is non-nil.
func TestRequireRepoMode_RepoModePasses(t *testing.T) {
	t.Parallel()
	pc := &PluginContext{
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Data:   NewData(),
	}
	pc.Data.SetRepo(&Repo{Owner: "o", Name: "r"})
	if reason, skip := RequireRepoMode(pc, "contributors"); skip || reason != "" {
		t.Fatalf("expected pass-through in repo mode, got reason=%q skip=%v", reason, skip)
	}
}

// TestRequireUserMode_NilContextSafe ensures the helper does not
// nil-panic when called with a nil pc / nil Data (defensive — keeps
// plugin Run() entry-checks future-proof).
func TestRequireUserMode_NilContextSafe(t *testing.T) {
	t.Parallel()
	if _, skip := RequireUserMode(nil, "x"); skip {
		t.Fatalf("nil pc must be a no-op")
	}
	if _, skip := RequireUserMode(&PluginContext{}, "x"); skip {
		t.Fatalf("nil Data must be a no-op")
	}
}
