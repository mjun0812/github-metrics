package partials_test

import (
	"context"
	"strings"
	"testing"
	"time"

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
	// #419 regression guard: the previous implementation emitted an
	// empty `<div class="row"><section></section><section></section></div>`
	// placeholder for the upstream sub-row (Joined GitHub, followed-by,
	// calendar, contributed-to). With no real data in the Go model the
	// placeholder rendered as a tall blank band; dropping it keeps the
	// header dense.
	if strings.Contains(got, `<div class="row"><section></section><section></section></div>`) {
		t.Errorf("header should not emit empty placeholder row: %s", got)
	}
}

// TestBaseHeader_PhaseOneFields anchors #429 Phase 1: when the User
// payload carries CreatedAt / Followers / Following the partial must
// render upstream-equivalent "Joined GitHub <age>", "Followed by N
// users" and "Following N users" rows.
func TestBaseHeader_PhaseOneFields(t *testing.T) {
	restore := partials.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()
	d := plugins.NewData()
	d.User = &plugins.User{
		Login:     "octocat",
		Name:      "Octo",
		AvatarURL: "https://example/x.png",
		CreatedAt: time.Date(2008, 1, 14, 4, 33, 35, 0, time.UTC),
		Followers: 1555,
		Following: 617,
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	for _, want := range []string{
		`data-block="header-counters"`,
		"Joined GitHub 18 years ago",
		"Followed by 1.6k users",
		"Following 617 users",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

// TestBaseHeader_HidesPhaseOneZeros confirms zero counters do not
// surface label rows; an account with only a known createdAt still
// gets the Joined row but no followers / following lines.
func TestBaseHeader_HidesPhaseOneZeros(t *testing.T) {
	restore := partials.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()
	d := plugins.NewData()
	d.User = &plugins.User{
		Login:     "freshie",
		CreatedAt: time.Date(2025, 11, 14, 0, 0, 0, 0, time.UTC),
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if !strings.Contains(got, "Joined GitHub 2 months ago") {
		t.Errorf("missing Joined GitHub label: %s", got)
	}
	if strings.Contains(got, "Followed by") || strings.Contains(got, "Following") {
		t.Errorf("zero counters should be hidden: %s", got)
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

// TestBaseActivityCommunity_NoDataReturnsEmpty asserts the #419 fix:
// with all activity counters at zero, the partial returns "" rather
// than emitting an empty `<section data-section="activity-community">`
// wrapper. The wrapper would otherwise produce a tall whitespace band
// in the rendered SVG when no indepth data is available (e.g. the
// foundational base-only sample render).
func TestBaseActivityCommunity_NoDataReturnsEmpty(t *testing.T) {
	t.Parallel()
	got, err := partials.BaseActivityCommunity(context.Background(), newPC(plugins.NewData()))
	if err != nil {
		t.Fatalf("BaseActivityCommunity: %v", err)
	}
	if got != "" {
		t.Errorf("BaseActivityCommunity(zero data) should return empty, got: %s", got)
	}
}

func TestBaseActivityCommunity_RendersCountersWhenIndepthPresent(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.TotalCommits = 3214
	d.Computed.TotalIssues = 42
	d.Computed.TotalPullRequests = 17
	got, err := partials.BaseActivityCommunity(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseActivityCommunity: %v", err)
	}
	for _, marker := range []string{
		`data-section="activity-community"`,
		`data-block="activity"`,
		`data-block="community"`,
		"3.2k Commits",
		"17 Pull requests opened",
		"42 Issues opened",
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q in %s", marker, got)
		}
	}
}

// TestBaseActivityCommunity_SingularLabel anchors the pluralLabel
// behaviour: counts of 1 must render without a trailing "s".
func TestBaseActivityCommunity_SingularLabel(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.TotalCommits = 1
	d.Computed.TotalIssues = 1
	d.Computed.TotalPullRequests = 1
	got, err := partials.BaseActivityCommunity(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseActivityCommunity: %v", err)
	}
	for _, marker := range []string{
		"1 Commit</div>",
		"1 Pull request opened",
		"1 Issue opened",
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("singular form missing %q in %s", marker, got)
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
	if strings.Contains(got, "Watching") {
		t.Errorf("Watching row should be hidden when User.Watching = 0: %s", got)
	}
	if strings.Contains(got, "sponsor") {
		t.Errorf("sponsor row should be hidden when User.SponsorshipsAsMaintainer = 0: %s", got)
	}
}

// TestBaseRepositories_PhaseOneFields anchors #429 Phase 1: with the
// User payload carrying Watching and SponsorshipsAsMaintainer counts,
// the partial surfaces "Watching N repositories" and "N sponsors"
// rows alongside the existing count / stargazers / forks fields.
func TestBaseRepositories_PhaseOneFields(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 8
	d.Computed.Repositories.Stargazers = 12
	d.Computed.Repositories.Forks = 1
	d.User = &plugins.User{
		Login:                    "octocat",
		Watching:                 16,
		SponsorshipsAsMaintainer: 4,
	}
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	for _, marker := range []string{
		"Watching 16 repositories",
		"4 sponsors",
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q in %s", marker, got)
		}
	}
}

// TestBaseRepositories_WatchingSingular anchors the "Watching 1
// repository" branch (singular noun when N == 1).
func TestBaseRepositories_WatchingSingular(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 1
	d.User = &plugins.User{Watching: 1, SponsorshipsAsMaintainer: 1}
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	if !strings.Contains(got, "Watching 1 repository") {
		t.Errorf("singular Watching label missing: %s", got)
	}
	if !strings.Contains(got, "1 sponsor</div>") {
		t.Errorf("singular sponsor label missing: %s", got)
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
