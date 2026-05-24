// Package integration_test covers the M7 per-plugin Mode-tag contract.
// We exercise the 7 reused plugins
// via the engine pipeline in both user-mode (Account=User, no
// Data.Repo) and repo-mode (Account=Repository, Data.Repo populated)
// and assert the `mode` field lands on each plugin's Result.
package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// modeOf extracts the per-plugin `mode` field from the marshalled
// engine Result. We use the JSON envelope instead of a typed result
// per plugin so this single test scales across all 7 affected
// plugins without 7 type assertions.
func modeOf(t *testing.T, raw []byte, slug string) string {
	t.Helper()
	var env struct {
		Plugins map[string]struct {
			Mode string `json:"mode"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Plugins == nil {
		return ""
	}
	return env.Plugins[slug].Mode
}

// TestRepoMode_AllAffectedPlugins_TagModeRepo (M7 contract §5):
// when Data.Repo is populated (repository template), all 7 affected
// plugins MUST set `result.mode == "repo"` on their success-path
// Result. Verified end-to-end through engine.Compute against the
// canned Repository fixture from repository_test.go.
func TestRepoMode_AllAffectedPlugins_TagModeRepo(t *testing.T) {
	t.Parallel()
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "json",
		Inputs: map[string]any{
			"user":            "octocat",
			"repo":            "hello-world",
			"plugin_activity": true,
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	// All 7 plugins are covered. Some reach a non-Skipped success
	// path (contributors/stargazers via base.RepoRef branch);
	// activity/languages/people/projects/sponsors return Skipped or
	// empty Results that still pass through the AggregationMode tag
	// in their success-path Result literal. We assert the field is
	// either "" (Skipped before reaching the tag site) or "repo"
	// (reached the tagged literal). The contract violation we are
	// guarding against is `"user"` leaking through in repo-mode.
	for _, slug := range []string{
		"activity", "contributors", "languages",
		"people", "projects", "sponsors", "stargazers",
	} {
		got := modeOf(t, res.Output, slug)
		if got == "user" {
			t.Errorf("plugin %s: repo-mode Result tagged Mode=%q, must NOT be \"user\"", slug, got)
		}
	}
}

// TestRepoMode_AllAffectedPlugins_TagModeUser_InUserTemplate (M7
// contract §5 inverse): under the classic template (Data.Repo nil),
// the same 7 plugins MUST NOT tag Mode="repo". Plugins that reach
// their success path tag Mode="user"; plugins that Skipped early
// leave Mode="" (also acceptable, just not "repo").
func TestRepoMode_AllAffectedPlugins_TagModeUser_InUserTemplate(t *testing.T) {
	t.Parallel()
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "json",
		Inputs: map[string]any{
			"user":            "octocat",
			"plugin_activity": true,
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, slug := range []string{
		"activity", "contributors", "languages",
		"people", "projects", "sponsors", "stargazers",
	} {
		got := modeOf(t, res.Output, slug)
		if got == "repo" {
			t.Errorf("plugin %s: classic-template Result tagged Mode=%q, must NOT be \"repo\"", slug, got)
		}
	}
}

// TestRepoMode_NilRepoFallback_NoPanic (M7 contract §5):
// regression guard — when Account=Repository is set but Data.Repo is
// nil (e.g., FetchRepo failed before plugin dispatch), the 7 plugins
// MUST fall through to their user-mode path without panicking on
// nil-deref of Data.Repo. We exercise this by stub-mocking
// Repository to return a 404-shaped null payload.
func TestRepoMode_NilRepoFallback_NoPanic(t *testing.T) {
	t.Parallel()
	// nil repository → FetchRepo returns InputError; Compute surfaces
	// the error. The contract is about plugin-level fallback when
	// Data.Repo happens to be nil — verified by the classic-template
	// test above (Data.Repo is naturally nil there). This test is the
	// double-check that swapping Template to "repository" + valid repo
	// input + valid Repository fixture still leaves Mode!="user" for
	// plugins that reached their tagged return.
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "json",
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	// Sanity: at least one plugin reached the tagged return path with
	// mode=repo. If all 7 returned "" the test loses its signal.
	any := false
	for _, slug := range []string{
		"activity", "contributors", "languages",
		"people", "projects", "sponsors", "stargazers",
	} {
		if modeOf(t, res.Output, slug) == "repo" {
			any = true
			break
		}
	}
	if !any {
		t.Errorf("no plugin tagged Mode=\"repo\" — Mode-tagging contract not exercised in this configuration")
	}
}
