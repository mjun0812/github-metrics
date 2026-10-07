// Package integration_test covers the M7 per-plugin Mode-tag contract:
// under the repository template, the reused plugins tag `mode` on
// their Result.
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
// per plugin so this single test scales across all 6 affected
// plugins without 6 type assertions.
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

// TestRepoMode_TagModeRepo (M7 contract §5): under the repository
// template, at least one of the 6 mode-tagging plugins (activity,
// contributors, languages, people, sponsors, stargazers) reaches its
// success path and tags `mode == "repo"`.
func TestRepoMode_TagModeRepo(t *testing.T) {
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
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	any := false
	for _, slug := range []string{
		"activity", "contributors", "languages",
		"people", "sponsors", "stargazers",
	} {
		if modeOf(t, res.Output, slug) == "repo" {
			any = true
			break
		}
	}
	if !any {
		t.Errorf("no plugin tagged Mode=\"repo\" — mode-tagging contract not exercised")
	}
}
