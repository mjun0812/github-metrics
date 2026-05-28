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

// TestBaseHeader_PhaseTwoFields anchors #429 Phase 2: a populated
// ContributedTo counter must surface the "Contributed to N
// repositories" row alongside the Phase 1 Joined / Followed by /
// Following labels. A zero ContributedTo is hidden.
func TestBaseHeader_PhaseTwoFields(t *testing.T) {
	restore := partials.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	defer restore()

	t.Run("populated renders contributed row", func(t *testing.T) {
		d := plugins.NewData()
		d.User = &plugins.User{
			Login:         "octocat",
			Name:          "Octo",
			CreatedAt:     time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			Followers:     10,
			Following:     5,
			ContributedTo: 37,
		}
		got, err := partials.BaseHeader(context.Background(), newPC(d))
		if err != nil {
			t.Fatalf("BaseHeader: %v", err)
		}
		if !strings.Contains(got, "Contributed to 37 repositories") {
			t.Errorf("missing Contributed to row: %s", got)
		}
	})
	t.Run("singular noun when count is 1", func(t *testing.T) {
		d := plugins.NewData()
		d.User = &plugins.User{Login: "x", ContributedTo: 1}
		got, err := partials.BaseHeader(context.Background(), newPC(d))
		if err != nil {
			t.Fatalf("BaseHeader: %v", err)
		}
		if !strings.Contains(got, "Contributed to 1 repository") {
			t.Errorf("singular noun missing: %s", got)
		}
	})
	t.Run("zero is hidden", func(t *testing.T) {
		d := plugins.NewData()
		d.User = &plugins.User{Login: "x", ContributedTo: 0}
		got, err := partials.BaseHeader(context.Background(), newPC(d))
		if err != nil {
			t.Fatalf("BaseHeader: %v", err)
		}
		if strings.Contains(got, "Contributed to") {
			t.Errorf("Contributed row should be hidden when ContributedTo=0: %s", got)
		}
	})
}

// TestBaseRepositories_PhaseTwoFields anchors #429 Phase 2: with
// Releases / Packages / DiskUsage / LicensePreference populated, the
// partial surfaces "N releases", "N packages", "<disk> used", and
// "License preference: A 60% / B 20% / ..." alongside the Phase 1
// labels.
func TestBaseRepositories_PhaseTwoFields(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 20
	d.Computed.Repositories.Stargazers = 100
	d.Computed.Repositories.Forks = 5
	d.Computed.Repositories.Releases = 10
	d.Computed.Repositories.Packages = 2
	d.Computed.Repositories.DiskUsage = 5242880 // 5 GB in KB
	d.Computed.Repositories.LicensePreference = []plugins.LicenseShare{
		{Name: "MIT License", Count: 12, Percent: 60},
		{Name: "Apache License 2.0", Count: 4, Percent: 20},
		{Name: "GNU General Public License v3.0", Count: 2, Percent: 10},
	}
	d.User = &plugins.User{Login: "octocat"}

	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	for _, marker := range []string{
		"10 releases",
		"2 packages",
		"5 GB used",
		"License preference:",
		"MIT License 60%",
		"Apache License 2.0 20%",
		"GNU General Public License v3.0 10%",
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q in %s", marker, got)
		}
	}
}

// TestBaseRepositories_PhaseTwoSingulars anchors plural→singular
// noun handling on Releases / Packages.
func TestBaseRepositories_PhaseTwoSingulars(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 1
	d.Computed.Repositories.Releases = 1
	d.Computed.Repositories.Packages = 1
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	for _, marker := range []string{"1 release</div>", "1 package</div>"} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing singular %q in %s", marker, got)
		}
	}
}

// TestBaseRepositories_DiskUsageFormat exercises the KB → MB → GB
// boundary in the disk-usage label.
func TestBaseRepositories_DiskUsageFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kb   int
		want string
	}{
		{500, "500 KB used"},
		{1536, "1.5 MB used"},
		{5242880, "5 GB used"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			d := plugins.NewData()
			d.Computed.Repositories.Count = 1
			d.Computed.Repositories.DiskUsage = tc.kb
			got, err := partials.BaseRepositories(context.Background(), newPC(d))
			if err != nil {
				t.Fatalf("BaseRepositories: %v", err)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("missing %q in %s", tc.want, got)
			}
		})
	}
}

// TestBaseRepositories_LicensePreference_TopN asserts the partial
// limits the license labels to the top three entries even when the
// data model carries more, and hides the row entirely when the slice
// is empty.
func TestBaseRepositories_LicensePreference_TopN(t *testing.T) {
	t.Parallel()
	t.Run("more than 3 entries → top 3", func(t *testing.T) {
		d := plugins.NewData()
		d.Computed.Repositories.Count = 10
		d.Computed.Repositories.LicensePreference = []plugins.LicenseShare{
			{Name: "MIT License", Count: 5, Percent: 50},
			{Name: "Apache License 2.0", Count: 3, Percent: 30},
			{Name: "BSD 3-Clause", Count: 1, Percent: 10},
			{Name: "ISC License", Count: 1, Percent: 10},
		}
		got, err := partials.BaseRepositories(context.Background(), newPC(d))
		if err != nil {
			t.Fatalf("BaseRepositories: %v", err)
		}
		for _, marker := range []string{"MIT License 50%", "Apache License 2.0 30%", "BSD 3-Clause 10%"} {
			if !strings.Contains(got, marker) {
				t.Errorf("missing %q in %s", marker, got)
			}
		}
		if strings.Contains(got, "ISC License") {
			t.Errorf("4th license entry should be capped out: %s", got)
		}
	})
	t.Run("empty slice hides the row", func(t *testing.T) {
		d := plugins.NewData()
		d.Computed.Repositories.Count = 1
		got, err := partials.BaseRepositories(context.Background(), newPC(d))
		if err != nil {
			t.Fatalf("BaseRepositories: %v", err)
		}
		if strings.Contains(got, "License preference") {
			t.Errorf("license row should be hidden when slice is empty: %s", got)
		}
	})
}

// TestBaseRepositories_PhaseTwoZerosHidden confirms that with zero
// Phase 2 counters the partial does not gain any Phase 2 rows — the
// Phase 1 fields stay the only visible content.
func TestBaseRepositories_PhaseTwoZerosHidden(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Computed.Repositories.Count = 1
	got, err := partials.BaseRepositories(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseRepositories: %v", err)
	}
	for _, marker := range []string{"releases", "packages", "used", "License preference"} {
		if strings.Contains(got, marker) {
			t.Errorf("zero counter should hide %q in %s", marker, got)
		}
	}
}

// TestBaseHeader_CalendarGrid anchors #429 Phase 3: when the User
// payload carries RecentContributions the partial renders a
// `<div class="field calendar">` block that contains exactly 11 weeks
// x 7 days = 77 <rect> cells.
func TestBaseHeader_CalendarGrid(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	weeks := make([]plugins.ContributionWeek, 11)
	for w := range weeks {
		days := make([]plugins.ContributionDay, 7)
		for i := 0; i < 7; i++ {
			days[i] = plugins.ContributionDay{
				Date:              "2026-05-01",
				ContributionCount: w + i,
				Color:             "#9be9a8",
			}
		}
		weeks[w] = plugins.ContributionWeek{Days: days}
	}
	d.User = &plugins.User{
		Login:               "octocat",
		Name:                "Octo",
		RecentContributions: weeks,
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if !strings.Contains(got, `class="field calendar"`) {
		t.Errorf("calendar wrapper missing: %s", got)
	}
	if !strings.Contains(got, `data-block="calendar-grid"`) {
		t.Errorf("data-block marker missing: %s", got)
	}
	if c := strings.Count(got, "calendar-graph-day-"); c != 77 {
		t.Errorf("expected 77 cells, got %d in %s", c, got)
	}
	if c := strings.Count(got, `<rect class="calendar-graph-day-`); c != 77 {
		t.Errorf("expected 77 <rect> entries, got %d", c)
	}
	// At least one cell should carry the GitHub-supplied hex.
	if !strings.Contains(got, `fill="#9be9a8"`) {
		t.Errorf("expected GraphQL-supplied color in output: %s", got)
	}
}

// TestBaseHeader_CalendarGrid_Empty: missing data hides the block.
func TestBaseHeader_CalendarGrid_Empty(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.User = &plugins.User{
		Login: "octocat",
		Name:  "Octo",
		// RecentContributions intentionally nil.
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if strings.Contains(got, "calendar-graph-day-") {
		t.Errorf("calendar block must be hidden when data is empty: %s", got)
	}
	if strings.Contains(got, `class="field calendar"`) {
		t.Errorf("calendar wrapper must be hidden when data is empty")
	}
}

// TestBaseHeader_CalendarGrid_LessThan11Weeks: fresh account with
// fewer than 11 weeks renders only the weeks that exist, not 11 padded
// columns.
func TestBaseHeader_CalendarGrid_LessThan11Weeks(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	weeks := make([]plugins.ContributionWeek, 3)
	for w := range weeks {
		days := make([]plugins.ContributionDay, 7)
		for i := 0; i < 7; i++ {
			days[i] = plugins.ContributionDay{
				Date:  "2026-05-01",
				Color: "#ebedf0",
			}
		}
		weeks[w] = plugins.ContributionWeek{Days: days}
	}
	d.User = &plugins.User{
		Login:               "octocat",
		Name:                "Octo",
		RecentContributions: weeks,
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if c := strings.Count(got, "calendar-graph-day-"); c != 21 {
		t.Errorf("expected 21 cells (3 weeks x 7 days), got %d", c)
	}
}

// TestBaseHeader_CalendarGrid_PartialWeek: when the last week has < 7
// days (today is mid-week), the missing rows must still render as
// empty padding cells so the grid stays a clean rectangle.
func TestBaseHeader_CalendarGrid_PartialWeek(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	// Two full weeks + a 3-day trailing week.
	week := plugins.ContributionWeek{
		Days: []plugins.ContributionDay{
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
			{Color: "#9be9a8", ContributionCount: 1},
		},
	}
	partial := plugins.ContributionWeek{
		Days: []plugins.ContributionDay{
			{Color: "#40c463", ContributionCount: 5},
			{Color: "#40c463", ContributionCount: 5},
			{Color: "#40c463", ContributionCount: 5},
		},
	}
	d.User = &plugins.User{
		Login:               "octocat",
		Name:                "Octo",
		RecentContributions: []plugins.ContributionWeek{week, week, partial},
	}
	got, err := partials.BaseHeader(context.Background(), newPC(d))
	if err != nil {
		t.Fatalf("BaseHeader: %v", err)
	}
	if c := strings.Count(got, "calendar-graph-day-"); c != 21 {
		t.Errorf("expected 21 cells (3 weeks x 7 days), got %d", c)
	}
	// The 4 padding cells in the trailing column must use the empty
	// color #ebedf0.
	if c := strings.Count(got, `fill="#ebedf0"`); c != 4 {
		t.Errorf("expected 4 empty-padding cells, got %d in %s", c, got)
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
