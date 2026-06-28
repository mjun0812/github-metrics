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

func TestIntroduction_AbsentPlugin(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := partials.Introduction(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Introduction(absent) = %q,%v", got, err)
	}
}

func TestLookup_CoversManifest(t *testing.T) {
	t.Parallel()
	// #625 restored base.activity+community and base.repositories with a
	// no-op emptyPartial fallback seeded inside the partials package's
	// own init(); the owning internal/plugins/base package overrides via
	// partials.Register when it is link-loaded. Lookup must resolve all
	// three manifest entries unconditionally even without the base
	// import below (the static dispatcher errors on missing partials).
	for _, name := range []string{"introduction", "base.activity+community", "base.repositories"} {
		if _, ok := partials.Lookup(name); !ok {
			t.Errorf("Lookup(%q) missing", name)
		}
	}
	if _, ok := partials.Lookup("nonexistent"); ok {
		t.Error("Lookup should reject unknown names")
	}
}
