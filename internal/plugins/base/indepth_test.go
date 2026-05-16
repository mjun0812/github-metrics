package base_test

import (
	"context"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	basepkg "github.com/mjun0812/github-metrics/internal/plugins/base"
)

// userIndepthBody is the canned indepth response: 1 repo with 42
// commits / 7 issues / 3 PRs, plus a contribution calendar with one
// week of data.
const userIndepthBody = `{
	"data": {
		"user": {
			"contributionsCollection": {
				"contributionCalendar": {
					"totalContributions": 1234,
					"weeks": [
						{
							"firstDay": "2026-01-05",
							"contributionDays": [
								{"date": "2026-01-05", "contributionCount": 3, "weekday": 0, "color": "#abcdef"},
								{"date": "2026-01-06", "contributionCount": 5, "weekday": 1, "color": "#abcdef"}
							]
						}
					]
				}
			},
			"repositories": {
				"totalCount": 1,
				"pageInfo": {"hasNextPage": false, "endCursor": null},
				"nodes": [
					{
						"nameWithOwner": "octocat/alpha",
						"defaultBranchRef": {
							"target": {"__typename": "Commit", "history": {"totalCount": 42}}
						},
						"issues": {"totalCount": 7},
						"pullRequests": {"totalCount": 3}
					}
				]
			}
		}
	}
}`

// trivialRepositoryPage is the minimal UserRepositories response so the
// indepth tests can satisfy base.runUser's paging stage before
// runIndepth fires.
const trivialRepositoryPage = `{
	"data": {
		"user": {
			"repositories": {
				"totalCount": 1,
				"pageInfo": {"hasNextPage": false, "endCursor": null},
				"nodes": [
					{
						"databaseId": 1, "id": "R_1", "name": "alpha",
						"nameWithOwner": "octocat/alpha",
						"url": "https://github.com/octocat/alpha",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 0, "forkCount": 0,
						"watchers": {"totalCount": 0}
					}
				]
			}
		}
	}
}`

// TestIndepth_TriggerMatrix asserts the trigger conditions documented
// in contracts/plugin-base-extension.md §2.1.
func TestIndepth_TriggerMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		inputs      map[string]any
		wantTrigger bool
	}{
		{name: "no_flags", inputs: map[string]any{"user": "octocat"}, wantTrigger: false},
		{name: "plugin_repositories_pinned", inputs: map[string]any{"user": "octocat", "plugin_repositories_pinned": true}, wantTrigger: true},
		{name: "plugin_isocalendar", inputs: map[string]any{"user": "octocat", "plugin_isocalendar": true}, wantTrigger: true},
		{name: "plugin_habits", inputs: map[string]any{"user": "octocat", "plugin_habits": true}, wantTrigger: true},
		{name: "plugin_notable_indepth_alone_no_trigger", inputs: map[string]any{"user": "octocat", "plugin_notable_indepth": true}, wantTrigger: false},
		{name: "plugin_notable_plus_indepth_triggers", inputs: map[string]any{"user": "octocat", "plugin_notable": true, "plugin_notable_indepth": true}, wantTrigger: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newGraphQLMux()
			mux.OnSequence("User", gqlResp{Body: userOctocatBody})
			mux.OnSequence("UserRepositories", gqlResp{Body: trivialRepositoryPage})
			mux.OnSequence("UserIndepth", gqlResp{Body: userIndepthBody})

			pc := newPCWithGraphQL(t, mux)
			pc.Data.Account = plugins.AccountUser
			pc.Inputs = tc.inputs

			if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
				t.Fatalf("base.Run: %v", err)
			}

			got := mux.Calls("UserIndepth") > 0
			if got != tc.wantTrigger {
				t.Fatalf("UserIndepth calls=%d, want trigger=%v", mux.Calls("UserIndepth"), tc.wantTrigger)
			}

			if tc.wantTrigger {
				if pc.Data.Computed.TotalCommits != 42 {
					t.Errorf("TotalCommits = %d, want 42", pc.Data.Computed.TotalCommits)
				}
				if pc.Data.Computed.ContributionCalendar == nil {
					t.Fatalf("ContributionCalendar nil after indepth trigger")
				}
				if pc.Data.Computed.ContributionCalendar.TotalContributions != 1234 {
					t.Errorf("Calendar.TotalContributions = %d", pc.Data.Computed.ContributionCalendar.TotalContributions)
				}
			}
		})
	}
}

// TestIndepth_DegradedPath: indepth GraphQL returns a "5xx"-like error
// body and base.Run still returns nil; base-standard fields stay
// populated and a *RetryableError lands on Data.Errors.
func TestIndepth_DegradedPath(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})
	mux.OnSequence("UserRepositories", gqlResp{Body: trivialRepositoryPage})
	mux.OnSequence("UserIndepth", gqlResp{Body: `{"errors":[{"message":"Internal Server Error: indepth blip"}]}`})

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{
		"user":               "octocat",
		"plugin_isocalendar": true,
	}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run should not surface indepth failure: %v", err)
	}

	// base-standard fields stay populated...
	if pc.Data.User == nil || pc.Data.User.Login != "octocat" {
		t.Errorf("standard Data.User missing after indepth failure: %+v", pc.Data.User)
	}
	if len(pc.Data.Computed.RepositoryList) != 1 {
		t.Errorf("standard RepositoryList missing after indepth failure: %v", pc.Data.Computed.RepositoryList)
	}
	// ...indepth-only fields stay zero...
	if pc.Data.Computed.TotalCommits != 0 {
		t.Errorf("TotalCommits = %d, want 0 (indepth failed)", pc.Data.Computed.TotalCommits)
	}
	if pc.Data.Computed.ContributionCalendar != nil {
		t.Errorf("ContributionCalendar should stay nil after indepth failure")
	}
	// ...and the error is recorded for engine.collectPluginErrors.
	errs := pc.Data.SnapshotErrors()
	if len(errs) == 0 {
		t.Fatalf("expected indepth degraded error on Data.Errors")
	}
	var retry *xerrors.RetryableError
	if !xerrors.As(errs[0], &retry) {
		t.Errorf("Data.Errors[0] not *RetryableError: %T (%v)", errs[0], errs[0])
	}
	if !strings.Contains(errs[0].Error(), "indepth") {
		t.Errorf("expected error to mention indepth, got %v", errs[0])
	}
}
