package plugins

import (
	"io"
	"log/slog"
	"testing"
)

func TestRequireMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		require  func(*PluginContext, string) (string, bool)
		repoMode bool
		wantSkip bool
	}{
		{"user-only plugin in repo mode skips", RequireUserMode, true, true},
		{"user-only plugin in user mode passes", RequireUserMode, false, false},
		{"repo-only plugin in user mode skips", RequireRepoMode, false, true},
		{"repo-only plugin in repo mode passes", RequireRepoMode, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pc := &PluginContext{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Data: NewData()}
			if tc.repoMode {
				pc.Data.SetRepo(&Repo{Owner: "o", Name: "r"})
			}
			reason, skip := tc.require(pc, "x")
			if skip != tc.wantSkip {
				t.Fatalf("skip = %v, want %v", skip, tc.wantSkip)
			}
			if (reason != "") != tc.wantSkip {
				t.Fatalf("reason = %q, want non-empty only when skipped", reason)
			}
		})
	}
}
