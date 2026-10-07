package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/people"
	"github.com/mjun0812/github-metrics/internal/templates"
)

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

func TestCheck_RejectsNonRepositoryAccount(t *testing.T) {
	t.Parallel()
	// `user` is not in the repository template's `supports` list.
	err := Template.Check(map[string]any{"repo": "hello-world"}, "user", "svg")
	if err == nil {
		t.Errorf("expected account-rejection error")
	}
}

// TestRun_NoChromeEmptyCard asserts that without any chrome_* opt-in the
// base.header section is suppressed, and that the resulting all-empty card
// still declares a valid size: a 0 height/viewBox is not a valid SVG size
// (rasterizers reject it), so the template must clamp to a minimal
// positive canvas.
func TestRun_NoChromeEmptyCard(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Account = plugins.AccountRepository
	d.Repo = &plugins.Repo{Owner: "octocat", Name: "hello-world", Deployments: 3}
	pc := &templates.PartialContext{
		Data:   d,
		Inputs: map[string]any{"repo": "hello-world"},
	}
	out, err := Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, `data-section="header"`) {
		t.Errorf("no chrome_* should suppress the base.header section; got %s", truncate(out, 400))
	}
	if strings.Contains(out, `height="0"`) || strings.Contains(out, `viewBox="0 0 480 0"`) {
		t.Fatalf("empty card must not declare a zero size; output:\n%s", truncate(out, 300))
	}
}

func TestRun_RepositoryPeopleCard(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Account = plugins.AccountRepository
	d.Repo = &plugins.Repo{Owner: "octocat", Name: "hello-world"}
	d.SetPlugin("people", &people.Result{
		Mode: plugins.ModeRepo,
		Types: map[string][]people.Person{
			"contributors": {{Login: "alice", AvatarURL: "https://avatars.example/alice.png"}},
			"stargazers":   {{Login: "bob", AvatarURL: "https://avatars.example/bob.png"}},
			"watchers":     {{Login: "carol", AvatarURL: "https://avatars.example/carol.png"}},
		},
	})
	pc := &templates.PartialContext{
		Data: d,
		// #464: plugin partials are now gated by `plugin_<slug>`; the
		// people section only renders when the toggle is on.
		Inputs: map[string]any{"repo": "hello-world", "plugin_people": "yes"},
	}

	out, err := Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, must := range []string{
		`data-section="people"`,
		`data-type="contributors"`,
		`1 contributor`,
		`1 stargazer`,
		`1 watcher`,
		`https://avatars.example/alice.png`,
		`https://avatars.example/bob.png`,
		`https://avatars.example/carol.png`,
	} {
		if !strings.Contains(out, must) {
			t.Errorf("Run output missing %q\nfull (truncated): %s", must, truncate(out, 600))
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
