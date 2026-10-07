// Package integration_test covers M7 User Story 1 (repository template):
// the repository template's SVG and JSON output end-to-end against
// mocked GraphQL deps, pinned by golden files.
package integration_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"

	// Side-effect imports to register the M7 repository template + the
	// core plugin pipeline + the classic template (sibling).
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
		Inputs: map[string]any{
			"user":                "octocat",
			"repo":                "hello-world",
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
		},
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
		Inputs: map[string]any{
			"user":                "octocat",
			"repo":                "hello-world",
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	golden.CompareJSON(t, res.Output, "repository/octocat_hello-world.json")
}
