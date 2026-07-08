package sponsorships_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

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
	if len(r.Active) != 0 {
		t.Errorf("Active should be empty in MVP; got %+v", r.Active)
	}
}

func TestRun_PastSection(t *testing.T) {
	t.Parallel()
	r := run(t, map[string]any{"plugin_sponsorships_sections": "active,past"})
	if len(r.Past) != 0 {
		t.Errorf("Past should remain empty in MVP")
	}
}

func TestRun_NoNilFields(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.Active == nil {
		t.Errorf("Active should be non-nil slice (was %v)", r.Active)
	}
}

func TestRun_IsSkippedFalse(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.IsSkipped() {
		t.Errorf("IsSkipped should be false")
	}
}

func TestRun_NilInputs(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData()}
	out, _ := sponsorships.Plugin.Run(context.Background(), pc)
	if out == nil {
		t.Fatalf("Result should not be nil even with nil inputs")
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

// TestPartial_ZeroState verifies the #449 fix: with zero sponsorships and
// the default sections, the partial still renders the amount section
// (heart image + "$0.00") and the "0 users" goal text rather than an
// empty string.
func TestPartial_ZeroState(t *testing.T) {
	t.Parallel()
	r := &sponsorships.Result{
		Active:   []sponsorships.Sponsored{},
		Sections: []string{"amount", "sponsorships"},
		Amount:   0,
		Image:    "https://github.githubassets.com/images/icons/emoji/hearts_around.png",
	}
	out := partialFor(t, r, "mjun0812")

	for _, want := range []string{
		`data-section="sponsorships"`,
		`<image href="https://github.githubassets.com/images/icons/emoji/hearts_around.png"`,
		`>$0.00</tspan>`,
		`mjun0812 helped funding the work of 0 users and organizations.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("zero-state output missing %q\ngot: %s", want, out)
		}
	}
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

func TestRun_GoldenShape(t *testing.T) {
	r := &sponsorships.Result{
		Active:   []sponsorships.Sponsored{},
		Sections: []string{"amount", "sponsorships"},
		Amount:   0,
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "sponsorships.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}
