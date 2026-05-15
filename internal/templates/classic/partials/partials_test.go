package partials_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func newPC(d *plugins.Data) *templates.PartialContext {
	return &templates.PartialContext{Data: d}
}

func TestBaseHeader_NilUser(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("BaseHeader(nil user) = %q,%v want \"\",nil", got, err)
	}
}

func TestBaseHeader_PopulatedEscapesName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.User = &plugins.User{
		Login:     "octocat",
		Name:      "<Octo & cat>",
		AvatarURL: "https://example/x.png",
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if !strings.Contains(got, `data-section="header"`) {
		t.Errorf("missing data-section: %s", got)
	}
	if !strings.Contains(got, "&lt;Octo &amp; cat&gt;") {
		t.Errorf("name not escaped: %s", got)
	}
	if !strings.Contains(got, `src="https://example/x.png"`) {
		t.Errorf("avatar not present: %s", got)
	}
}

func TestIntroduction_AbsentPlugin(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := partials.Introduction(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Introduction(absent) = %q,%v", got, err)
	}
}

func TestBaseActivityCommunity_OuterAlwaysEmits(t *testing.T) {
	t.Parallel()
	got, err := partials.BaseActivityCommunity(context.Background(), newPC(plugins.NewData()))
	if err != nil {
		t.Fatalf("BaseActivityCommunity: %v", err)
	}
	if !strings.Contains(got, `data-section="activity-community"`) {
		t.Errorf("outer section missing: %s", got)
	}
	for _, marker := range []string{`data-block="activity"`, `data-block="community"`} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q in %s", marker, got)
		}
	}
}

func TestBaseRepositories_ZeroCountReturnsEmpty(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("BaseRepositories(zero) = %q,%v", got, err)
	}
}

func TestBaseRepositories_RendersCounts(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 250
	d.Computed.Repositories.Stargazers = 1500
	d.Computed.Repositories.Forks = 13
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	for _, marker := range []string{"250 repositories", "1.5k stargazers", "13 forks"} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q in %s", marker, got)
		}
	}
}

func TestLookup_CoversManifest(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"base.header", "introduction", "base.activity+community", "base.repositories"} {
		if _, ok := partials.Lookup(name); !ok {
			t.Errorf("Lookup(%q) missing", name)
		}
	}
	if _, ok := partials.Lookup("nonexistent"); ok {
		t.Error("Lookup should reject unknown names")
	}
}
