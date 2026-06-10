package classic_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"
)

// updateBaseGolden is the unit-test-package equivalent of the project-
// wide `-update` flag declared in tests/integration/output_json_test.go.
// Each Go test binary is built per-package, so the integration flag is
// not visible here — we always own the registration in this binary.
// Other test files in the same package can reuse this pointer if they
// adopt the `-update` workflow later.
var updateBaseGolden = flag.Bool("update", false, "regenerate golden test fixtures from current output")

// baseRenderGoldenPath returns the absolute path of the golden file
// at <repo-root>/tests/golden/<rel>.
func baseRenderGoldenPath(t *testing.T, rel string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "tests", "golden", rel)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", cwd)
	return ""
}

// TestClassic_BaseRender_AllSections is the #419 regression anchor:
// with no plugin_* toggles set, the classic template must render every
// non-empty base section exactly once and must not emit empty wrapper
// sections that previously inflated the SVG with placeholder rows.
//
// The fixture injects an indepth-populated Data so the
// activity-community section also renders (mirroring a normal Action
// run where the base plugin's runIndepth path has fired). The golden
// file lives at tests/golden/classic/base_default.svg; rerun with
// `-update` after intentional changes.
func TestClassic_BaseRender_AllSections(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.User = &plugins.User{
		Login:     "octocat",
		Name:      "The Octocat",
		AvatarURL: "https://avatars.githubusercontent.com/u/12345?v=4",
		// 442: Activity / Community counters sourced from the base
		// plugin's contributionsCollection + connection fetches.
		Commits:              3214,
		PullRequestsReviewed: 9,
		PullRequestsOpened:   17,
		IssuesOpened:         42,
		IssueComments:        88,
		Organizations:        3,
		Following:            20,
		Sponsoring:           0,
		Starred:              120,
		Watching:             32,
	}
	data.Config.Timezone.Name = "UTC"
	data.Computed.Repositories.Count = 51
	data.Computed.Repositories.Stargazers = 1500
	data.Computed.Repositories.Forks = 81

	pc := &templates.PartialContext{
		Inputs: map[string]any{},
		Data:   data,
	}
	out, err := classic.Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, marker := range []string{
		`data-section="header"`,
		`data-section="activity-community"`,
		`data-block="activity"`,
		`data-block="community"`,
		`data-section="repositories"`,
		`data-section="metadata"`,
		`51 Repositories`,
		`1500 Stargazers`,
		`81 Forkers`,
		`3214 Commits`,
		`9 Pull requests reviewed`,
		`17 Pull requests opened`,
		`42 Issues opened`,
		`88 issue comments`,
		`Member of 3 organizations`,
		`Sponsoring 0 repositories`,
		`Starred 120 repositories`,
		`Watching 32 repositories`,
		`The Octocat`,
		`<footer>`,
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("base render missing marker %q", marker)
		}
	}

	gp := baseRenderGoldenPath(t, "classic/base_default.svg")
	if *updateBaseGolden {
		if mkErr := os.MkdirAll(filepath.Dir(gp), 0o750); mkErr != nil {
			t.Fatalf("MkdirAll: %v", mkErr)
		}
		if wErr := os.WriteFile(gp, []byte(out), 0o600); wErr != nil {
			t.Fatalf("WriteFile: %v", wErr)
		}
		t.Logf("golden updated: %s (%d bytes)", gp, len(out))
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", gp, err)
	}
	gotNorm, err := golden.NormalizeSVG([]byte(out))
	if err != nil {
		t.Fatalf("normalize got: %v", err)
	}
	wantNorm, err := golden.NormalizeSVG(want)
	if err != nil {
		t.Fatalf("normalize want: %v", err)
	}
	if string(gotNorm) != string(wantNorm) {
		t.Errorf("base_default.svg drift (len got=%d want=%d); first divergence at\n  got:  %s\n  want: %s",
			len(gotNorm), len(wantNorm),
			window(gotNorm, firstDiff(gotNorm, wantNorm)),
			window(wantNorm, firstDiff(gotNorm, wantNorm)))
	}
}

// TestClassic_BaseRender_NoIndepthSkipsActivitySection covers the
// scenario where indepth data is unavailable (e.g. the foundational
// base sample produced by gen-doc-samples.sh's plugin-base block).
// The activity-community wrapper MUST be omitted entirely — that was
// the visible regression in #419 where an empty `<section
// data-section="activity-community">` produced a tall whitespace band
// in the rendered card.
func TestClassic_BaseRender_NoIndepthSkipsActivitySection(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.User = &plugins.User{
		Login:     "octocat",
		Name:      "The Octocat",
		AvatarURL: "https://avatars.githubusercontent.com/u/12345?v=4",
	}
	data.Computed.Repositories.Count = 51
	data.Computed.Repositories.Stargazers = 1500
	data.Computed.Repositories.Forks = 81
	// Note: no Computed.Total* values populated — indepth was not
	// triggered, so the activity-community block has nothing to show.

	pc := &templates.PartialContext{
		Inputs: map[string]any{},
		Data:   data,
	}
	out, err := classic.Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, marker := range []string{
		`data-section="header"`,
		`data-section="repositories"`,
		`data-section="metadata"`,
		`51 Repositories`,
		`1500 Stargazers`,
		`81 Forkers`,
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("base render missing marker %q", marker)
		}
	}

	// Empty activity-community wrapper must NOT appear. Asserting on
	// the open tag is sufficient because BaseActivityCommunity now
	// short-circuits to "" instead of emitting the wrapper at all.
	if strings.Contains(out, `data-section="activity-community"`) {
		t.Errorf("activity-community wrapper should be omitted when indepth data is absent\noutput:\n%s", out)
	}
	// Header sub-row placeholder must NOT appear either.
	if strings.Contains(out, `<div class="row"><section></section><section></section></div>`) {
		t.Errorf("header empty placeholder row should not be emitted\noutput:\n%s", out)
	}

	gp := baseRenderGoldenPath(t, "classic/base_minimal.svg")
	if *updateBaseGolden {
		if mkErr := os.MkdirAll(filepath.Dir(gp), 0o750); mkErr != nil {
			t.Fatalf("MkdirAll: %v", mkErr)
		}
		if wErr := os.WriteFile(gp, []byte(out), 0o600); wErr != nil {
			t.Fatalf("WriteFile: %v", wErr)
		}
		t.Logf("golden updated: %s (%d bytes)", gp, len(out))
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", gp, err)
	}
	gotNorm, err := golden.NormalizeSVG([]byte(out))
	if err != nil {
		t.Fatalf("normalize got: %v", err)
	}
	wantNorm, err := golden.NormalizeSVG(want)
	if err != nil {
		t.Fatalf("normalize want: %v", err)
	}
	if string(gotNorm) != string(wantNorm) {
		t.Errorf("base_minimal.svg drift (len got=%d want=%d); first divergence at\n  got:  %s\n  want: %s",
			len(gotNorm), len(wantNorm),
			window(gotNorm, firstDiff(gotNorm, wantNorm)),
			window(wantNorm, firstDiff(gotNorm, wantNorm)))
	}
}

// firstDiff returns the first byte offset at which a and b differ. If
// one is a prefix of the other, returns the length of the shorter.
func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// window renders a 60-byte window around offset for diff messages.
func window(b []byte, offset int) string {
	lo := offset - 30
	if lo < 0 {
		lo = 0
	}
	hi := offset + 30
	if hi > len(b) {
		hi = len(b)
	}
	if lo >= hi {
		return "(empty)"
	}
	return string(b[lo:hi])
}
