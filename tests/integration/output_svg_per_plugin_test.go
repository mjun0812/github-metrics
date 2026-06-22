// Package integration_test — per-plugin SVG golden regression suite.
//
// Each adopted plugin gets one subtest that drives engine.Compute with
// minimal fixture data and compares the resulting SVG against a golden
// file in tests/golden/classic/plugin-<slug>.svg.
//
// Run with -update to bake initial goldens:
//
//	go test ./tests/integration/... -run TestComputeSVG_PerPluginGolden -update
//
// Then re-run without -update to confirm idempotence.
//
// # Excluded plugins
//
//   - contributors: repository-mode only; tests/integration/repo_mode_test.go
//     covers repository-mode end-to-end. A follow-up PR will add a
//     per-plugin golden for repository-mode plugins.
//
//   - topics: uses HTML scraping (https://github.com/stars/{user}/topics)
//     via a Navigator interface seam. Included below using a fake Navigator
//     that returns an empty list so the partial renders the empty state.
//
//   - starlists: uses GraphQL UserLists via a Navigator seam. Included
//     below using a fake Navigator injected via starlists.NavigatorKey.
package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"

	// Register classic template.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// fakeTopicsNavigator is a test double for the topics.Navigator interface.
type fakeTopicsNavigator struct{}

func (f *fakeTopicsNavigator) Fetch(_ context.Context, _ string) ([]topics.Topic, error) {
	return []topics.Topic{}, nil
}

// fakeStarlistsNavigator is a test double for the starlists.Navigator interface.
type fakeStarlistsNavigator struct{}

func (f *fakeStarlistsNavigator) FetchLists(_ context.Context, _ string) ([]starlists.Starlist, error) {
	return []starlists.Starlist{}, nil
}

func (f *fakeStarlistsNavigator) FetchRepos(_ context.Context, _ string) ([]string, error) {
	return []string{}, nil
}

type perPluginCase struct {
	name        string
	slug        string
	extraInputs map[string]any
	fixtures    map[string]string // GraphQL op name -> response body
	useREST     bool
	restSetup   func(*mocks.RESTMux)
	goldenPath  string
}

var perPluginCases = []perPluginCase{
	{
		name:       "achievements",
		slug:       "achievements",
		fixtures:   map[string]string{},
		goldenPath: "classic/plugin-achievements.svg",
	},
	{
		name:     "activity",
		slug:     "activity",
		fixtures: map[string]string{},
		useREST:  true,
		restSetup: func(m *mocks.RESTMux) {
			// activity paginates /users/{login}/events; empty first page terminates loop.
			m.OnBody("/users/octocat/events", 200, "[]")
		},
		goldenPath: "classic/plugin-activity.svg",
	},
	{
		name: "calendar",
		slug: "calendar",
		// calendar and isocalendar trigger the base.UserIndepth query (for the
		// contribution calendar); habits triggers it too (for commit history).
		// repositories triggers it when plugin_repositories_pinned is set.
		fixtures: map[string]string{
			"UserIsocalendar": `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[]}}}}}`,
			"UserIndepth":     `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":0,"weeks":[]}},"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-calendar.svg",
	},
	{
		name: "habits",
		slug: "habits",
		fixtures: map[string]string{
			"UserIndepth": `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":0,"weeks":[]}},"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`,
		},
		useREST: true,
		restSetup: func(m *mocks.RESTMux) {
			// habits pages /users/{login}/events; empty first page terminates loop.
			m.OnBody("/users/octocat/events", 200, "[]")
		},
		goldenPath: "classic/plugin-habits.svg",
	},
	{
		name: "isocalendar",
		slug: "isocalendar",
		fixtures: map[string]string{
			"UserIsocalendar": `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[]}}}}}`,
			"UserIndepth":     `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":0,"weeks":[]}},"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-isocalendar.svg",
	},
	{
		name:       "languages",
		slug:       "languages",
		fixtures:   map[string]string{},
		goldenPath: "classic/plugin-languages.svg",
	},
	{
		name: "notable",
		slug: "notable",
		fixtures: map[string]string{
			"UserNotable": `{"data":{"user":{"repositoriesContributedTo":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-notable.svg",
	},
	{
		name: "people",
		slug: "people",
		fixtures: map[string]string{
			"UserFollowers": `{"data":{"user":{"followers":{"totalCount":0,"nodes":[]},"following":{"totalCount":0,"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-people.svg",
	},
	{
		name: "projects",
		slug: "projects",
		fixtures: map[string]string{
			"ViewerProjects": `{"data":{"viewer":{"projectsV2":{"totalCount":0,"nodes":[]}}}}`,
		},
		useREST: true,
		restSetup: func(m *mocks.RESTMux) {
			// projects checks read:project scope via REST.Scopes() → GET /
			m.OnHeader("/", 200, `{}`, map[string][]string{
				"X-OAuth-Scopes": {"read:project, repo"},
			})
		},
		goldenPath: "classic/plugin-projects.svg",
	},
	{
		name: "reactions",
		slug: "reactions",
		fixtures: map[string]string{
			"UserReactions": `{"data":{"user":{"issues":{"totalCount":0,"nodes":[]},"issueComments":{"totalCount":0,"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-reactions.svg",
	},
	{
		name: "repositories",
		slug: "repositories",
		fixtures: map[string]string{
			// plugin_repositories_pinned triggers base.UserIndepth (see base/indepth.go).
			"UserIndepth":       `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":0,"weeks":[]}},"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`,
			"ViewerPinnedItems": `{"data":{"viewer":{"pinnedItems":{"totalCount":0,"nodes":[]}}}}`,
		},
		extraInputs: map[string]any{
			"plugin_repositories_pinned": "yes",
		},
		useREST: true,
		restSetup: func(m *mocks.RESTMux) {
			// repositories.starred fetches /users/{login}/starred when enabled
			m.OnBody("/users/octocat/starred", 200, "[]")
		},
		goldenPath: "classic/plugin-repositories.svg",
	},
	{
		name: "sponsors",
		slug: "sponsors",
		fixtures: map[string]string{
			"ViewerSponsors": `{"data":{"viewer":{"sponsorsListing":null,"active":{"totalCount":0,"nodes":[]},"past":{"totalCount":0,"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-sponsors.svg",
	},
	{
		name: "sponsorships",
		slug: "sponsorships",
		fixtures: map[string]string{
			"ViewerSponsorships": `{"data":{"viewer":{"totalSponsorshipAmountAsSponsorInCents":null,"sponsorshipsAsSponsor":{"totalCount":0,"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-sponsorships.svg",
	},
	{
		name: "stargazers",
		slug: "stargazers",
		fixtures: map[string]string{
			"ViewerStargazersRepos": `{"data":{"viewer":{"repositories":{"totalCount":0,"nodes":[]}}}}`,
		},
		goldenPath: "classic/plugin-stargazers.svg",
	},
	{
		name: "starlists",
		slug: "starlists",
		extraInputs: map[string]any{
			// extras.metrics.run.puppeteer.scrapping must be enabled for starlists to run
			"extras.metrics.run.puppeteer.scrapping": "yes",
			// inject test-seam Navigator instead of hitting GraphQL
			starlists.NavigatorKey: &fakeStarlistsNavigator{},
		},
		fixtures:   map[string]string{},
		goldenPath: "classic/plugin-starlists.svg",
	},
	{
		name: "stars",
		slug: "stars",
		fixtures: map[string]string{
			"UserStarredRepositories": `{"data":{"user":{"starredRepositories":{"totalCount":0,"edges":[]}}}}`,
		},
		goldenPath: "classic/plugin-stars.svg",
	},
	{
		name: "topics",
		slug: "topics",
		extraInputs: map[string]any{
			// extras.metrics.run.puppeteer.scrapping must be enabled for topics to run
			"extras.metrics.run.puppeteer.scrapping": "yes",
			// inject test-seam Navigator instead of scraping github.com
			topics.NavigatorKey: &fakeTopicsNavigator{},
		},
		fixtures:   map[string]string{},
		goldenPath: "classic/plugin-topics.svg",
	},
	{
		name:     "traffic",
		slug:     "traffic",
		fixtures: map[string]string{},
		useREST:  true,
		restSetup: func(m *mocks.RESTMux) {
			// traffic checks `repo` scope via REST.Scopes() → GET /
			m.OnHeader("/", 200, `{}`, map[string][]string{
				"X-OAuth-Scopes": {"repo"},
			})
			// traffic fetches views for every repo in Computed.RepositoryList.
			// The base fixture (user_repositories_250) has octocat/alpha and octocat/beta.
			m.OnBody("/repos/octocat/alpha/traffic/views", 200, `{"count":0,"uniques":0}`)
			m.OnBody("/repos/octocat/beta/traffic/views", 200, `{"count":0,"uniques":0}`)
		},
		goldenPath: "classic/plugin-traffic.svg",
	},
}

func TestComputeSVG_PerPluginGolden(t *testing.T) {
	for _, tc := range perPluginCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engine.SetVersionForTest(t, "test-version")
			restore := partials.SetNowForTest(func() time.Time {
				return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
			})
			t.Cleanup(restore)

			fixtures := map[string]string{
				"User":             userOctocat,
				"UserRepositories": userRepositories250,
			}
			for k, v := range tc.fixtures {
				fixtures[k] = v
			}

			var deps engine.Deps
			if tc.useREST {
				deps, _ = newEngineDepsWithREST(t, fixtures, tc.restSetup)
			} else {
				deps, _ = newEngineDeps(t, fixtures)
			}

			inputs := map[string]any{
				"plugin_" + tc.slug: "yes",
			}
			for k, v := range tc.extraInputs {
				inputs[k] = v
			}

			res, err := engine.Compute(context.Background(), engine.Request{
				Login:    "octocat",
				Template: "classic",
				Format:   "svg",
				Inputs:   inputs,
			}, deps)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			golden.CompareSVG(t, res.Output, tc.goldenPath)
		})
	}
}
