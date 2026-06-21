package header_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func newPC(d *plugins.Data) *templates.PartialContext {
	return &templates.PartialContext{Data: d}
}

func TestPartial_NilContext(t *testing.T) {
	t.Parallel()
	got, err := header.Partial(context.Background(), nil)
	if err != nil || got != "" {
		t.Fatalf("Partial(nil) = %q,%v want \"\",nil", got, err)
	}
}

func TestPartial_MissingResult(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(no result) = %q,%v want \"\",nil", got, err)
	}
}

func TestPartial_SkippedResult(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{Skipped: true})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil || got != "" {
		t.Fatalf("Partial(skipped) = %q,%v want \"\",nil", got, err)
	}
}

func TestPartial_UserBranchEscapesName(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{
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

// TestPartial_UserBranchPopulatedRows anchors the full identity-row
// rendering: Joined GitHub <age>, Followed by N users, Following N
// users. Uses SetNowForTest to anchor the rendered age string.
func TestPartial_UserBranchPopulatedRows(t *testing.T) {
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:     "octocat",
				Name:      "Octo",
				AvatarURL: "https://example/x.png",
				CreatedAt: time.Date(2008, 1, 14, 4, 33, 35, 0, time.UTC),
				Followers: 1555,
				Following: 617,
			},
		},
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
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

// TestPartial_UserBranchHidesZeroCounters confirms zero counters do
// not surface label rows; an account with only a known createdAt still
// gets the Joined row but no followers / following lines.
func TestPartial_UserBranchHidesZeroCounters(t *testing.T) {
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:     "octocat",
				CreatedAt: time.Date(2008, 1, 14, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "Joined GitHub") {
		t.Errorf("missing Joined row: %s", got)
	}
	if strings.Contains(got, "Followed by") || strings.Contains(got, "Following") {
		t.Errorf("zero counters should not render rows: %s", got)
	}
}

// TestPartial_UserBranchRendersContributedTo anchors the right-column
// "Contributed to N repositories" line for user accounts.
func TestPartial_UserBranchRendersContributedTo(t *testing.T) {
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{
				Login:         "octocat",
				ContributedTo: 7,
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "Contributed to 7 repositories") {
		t.Errorf("missing contributed-to row: %s", got)
	}
}

// TestPartial_OrganizationBranchRendersMembers anchors the org-account
// flavour: avatar.organization class, login span, member count row.
func TestPartial_OrganizationBranchRendersMembers(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(header.Name, &header.Result{
		Profile: &plugins.Profile{
			Kind: plugins.ProfileKindOrganization,
			Organization: &plugins.Organization{
				Login:        "octocorp",
				Name:         "Octo & Corp",
				AvatarURL:    "https://example/org.png",
				MembersCount: 12,
			},
		},
	})
	got, err := header.Partial(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, `avatar organization`) {
		t.Errorf("missing organization avatar class: %s", got)
	}
	if !strings.Contains(got, "Octo &amp; Corp") {
		t.Errorf("org name not escaped: %s", got)
	}
	if !strings.Contains(got, "12 members") {
		t.Errorf("missing members row: %s", got)
	}
}

// TestRun_PopulatesResult drives Plugin.Run with a CountingMock to
// verify the Result carries Profile and the trailing 14-day calendar
// slice.
func TestRun_PopulatesResult(t *testing.T) {
	mock := dataprovider.NewCountingMock()
	mock.StubProfile = &plugins.Profile{
		Kind: plugins.ProfileKindUser,
		User: &plugins.User{Login: "octocat"},
	}
	// 20 days of calendar; Run should keep only the trailing 14.
	days := make([]plugins.ContributionDay, 0, 20)
	for i := 0; i < 20; i++ {
		days = append(days, plugins.ContributionDay{
			Date:              "2026-01-",
			ContributionCount: i,
		})
	}
	mock.StubCommitCalendar = &plugins.ContributionCalendar{
		Weeks: []plugins.ContributionWeek{{Days: days}},
	}

	pc := mocks.NewPluginContext(
		t,
		mocks.WithInputs(map[string]any{
			"user":          "octocat",
			"plugin_header": "yes",
		}),
	)
	pc.Provider = mock

	raw, err := header.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := raw.(*header.Result)
	if !ok || r == nil {
		t.Fatalf("Run returned %T %v, want *header.Result", raw, raw)
	}
	if r.Profile == nil || r.Profile.Kind != plugins.ProfileKindUser {
		t.Errorf("Profile not propagated: %+v", r.Profile)
	}
	if len(r.Calendar) != 14 {
		t.Errorf("Calendar trailing slice length = %d, want 14", len(r.Calendar))
	}
}

// TestRun_NilProvider returns a skipped Result instead of panicking,
// supporting legacy harnesses that drive Plugin.Run without a Provider.
func TestRun_NilProvider(t *testing.T) {
	pc := mocks.NewPluginContext(t)
	pc.Provider = nil

	raw, err := header.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := raw.(*header.Result)
	if !ok || r == nil || !r.Skipped {
		t.Fatalf("Run(nil provider) = %T %+v, want *header.Result{Skipped:true}", raw, raw)
	}
}
