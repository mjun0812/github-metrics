package languages_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins/languages"
)

// TestPluginIdentity pins the Name / Metadata getters every Plugin
// implementation must expose. The `languages` and `languages.indepth`
// plugins both return nil metadata (their inputs live on the parent
// metadata.yml manifest, not a per-plugin one).
func TestPluginIdentity(t *testing.T) {
	t.Parallel()
	if got := languages.Plugin.Name(); got != "languages" {
		t.Errorf("Plugin.Name() = %q, want %q", got, "languages")
	}
	if got := languages.Plugin.Metadata(); got != nil {
		t.Errorf("Plugin.Metadata() = %v, want nil", got)
	}
	if got := languages.IndepthPlugin.Name(); got != "languages.indepth" {
		t.Errorf("IndepthPlugin.Name() = %q, want %q", got, "languages.indepth")
	}
	if got := languages.IndepthPlugin.Metadata(); got != nil {
		t.Errorf("IndepthPlugin.Metadata() = %v, want nil", got)
	}
}

// TestResultIsSkipped covers the duck-typed IsSkipped() the classic
// dispatcher uses to filter skipped plugins out of the partial pipeline.
func TestResultIsSkipped(t *testing.T) {
	t.Parallel()

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()
		var r *languages.Result
		if r.IsSkipped() {
			t.Errorf("nil *Result.IsSkipped() = true, want false")
		}
		var ir *languages.IndepthResult
		if ir.IsSkipped() {
			t.Errorf("nil *IndepthResult.IsSkipped() = true, want false")
		}
	})

	t.Run("Skipped flag drives the return value", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			skipped bool
			want    bool
		}{{false, false}, {true, true}}
		for _, tc := range cases {
			if got := (&languages.Result{Skipped: tc.skipped}).IsSkipped(); got != tc.want {
				t.Errorf("Result{Skipped: %v}.IsSkipped() = %v, want %v", tc.skipped, got, tc.want)
			}
			if got := (&languages.IndepthResult{Skipped: tc.skipped}).IsSkipped(); got != tc.want {
				t.Errorf("IndepthResult{Skipped: %v}.IsSkipped() = %v, want %v", tc.skipped, got, tc.want)
			}
		}
	})
}
