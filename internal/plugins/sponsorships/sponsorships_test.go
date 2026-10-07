package sponsorships_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	"github.com/mjun0812/github-metrics/internal/templates"
)

func run(t *testing.T, inputs map[string]any) *sponsorships.Result {
	t.Helper()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: inputs}
	out, err := sponsorships.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*sponsorships.Result)
}

func TestRun_DefaultEmpty(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.Skipped {
		t.Errorf("MVP should return empty non-Skipped Result")
	}
	if r.Active == nil || len(r.Active) != 0 {
		t.Errorf("Active should be a non-nil empty slice; got %#v", r.Active)
	}
}

// partialFor renders the partial against a Data carrying the given
// Result and user login. Mirrors how the classic dispatcher invokes it.
func partialFor(t *testing.T, r *sponsorships.Result, login string) string {
	t.Helper()
	data := plugins.NewData()
	data.User = &plugins.User{Login: login}
	data.SetPlugin(sponsorships.Name, r)
	out, _, err := sponsorships.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return out
}

// TestPartial_AmountFormatting checks the en-US USD formatting of
// non-zero, thousands-separated amounts.
func TestPartial_AmountFormatting(t *testing.T) {
	t.Parallel()
	r := &sponsorships.Result{
		Active:   []sponsorships.Sponsored{},
		Sections: []string{"amount"},
		Amount:   1234.5,
	}
	out := partialFor(t, r, "octocat")
	if !strings.Contains(out, `>$1,234.50</tspan>`) {
		t.Errorf("expected $1,234.50 in output\ngot: %s", out)
	}
	// Only the amount section requested: no goal-text line.
	if strings.Contains(out, "helped funding the work of") {
		t.Errorf("amount-only sections should not render the sponsorships branch\ngot: %s", out)
	}
}

// TestPartial_SponsorshipsOnly verifies the sponsorships branch can run
// without the amount section (no heart image emitted).
func TestPartial_SponsorshipsOnly(t *testing.T) {
	t.Parallel()
	r := &sponsorships.Result{
		Active:   []sponsorships.Sponsored{{Login: "alice", Type: "user"}},
		Sections: []string{"sponsorships"},
	}
	out := partialFor(t, r, "octocat")
	if strings.Contains(out, "hearts_around.png") {
		t.Errorf("sponsorships-only output should not include the amount heart image\ngot: %s", out)
	}
	if !strings.Contains(out, "helped funding the work of 1 user and organizations.") {
		t.Errorf("expected singular goal text for 1 sponsorship\ngot: %s", out)
	}
	if !strings.Contains(out, `href="https://github.com/alice.png?size=64"`) {
		t.Errorf("expected alice avatar img\ngot: %s", out)
	}
}
