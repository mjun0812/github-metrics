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
// inline.
func gates(extra ...string) map[string]any {
	m := map[string]any{
		"plugin_base":              "yes",
		"plugin_base_activity":     "yes",
		"plugin_base_repositories": "yes",
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
				{Name: "MIT License", Count: 30, Percent: 60},
			},
		},
	}
}

// ---------------------------------------------------------------------
// ActivityPartial
// ---------------------------------------------------------------------

func TestActivityPartial_NilContextNoGate(t *testing.T) {
	t.Parallel()
	got, err := base.ActivityPartial(context.Background(), nil)
	if err != nil || got != "" {
		t.Fatalf("ActivityPartial(nil) = %q, %v; want \"\", nil", got, err)
	}
}

func TestActivityPartial_GatedOff(t *testing.T) {
	t.Parallel()
	cases := []map[string]any{
		nil,
		{},
		{"plugin_base": "yes"},          // sub-toggle missing
		{"plugin_base_activity": "yes"}, // master missing
		{"plugin_base": "no", "plugin_base_activity": "yes"},
	}
	for i, in := range cases {
		d := plugins.NewData()
		putResult(d, activitySample())
		got, err := base.ActivityPartial(context.Background(), newPC(d, in))
		if err != nil || got != "" {
			t.Errorf("case %d: ActivityPartial = %q, %v; want \"\", nil", i, got, err)
		}
	}
}

func TestActivityPartial_MissingPluginEntry(t *testing.T) {
	t.Parallel()
	got, err := base.ActivityPartial(context.Background(), newPC(plugins.NewData(), gates()))
	if err != nil || got != "" {
		t.Fatalf("ActivityPartial(no plugin entry) = %q, %v; want \"\", nil", got, err)
	}
}

func TestActivityPartial_WrongResultType(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(base.Name, "not a *Result")
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil || got != "" {
		t.Fatalf("ActivityPartial(wrong type) = %q, %v; want \"\", nil", got, err)
	}
}

func TestActivityPartial_OrgProfileEmits(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{
		Profile: &plugins.Profile{
			Kind:         plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{Login: "octolabs"},
		},
	})
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil || got != "" {
		t.Fatalf("ActivityPartial(org profile) must be empty; got %q, %v", got, err)
	}
}

func TestActivityPartial_RendersAllRows(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("ActivityPartial: %v", err)
	}
	for _, want := range []string{
		`<section class="largeable largeable-inline-flex">`,
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
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
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
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
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

// TestActivityPartial_ResultWithErrorStillRenders — a Provider failure
// recorded on Result.Error must not stop the partial from rendering
// what it can (Profile may be nil → partial returns "" without panic).
func TestActivityPartial_ResultWithErrorStillRenders(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &base.Result{Error: context.DeadlineExceeded})
	got, err := base.ActivityPartial(context.Background(), newPC(d, gates()))
	if err != nil || got != "" {
		t.Fatalf("ActivityPartial(error result, nil Profile) = %q, %v; want \"\", nil", got, err)
	}
}

// ---------------------------------------------------------------------
// RepositoriesPartial
// ---------------------------------------------------------------------

func TestRepositoriesPartial_NilContextNoGate(t *testing.T) {
	t.Parallel()
	got, err := base.RepositoriesPartial(context.Background(), nil)
	if err != nil || got != "" {
		t.Fatalf("RepositoriesPartial(nil) = %q, %v; want \"\", nil", got, err)
	}
}

func TestRepositoriesPartial_GatedOff(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	for i, in := range []map[string]any{
		nil,
		{},
		{"plugin_base": "yes"},              // sub-toggle missing
		{"plugin_base_repositories": "yes"}, // master missing
		{"plugin_base": "yes", "plugin_base_repositories": "no"},
	} {
		got, err := base.RepositoriesPartial(context.Background(), newPC(d, in))
		if err != nil || got != "" {
			t.Errorf("case %d: RepositoriesPartial = %q, %v; want \"\", nil", i, got, err)
		}
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
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil || got != "" {
		t.Fatalf("RepositoriesPartial(org) = %q, %v; want \"\", nil", got, err)
	}
}

func TestRepositoriesPartial_RendersHeadingAndRows(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	for _, want := range []string{
		`<section class="largeable largeable-inline-flex">`,
		`50 Repositories`,
		`Prefers MIT License license`,
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
	// The "(including N forks)" clause was dropped from the heading;
	// the fork count is still represented via the "<F> Forkers" row.
	if strings.Contains(got, "(including") {
		t.Errorf("repositories heading must no longer carry the forks clause:\n%s", got)
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
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
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
	if strings.Contains(got, "(including") {
		t.Errorf("zero-fork user must not get fork suffix:\n%s", got)
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
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, `1 Repository`) {
		t.Errorf("expected singular heading, got:\n%s", got)
	}
	// Forked counter must not surface in the heading even when > 0.
	if strings.Contains(got, "(including") {
		t.Errorf("heading must no longer carry the forks clause:\n%s", got)
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
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, `Prefers &lt;MIT &amp; friends&gt; license`) {
		t.Errorf("license name not escaped:\n%s", got)
	}
}

// trafficStub satisfies the interface{ TotalViews() int } contract that
// RepositoriesPartial uses to inline the traffic row without importing
// internal/plugins/traffic (which would risk an init cycle).
type trafficStub struct{ total int }

func (s *trafficStub) TotalViews() int { return s.total }

// TestRepositoriesPartial_TrafficInline verifies that when the traffic
// plugin published a non-zero TotalViews, the right column gains an
// inline "<N> views in last two weeks" row. Mirrors upstream
// base.repositories.ejs's `<%= plugins.traffic.views.count %>` block.
func TestRepositoriesPartial_TrafficInline(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	d.SetPlugin("traffic", &trafficStub{total: 2345})
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, `2.3k views in last two weeks`) {
		t.Errorf("missing traffic views row in:\n%s", got)
	}
}

// TestRepositoriesPartial_TrafficSingularNoun verifies the pluralisation
// flips to "view" (singular) when TotalViews == 1.
func TestRepositoriesPartial_TrafficSingularNoun(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	d.SetPlugin("traffic", &trafficStub{total: 1})
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if !strings.Contains(got, `1 view in last two weeks`) {
		t.Errorf("singular traffic noun missing in:\n%s", got)
	}
}

// TestRepositoriesPartial_TrafficZeroSkipped verifies that a traffic
// Result with TotalViews == 0 (Skipped, error, or genuinely zero
// traffic) does NOT emit the row.
func TestRepositoriesPartial_TrafficZeroSkipped(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	d.SetPlugin("traffic", &trafficStub{total: 0})
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if strings.Contains(got, `in last two weeks`) {
		t.Errorf("zero-views traffic row should be suppressed in:\n%s", got)
	}
}

// TestRepositoriesPartial_TrafficForeignType verifies that a non-traffic
// object placed under data.Plugins["traffic"] (defensive: should never
// happen in practice) does not crash and does not render the row.
func TestRepositoriesPartial_TrafficForeignType(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, activitySample())
	d.SetPlugin("traffic", struct{}{}) // intentionally not implementing TotalViews()
	got, err := base.RepositoriesPartial(context.Background(), newPC(d, gates()))
	if err != nil {
		t.Fatalf("RepositoriesPartial: %v", err)
	}
	if strings.Contains(got, `in last two weeks`) {
		t.Errorf("foreign traffic type should be ignored in:\n%s", got)
	}
}
