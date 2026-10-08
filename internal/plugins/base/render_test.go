package base_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// newPC seeds a PartialContext with the given Data and inputs.
func newPC(d *plugins.Data, inputs map[string]any) *templates.PartialContext {
	return &templates.PartialContext{Data: d, Inputs: inputs}
}

func putResult(d *plugins.Data, r *base.Result) { d.SetPlugin(base.Name, r) }

// gates returns the canonical "everything on" input set so tests stay
// terse. Individual tests override single keys with maps.Copy semantics
// inline. The chrome_* booleans (#640) are the canonical surface for
// partial visibility.
func gates(extra ...string) map[string]any {
	m := map[string]any{
		"chrome_activity":     "yes",
		"chrome_community":    "yes",
		"chrome_repositories": "yes",
	}
	for i := 0; i+1 < len(extra); i += 2 {
		m[extra[i]] = extra[i+1]
	}
	return m
}

// activitySample seeds a populated user profile suitable for the
// activity+community panel.
func activitySample() *base.Result {
	return &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:                    "octocat",
				Commits:                  1500,
				PullRequestsReviewed:     12,
				PullRequestsOpened:       30,
				IssuesOpened:             5,
				IssueComments:            200,
				Organizations:            3,
				Sponsoring:               2,
				Starred:                  88,
				Watching:                 4,
				Following:                7,
				SponsorshipsAsMaintainer: 1,
			},
		},
		RepositorySummary: &plugins.ComputedRepositories{
			Count:      50,
			Forked:     5,
			Forks:      120,
			Stargazers: 4321,
			Watchers:   10,
			Releases:   15,
			Packages:   2,
			DiskUsage:  1024 * 1024,
			LicensePreference: []plugins.LicenseShare{
				{Name: "MIT", Count: 30, Percent: 60},
			},
		},
	}
}

// ---------------------------------------------------------------------
// ActivityPartial
// ---------------------------------------------------------------------

func TestActivityPartial_GatedOff(t *testing.T) {
	t.Parallel()
	// The activity panel requires an explicit opt-in via
	// chrome_activity / chrome_community (or the legacy plugin_base
	// master switch while no chrome_* is declared). Bare `nil` / `{}`
	// inputs render nothing in v3.0 (#649 removed the v2 default-all
	// fallback).
	cases := []map[string]any{
		nil,
		{},
		// chrome_activity / chrome_community both off — even if other
		// chrome keys are present.
		{"chrome_header": "yes"},
		{"chrome_repositories": "yes"},
		// Explicit chrome_*=no must keep the partial empty.
		{"chrome_activity": "no", "chrome_community": "no"},
		// Legacy compat: plugin_base=yes alone enables, but only while
		// no chrome_* input is declared. Combining the two below
		// suppresses the panel.
		{"plugin_base": "yes", "chrome_header": "yes"},
		// Legacy `base` input is silently ignored in v3.0.
		{"base": ""},
		{"base": "activity,community"},
	}
	for i, in := range cases {
		d := plugins.NewData()
		putResult(d, activitySample())
		got, _, err := base.ActivityPartial(context.Background(), newPC(d, in))
		if err != nil || got != "" {
			t.Errorf("case %d: ActivityPartial = %q, %v; want \"\", nil", i, got, err)
		}
	}
}

// TestActivityPartial_ChromeActivityAlone — opting into chrome_activity
// without chrome_community must still render the panel (the panel
// emits both columns; chrome_community is an alias for the same gate).
func TestActivityPartial_ChromeActivityAlone(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.ActivityPartial(context.Background(),
		newPC(d, map[string]any{"chrome_activity": "yes"}))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	if !strings.Contains(got, "Activity") || !strings.Contains(got, "Community stats") {
		t.Errorf("expected both columns on chrome_activity=yes alone; got:\n%s", got)
	}
}

// TestActivityPartial_LegacyPluginBaseEnables verifies the v2 compat
// path: `plugin_base=yes` alone (no chrome_* key declared) still pulls
// the panel in.
func TestActivityPartial_LegacyPluginBaseEnables(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.ActivityPartial(context.Background(),
		newPC(d, map[string]any{"plugin_base": "yes"}))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	if !strings.Contains(got, "Activity") {
		t.Errorf("legacy plugin_base=yes should enable; got:\n%s", got)
	}
}

// TestActivityPartial_EmptyWithoutUserProfile — the panel renders nothing
// when the plugin entry is missing, the profile is an organization, or the
// Provider failed (Result.Error set, Profile nil).
func TestActivityPartial_EmptyWithoutUserProfile(t *testing.T) {
	t.Parallel()
	missing := plugins.NewData()
	org := plugins.NewData()
	putResult(org, &base.Result{
		Profile: &plugins.Profile{
			Kind:         plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{Login: "octolabs"},
		},
	})
	failed := plugins.NewData()
	putResult(failed, &base.Result{Error: context.DeadlineExceeded})
	for name, d := range map[string]*plugins.Data{"missing": missing, "org": org, "provider error": failed} {
		got, _, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
		if err != nil || got != "" {
			t.Errorf("%s: ActivityPartial = %q, %v; want \"\", nil", name, got, err)
		}
	}
}

func TestActivityPartial_RendersAllRows(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	for _, want := range []string{
		`data-section="activity-community"`,
		`Activity`,
		`1.5k Commits`,
		`12 Pull requests reviewed`,
		`30 Pull requests opened`,
		`5 Issues opened`,
		`200 issue comments`,
		`Community stats`,
		`Member of 3 organizations`,
		`Following 7 users`,
		`Sponsoring 2 repositories`,
		`Starred 88 repositories`,
		`Watching 4 repositories`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in activity output:\n%s", want, got)
		}
	}
}

func TestActivityPartial_SingularGrammar(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:                "octocat",
				Commits:              1,
				PullRequestsReviewed: 1,
				PullRequestsOpened:   1,
				IssuesOpened:         1,
				IssueComments:        1,
				Organizations:        1,
				Sponsoring:           1,
				Starred:              1,
				Watching:             1,
				Following:            1,
			},
		},
	})
	got, _, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	for _, want := range []string{
		`1 Commit`,
		`1 Pull request reviewed`,
		`1 Pull request opened`,
		`1 Issue opened`,
		`1 issue comment`,
		`Member of 1 organization`,
		`Following 1 user`,
		`Sponsoring 1 repository`,
		`Starred 1 repository`,
		`Watching 1 repository`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing singular %q in:\n%s", want, got)
		}
	}
}

func TestActivityPartial_ZeroRowsStillEmit(t *testing.T) {
	t.Parallel()
	// Upstream EJS renders rows even at zero (e.g. "Sponsoring 0
	// repositories"). Our implementation mirrors that — anchor it so a
	// future change that hides zero rows is loud.
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "freshie"},
		},
	})
	got, _, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	for _, want := range []string{
		`0 Commits`,
		`Sponsoring 0 repositories`,
		`Member of 0 organizations`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing zero-state row %q in:\n%s", want, got)
		}
	}
}

// ---------------------------------------------------------------------
// RepositoriesPartial
// ---------------------------------------------------------------------

func TestRepositoriesPartial_GatedOff(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	for i, in := range []map[string]any{
		nil,
		{},
		{"chrome_header": "yes"},
		{"chrome_activity": "yes", "chrome_community": "yes"},
		// Explicit chrome_repositories=no must keep the partial empty.
		{"chrome_repositories": "no"},
		// plugin_base=yes combined with any chrome_* key suppresses
		// the v2 compat fallback.
		{"plugin_base": "yes", "chrome_header": "yes"},
		// Legacy `base` input is silently ignored in v3.0.
		{"base": ""},
		{"base": "repositories"},
	} {
		got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, in))
		if err != nil || got != "" {
			t.Errorf("case %d: RepositoriesPartial = %q, %v; want \"\", nil", i, got, err)
		}
	}
}

// TestRepositoriesPartial_ChromeRepositoriesAlone — opting in via the
// new canonical input renders the panel.
func TestRepositoriesPartial_ChromeRepositoriesAlone(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.RepositoriesPartial(context.Background(),
		newPC(d, map[string]any{"chrome_repositories": "yes"}))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, "Repositories") {
		t.Errorf("expected repositories heading on chrome_repositories=yes; got:\n%s", got)
	}
}

// TestRepositoriesPartial_LegacyPluginBaseEnables verifies the v2
// compat path: `plugin_base=yes` alone (no chrome_* key declared)
// still pulls the repositories panel in.
func TestRepositoriesPartial_LegacyPluginBaseEnables(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.RepositoriesPartial(context.Background(),
		newPC(d, map[string]any{"plugin_base": "yes"}))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, "Repositories") {
		t.Errorf("legacy plugin_base=yes should enable; got:\n%s", got)
	}
}

func TestRepositoriesPartial_OrgProfileEmpty(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind:         plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{Login: "octolabs"},
		},
		RepositorySummary: &plugins.ComputedRepositories{Count: 1},
	})
	got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil || got != "" {
		t.Fatalf("RepositoriesPartial(org) = %q, %v; want \"\", nil", got, err)
	}
}

func TestRepositoriesPartial_RendersHeadingAndRows(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	for _, want := range []string{
		`data-section="repositories"`,
		`50 Repositories`,
		`Prefers MIT license`,
		`15 Releases`,
		`2 Packages`,
		`1 GB used`,
		`1 Sponsor`,
		`4.3k Stargazers`,
		`120 Forkers`,
		`10 Watchers`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in repositories output:\n%s", want, got)
		}
	}
}

func TestRepositoriesPartial_NilSummaryFallback(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "freshie"},
		},
	})
	got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	for _, want := range []string{
		`0 Repositories`,
		`No license preference`,
		`0 Releases`,
		`0 KB used`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing zero-state %q in:\n%s", want, got)
		}
	}
}

func TestRepositoriesPartial_SingularHeading(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "octocat"},
		},
		RepositorySummary: &plugins.ComputedRepositories{Count: 1, Forked: 1},
	})
	got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, `1 Repository`) {
		t.Errorf("expected singular heading, got:\n%s", got)
	}
}

func TestRepositoriesPartial_EscapesLicenseName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "octocat"},
		},
		RepositorySummary: &plugins.ComputedRepositories{
			Count: 1,
			LicensePreference: []plugins.LicenseShare{
				{Name: "<MIT & friends>", Count: 1, Percent: 100},
			},
		},
	})
	got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	// The native-SVG field ellipsis-truncates the label to the column
	// width, so assert the escaped prefix survives (single-escaped) and
	// is NOT double-encoded (`&amp;lt;`), which would be the bug if the
	// writer pre-escaped before chrome.SVGText escaped again.
	if !strings.Contains(got, `Prefers &lt;MIT &amp; friends`) {
		t.Errorf("license name not escaped once:\n%s", got)
	}
	if strings.Contains(got, `&amp;lt;`) {
		t.Errorf("license name double-escaped:\n%s", got)
	}
}

// trafficStub satisfies the interface{ TotalViews() int } contract that
// RepositoriesPartial uses to inline the traffic row without importing
// internal/plugins/traffic (which would risk an init cycle).
type trafficStub struct{ total int }

func (s *trafficStub) TotalViews() int { return s.total }

// TestRepositoriesPartial_TrafficInline verifies that when the traffic
// plugin published a non-zero TotalViews, the right column gains an
// inline "<N> view(s) in last two weeks" row, and that a zero total
// (skipped, error, or genuinely zero traffic) emits no row. Mirrors
// upstream base.repositories.ejs's `<%= plugins.traffic.views.count %>`
// block.
func TestRepositoriesPartial_TrafficInline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		total int
		want  string // empty means the row must be absent
	}{
		{2345, `2.3k views in last two weeks`},
		{1, `1 view in last two weeks`},
		{0, ""},
	} {
		d := plugins.NewData()
		putResult(d, activitySample())
		d.SetPlugin("traffic", &trafficStub{total: tc.total})
		got, _, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
		if err != nil {
			t.Fatalf("RepositoriesPartial: %v", err)
		}
		if tc.want == "" {
			if strings.Contains(got, `in last two weeks`) {
				t.Errorf("total=%d: traffic row should be suppressed in:\n%s", tc.total, got)
			}
		} else if !strings.Contains(got, tc.want) {
			t.Errorf("total=%d: missing %q in:\n%s", tc.total, tc.want, got)
		}
	}
}
