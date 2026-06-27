package header_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// newPC returns a minimal PartialContext seeded with the given Data.
func newPC(d *plugins.Data) *templates.PartialContext {
	return &templates.PartialContext{Data: d}
}

// putResult writes a *header.Result under data.Plugins[header.Name] so
// header.Partial sees it on lookup.
func putResult(d *plugins.Data, r *header.Result) {
	d.SetPlugin(header.Name, r)
}

func TestPartial_NilContext(t *testing.T) {
	t.Parallel()
	got, err := header.Partial(context.Background(), nil)
	if err != nil || got != "" {
		t.Fatalf("Partial(nil) = %q, %v; want \"\", nil", got, err)
	}
}

func TestPartial_NilData(t *testing.T) {
	t.Parallel()
	got, err := header.Partial(context.Background(), &templates.PartialContext{})
	if err != nil || got != "" {
		t.Fatalf("Partial(nil data) = %q, %v; want \"\", nil", got, err)
	}
}

func TestPartial_MissingPluginEntry(t *testing.T) {
	t.Parallel()
	got, err := header.Partial(context.Background(), newPC(plugins.NewData()))
	if err != nil || got != "" {
		t.Fatalf("Partial(missing plugin) = %q, %v; want \"\", nil", got, err)
	}
}

func TestPartial_NilProfile(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{}) // Profile is nil
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(nil profile) = %q, %v; want \"\", nil", got, err)
	}
}

func TestPartial_WrongResultType(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(header.Name, "not a *Result")
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(wrong type) = %q, %v; want \"\", nil", got, err)
	}
}

// TestPartial_UserPopulatedEscapesName verifies the user-mode header
// emits <section data-section="header">, escapes the display name, and
// inlines the avatar URL.
func TestPartial_UserPopulatedEscapesName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:     "octocat",
				Name:      "<Octo & cat>",
				AvatarURL: "https://example/x.png",
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		`data-section="header"`,
		"&lt;Octo &amp; cat&gt;",
		`src="https://example/x.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

// TestPartial_UserCountersAndAge anchors the joined / followers /
// following / contributed-to rows together with a 14-day calendar.
func TestPartial_UserCountersAndAge(t *testing.T) {
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()

	// Build a 14-day calendar so ContributionRow emits a non-empty row.
	week := plugins.ContributionWeek{
		Days: []plugins.ContributionDay{
			{Date: "2026-01-01", ContributionCount: 1, Color: "#196127"},
			{Date: "2026-01-02", ContributionCount: 2, Color: "#196127"},
			{Date: "2026-01-03", ContributionCount: 0, Color: "#ebedf0"},
			{Date: "2026-01-04", ContributionCount: 3, Color: "#196127"},
			{Date: "2026-01-05", ContributionCount: 4, Color: "#196127"},
			{Date: "2026-01-06", ContributionCount: 5, Color: "#196127"},
			{Date: "2026-01-07", ContributionCount: 6, Color: "#196127"},
		},
	}
	cal := &plugins.ContributionCalendar{Weeks: []plugins.ContributionWeek{week, week}}

	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:         "octocat",
				Name:          "Octo",
				AvatarURL:     "https://example/x.png",
				CreatedAt:     time.Date(2008, 1, 14, 4, 33, 35, 0, time.UTC),
				Followers:     1555,
				Following:     617,
				ContributedTo: 42,
			},
		},
		CommitCalendar: cal,
	})

	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		`data-block="header-counters"`,
		"Joined GitHub 18 years ago",
		"Followed by 1.6k users",
		"Following 617 users",
		"Contributed to 42 repositories",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

// TestPartial_ContributedToSingular checks the singular noun form.
func TestPartial_ContributedToSingular(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:         "octocat",
				ContributedTo: 1,
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "Contributed to 1 repository") {
		t.Errorf("expected singular 'repository', got: %s", got)
	}
}

// TestPartial_HidesZeroCounters confirms zero followers / following /
// contributed-to do not surface rows.
func TestPartial_HidesZeroCounters(t *testing.T) {
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()

	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:     "freshie",
				CreatedAt: time.Date(2025, 11, 14, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "Joined GitHub 2 months ago") {
		t.Errorf("missing Joined GitHub label: %s", got)
	}
	for _, hidden := range []string{"Followed by", "Following ", "Contributed to"} {
		if strings.Contains(got, hidden) {
			t.Errorf("zero counter should be hidden but found %q: %s", hidden, got)
		}
	}
}

// TestPartial_EmptyLoginAndName returns "" because both identity
// fields are absent.
func TestPartial_EmptyLoginAndName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(empty user) = %q, %v; want \"\", nil", got, err)
	}
}

// TestPartial_OrgRenders verifies the organization branch emits the
// header section with escaped name + avatar.
func TestPartial_OrgRenders(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{
				Login:     "octolabs",
				Name:      "<Octo & Labs>",
				AvatarURL: "https://example/org.png",
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		`data-section="header"`,
		"&lt;Octo &amp; Labs&gt;",
		`src="https://example/org.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in org output: %s", want, got)
		}
	}
	// Org branch never emits the counter row.
	if strings.Contains(got, `data-block="header-counters"`) {
		t.Errorf("org header should not emit counters block: %s", got)
	}
}

// TestPartial_OrgNilOrganization returns "" when Profile.Kind=org but
// the Organization payload is nil.
func TestPartial_OrgNilOrganization(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{Kind: plugins.ProfileKindOrganization},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(nil org) = %q, %v; want \"\", nil", got, err)
	}
}

// TestPartial_OrgEmptyLoginAndName mirrors the user-empty case.
func TestPartial_OrgEmptyLoginAndName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	putResult(d, &header.Result{
		Profile: &plugins.Profile{
			Kind:         plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(empty org) = %q, %v; want \"\", nil", got, err)
	}
}
