package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

func TestTemplate_Registered(t *testing.T) {
	t.Parallel()
	got, ok := templates.Get(Name)
	if !ok {
		t.Fatalf("template %q not registered", Name)
	}
	if got.Name() != Name {
		t.Errorf("Get(%q).Name() = %q", Name, got.Name())
	}
}

func TestCheck_RejectsMissingRepo(t *testing.T) {
	t.Parallel()
	err := Template.Check(map[string]any{}, "repository", "svg")
	if err == nil {
		t.Fatal("expected error for missing repo input")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("error should mention 'repo'; got %v", err)
	}
}

func TestCheck_AcceptsValidInput(t *testing.T) {
	t.Parallel()
	if err := Template.Check(map[string]any{"repo": "hello-world"}, "repository", "svg"); err != nil {
		t.Errorf("Check happy path: %v", err)
	}
}

func TestCheck_RejectsNonRepositoryAccount(t *testing.T) {
	t.Parallel()
	// `user` is not in the repository template's `supports` list.
	err := Template.Check(map[string]any{"repo": "hello-world"}, "user", "svg")
	if err == nil {
		t.Errorf("expected account-rejection error")
	}
}

func TestRun_EmitsValidSVGSkeleton(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Account = plugins.AccountRepository
	d.User = &plugins.User{Login: "octocat", AvatarURL: "https://x"}
	d.Repo = &plugins.Repo{
		Owner:        "octocat",
		OwnerAvatar:  "https://x/avatar.png",
		Name:         "hello-world",
		Description:  "My first repository",
		Stargazers:   42,
		Forks:        7,
		Contributors: 3,
		Activity:     plugins.RepoActivity{RecentCommits: 5, OpenIssues: 2, OpenPullRequests: 1},
	}
	pc := &templates.PartialContext{
		Data:   d,
		Inputs: map[string]any{"repo": "hello-world"},
	}
	out, err := Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, must := range []string{
		`<svg`,
		`</svg>`,
		`data-template="repository"`,
		`octocat/hello-world`,
		`My first repository`,
	} {
		if !strings.Contains(out, must) {
			t.Errorf("Run output missing %q\nfull (truncated): %s", must, truncate(out, 400))
		}
	}
}

func TestRun_NilRepo_StillEmitsSkeleton(t *testing.T) {
	t.Parallel()
	pc := &templates.PartialContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"repo": "hello-world"},
	}
	out, err := Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Partials are nil-safe; the SVG envelope still emits.
	if !strings.HasPrefix(out, `<svg`) || !strings.HasSuffix(out, `</svg>`) {
		t.Errorf("Run output not a valid SVG skeleton; got %s", truncate(out, 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
