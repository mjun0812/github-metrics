package partials

import (
	"context"
	"strings"
	"testing"

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
	t.Parallel()
	got, err := BaseHeader(context.Background(), newPC(&plugins.Repo{
		Owner: "octocat", Name: "hello-world",
		OwnerAvatar: "https://x/a.png", Description: "Test repo",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, s := range []string{`data-template="repository"`, "octocat/hello-world", "Test repo", "https://x/a.png"} {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in %q", s, got)
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
