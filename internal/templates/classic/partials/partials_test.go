package partials_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func newPC(d *plugins.Data) *templates.PartialContext {
	return &templates.PartialContext{Data: d}
}

// TestIntroduction_AbsentPlugin confirms the introduction stub returns
// "" when no introduction plugin Result is published. Until the
// introduction plugin lands the partial is intentionally inert.
func TestIntroduction_AbsentPlugin(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := partials.Introduction(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Introduction(absent) = %q,%v", got, err)
	}
}

// TestLookup_CoversManifest asserts every entry in `_.json` resolves to
// a registered partial. After #602 only `introduction` remains as a
// static partial; the header card is now rendered through the `header`
// plugin's `plugin.header` registration in PluginPartialOrder.
func TestLookup_CoversManifest(t *testing.T) {
	t.Parallel()
	if _, ok := partials.Lookup("introduction"); !ok {
		t.Error(`Lookup("introduction") missing`)
	}
	if _, ok := partials.Lookup("nonexistent"); ok {
		t.Error("Lookup should reject unknown names")
	}
}
