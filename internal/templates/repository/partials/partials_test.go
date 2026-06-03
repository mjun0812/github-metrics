package partials

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

func newPC(repo *plugins.Repo) *templates.PartialContext {
	d := plugins.NewData()
	d.Repo = repo
	return &templates.PartialContext{Data: d}
}

func TestBaseHeader_NilRepoSafe(t *testing.T) {
	t.Parallel()
	got, err := BaseHeader(context.Background(), newPC(nil))
	if err != nil || got != "" {
		t.Errorf("BaseHeader nil-safe: got=%q err=%v", got, err)
	}
}

func TestBaseHeader_RepoNameAndOwner(t *testing.T) {
	got, err := BaseHeader(context.Background(), newPC(&plugins.Repo{
		Owner: "octocat", Name: "hello-world",
		OwnerAvatar: "https://x/a.png", Description: "Test repo",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Upstream base.header.ejs renders the repo identity span; the avatar
	// + description belong to the introduction partial, not the header.
	for _, s := range []string{`data-template="repository"`, "octocat/hello-world"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
		}
	}
	for _, s := range []string{"Test repo", "https://x/a.png"} {
		if strings.Contains(got, s) {
			t.Errorf("did not expect %q in repository base.header %q", s, got)
		}
	}
}

// TestBaseHeader_UpstreamFields asserts the upstream base.header content
// (Created / Deployed / disk usage / Environments / calendar) renders
// from the repo payload. #464.
func TestBaseHeader_UpstreamFields(t *testing.T) {
	restore := SetNow(func() time.Time {
		return time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	})
	defer restore()

	got, err := BaseHeader(context.Background(), newPC(&plugins.Repo{
		Owner: "mjun0812", Name: "flash-attention-prebuild-wheels",
		CreatedAt:    time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		DiskUsageKB:  18739, // ~18.3 MB
		Deployments:  127,
		Environments: 2,
		Calendar: []plugins.ContributionDay{
			{Color: "#9be9a8"}, {Color: "#40c463"},
		},
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, s := range []string{
		"Created 1 year ago",
		"Deployed 127 times",
		"used",
		"2 Environments",
		`class="field calendar"`,
		`fill="#9be9a8"`,
	} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
		}
	}
}

// TestBaseHeader_Pluralization asserts the singular/plural toggles for
// Deployed / Environments match upstream's `s()` helper. #464.
func TestBaseHeader_Pluralization(t *testing.T) {
	got, _ := BaseHeader(context.Background(), newPC(&plugins.Repo{
		Owner: "o", Name: "n", Deployments: 1, Environments: 1,
	}))
	for _, s := range []string{"Deployed 1 time<", "1 Environment<"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected singular %q in %q", s, got)
		}
	}
}

func TestIntroduction_NilRepoSafe(t *testing.T) {
	t.Parallel()
	got, err := Introduction(context.Background(), newPC(nil))
	if err != nil || got != "" {
		t.Errorf("Introduction nil-safe: got=%q err=%v", got, err)
	}
}

func TestIntroduction_EmptyMetaSkips(t *testing.T) {
	t.Parallel()
	got, _ := Introduction(context.Background(), newPC(&plugins.Repo{Owner: "x", Name: "y"}))
	if got != "" {
		t.Errorf("expected empty when no language/license/branch, got %q", got)
	}
}

func TestIntroduction_PrimaryLanguageBadge(t *testing.T) {
	t.Parallel()
	got, _ := Introduction(context.Background(), newPC(&plugins.Repo{
		PrimaryLanguage: "Go", PrimaryLanguageColor: "#00ADD8", LicenseName: "MIT", DefaultBranch: "main",
	}))
	for _, s := range []string{`badge language`, "Go", "#00ADD8", "MIT", "main"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
		}
	}
}

func TestBaseCommunity_NilOrZero(t *testing.T) {
	t.Parallel()
	if got, _ := BaseCommunity(context.Background(), newPC(nil)); got != "" {
		t.Errorf("nil should be empty")
	}
	if got, _ := BaseCommunity(context.Background(), newPC(&plugins.Repo{})); got != "" {
		t.Errorf("all-zero should be empty")
	}
}

func TestBaseCommunity_Populated(t *testing.T) {
	t.Parallel()
	got, _ := BaseCommunity(context.Background(), newPC(&plugins.Repo{Stargazers: 42, Forks: 7, Contributors: 3}))
	for _, s := range []string{`data-section="community"`, "42", "7", "3"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
		}
	}
}

func TestBaseActivity_NilOrZero(t *testing.T) {
	t.Parallel()
	if got, _ := BaseActivity(context.Background(), newPC(nil)); got != "" {
		t.Errorf("nil should be empty")
	}
	if got, _ := BaseActivity(context.Background(), newPC(&plugins.Repo{})); got != "" {
		t.Errorf("zero-activity should be empty")
	}
}

func TestBaseActivity_Populated(t *testing.T) {
	t.Parallel()
	got, _ := BaseActivity(context.Background(), newPC(&plugins.Repo{
		Activity: plugins.RepoActivity{RecentCommits: 5, OpenIssues: 2, OpenPullRequests: 1},
	}))
	for _, s := range []string{`data-section="activity"`, "5", "2", "1"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
		}
	}
}

func TestLookup_AllFourPartials(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"base.header", "introduction", "base.community", "base.activity"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%q) returned !ok; want true", name)
		}
	}
	if _, ok := Lookup("nonexistent"); ok {
		t.Errorf("Lookup of unknown partial should return false")
	}
}
