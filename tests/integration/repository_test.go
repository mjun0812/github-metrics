// Package integration_test covers M7 User Story 1 (repository template).
//
// TestRepositoryTemplate_HelloWorld_SVG exercises SC-001: render a
// repository's metrics SVG end-to-end against mocked GraphQL deps and
// assert it carries the repo identity markers.
package integration_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"

	// Side-effect imports to register the M7 repository template +
	// the core plugin pipeline + the classic template (sibling).
	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
	_ "github.com/mjun0812/github-metrics/internal/templates/repository"
)

// repositoryHelloWorld is the canned `Repository` GraphQL response
// matching the schema query at internal/githubapi/queries/repository.graphql.
const repositoryHelloWorld = `{
  "data": {
    "repository": {
      "databaseId": 1296269,
      "name": "hello-world",
      "nameWithOwner": "octocat/hello-world",
      "description": "My first repository on GitHub.",
      "stargazerCount": 80,
      "forkCount": 9,
      "watchers": { "totalCount": 7 },
      "isArchived": false,
      "primaryLanguage": { "name": "Go", "color": "#00ADD8" },
      "licenseInfo": { "name": "MIT License", "spdxId": "MIT" },
      "defaultBranchRef": { "name": "master" },
      "owner": { "__typename": "User", "login": "octocat", "avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4" },
      "issues": { "totalCount": 5 },
      "pullRequests": { "totalCount": 2 }
    }
  }
}`

func TestRepositoryTemplate_HelloWorld_SVG(t *testing.T) {
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
		Format:   "svg",
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml", res.MIME)
	}
	out := string(res.Output)
	for _, must := range []string{
		"<svg",
		"</svg>",
		`data-template="repository"`,
		"octocat/hello-world",
		// #464: base.header renders the upstream chrome fields.
		"Deployed",
		"Environment",
	} {
		if !strings.Contains(out, must) {
			t.Errorf("SVG output missing %q\nfirst 400 bytes: %s", must, out[:min(400, len(out))])
		}
	}
	// #464: the repo description is introduction content (gated by
	// `plugin_introduction`), not base.header chrome, so a plain base
	// repository render must not surface it — matching upstream's
	// base-only `metrics.repository.svg`.
	if strings.Contains(out, "My first repository on GitHub.") {
		t.Errorf("base-only repository SVG should not include the description")
	}
}

// TestRepositoryTemplate_HelloWorld_JSON_DataRepo verifies the M7
// JSON envelope extension: `data.repo` MUST be populated with the
// upstream-compatible snake_case shape when the repository template
// is selected; classic-template runs MUST omit the field.
func TestRepositoryTemplate_HelloWorld_JSON_DataRepo(t *testing.T) {
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
	var envelope struct {
		Repo *struct {
			Owner         string `json:"owner"`
			Name          string `json:"name"`
			NameWithOwner string `json:"name_with_owner"`
			Description   string `json:"description"`
			Stargazers    int    `json:"stargazers"`
			Forks         int    `json:"forks"`
			DefaultBranch string `json:"default_branch"`
			LicenseName   string `json:"license_name"`
		} `json:"repo"`
	}
	if err := json.Unmarshal(res.Output, &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Repo == nil {
		t.Fatalf("data.repo missing")
	}
	r := envelope.Repo
	if r.Owner != "octocat" || r.Name != "hello-world" || r.NameWithOwner != "octocat/hello-world" {
		t.Errorf("data.repo identity: %+v", r)
	}
	if r.Stargazers != 80 || r.Forks != 9 {
		t.Errorf("data.repo counts: stars=%d forks=%d", r.Stargazers, r.Forks)
	}
	if r.DefaultBranch != "master" {
		t.Errorf("default_branch = %q, want master", r.DefaultBranch)
	}
	if r.LicenseName != "MIT License" {
		t.Errorf("license_name = %q, want MIT License", r.LicenseName)
	}
}

// TestRepositoryTemplate_ClassicJSON_OmitsDataRepo guards the
// inverse: when the classic template runs, the JSON envelope MUST
// NOT include a `data.repo` field (the contract surface stays
// opt-in via template choice).
func TestRepositoryTemplate_ClassicJSON_OmitsDataRepo(t *testing.T) {
	t.Parallel()
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "json",
		Inputs:   map[string]any{"user": "octocat"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if strings.Contains(string(res.Output), `"repo":{`) {
		t.Errorf("classic-template JSON must omit data.repo; got: %s", string(res.Output)[:min(300, len(res.Output))])
	}
}

// TestRepositoryTemplate_HelloWorld_SVG_Golden compares the SVG
// output for the repository template via the M9 shared
// `golden.CompareSVG` helper.
func TestRepositoryTemplate_HelloWorld_SVG_Golden(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
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
		Format:   "svg",
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	golden.CompareSVG(t, res.Output, "repository/octocat_hello-world.svg")
}

// TestRepositoryTemplate_HelloWorld_JSON_Golden compares the JSON
// output via the M9 shared `golden.CompareJSON` helper.
func TestRepositoryTemplate_HelloWorld_JSON_Golden(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
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
	golden.CompareJSON(t, res.Output, "repository/octocat_hello-world.json")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
