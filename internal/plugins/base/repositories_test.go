package base_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	basepkg "github.com/mjun0812/github-metrics/internal/plugins/base"
)

// userOctocatBody is the minimal User payload the base plugin needs to
// satisfy its runUser preamble before the paging loop kicks off.
//
// 429 Phase 3: a 13-week contributionsCollection payload is embedded so
// runUser exercises the "trim to last 11 weeks" path. Weeks 0/1 should
// be discarded; weeks 2..12 (11 total) end up on Data.User.RecentContributions.
const userOctocatBody = `{
	"data": {
		"user": {
			"databaseId": 1,
			"id": "MDQ6VXNlcjE=",
			"login": "octocat",
			"name": "Octocat",
			"location": "",
			"createdAt": "2008-01-14T04:33:35Z",
			"avatarUrl": "https://avatars.githubusercontent.com/u/1?v=4",
			"followers": {"totalCount": 1555},
			"following": {"totalCount": 617},
			"watching": {"totalCount": 16},
			"sponsorshipsAsMaintainer": {"totalCount": 4},
			"organizations": {"totalCount": 3},
			"sponsorshipsAsSponsor": {"totalCount": 2},
			"starredRepositories": {"totalCount": 88},
			"issueComments": {"totalCount": 120},
			"repositoriesContributedTo": {"totalCount": 37},
			"contributionsCollection": {
				"totalCommitContributions": 7293,
				"totalPullRequestReviewContributions": 68,
				"totalPullRequestContributions": 290,
				"totalIssueContributions": 443,
				"contributionCalendar": {
					"totalContributions": 1234,
					"weeks": [
						{"firstDay": "2026-02-22", "contributionDays": [
							{"date": "2026-02-22", "contributionCount": 0, "weekday": 0, "color": "#ebedf0"},
							{"date": "2026-02-23", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-02-24", "contributionCount": 2, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-02-25", "contributionCount": 3, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-02-26", "contributionCount": 4, "weekday": 4, "color": "#40c463"},
							{"date": "2026-02-27", "contributionCount": 5, "weekday": 5, "color": "#40c463"},
							{"date": "2026-02-28", "contributionCount": 6, "weekday": 6, "color": "#40c463"}
						]},
						{"firstDay": "2026-03-01", "contributionDays": [
							{"date": "2026-03-01", "contributionCount": 7, "weekday": 0, "color": "#30a14e"},
							{"date": "2026-03-02", "contributionCount": 8, "weekday": 1, "color": "#30a14e"},
							{"date": "2026-03-03", "contributionCount": 9, "weekday": 2, "color": "#30a14e"},
							{"date": "2026-03-04", "contributionCount": 10, "weekday": 3, "color": "#216e39"},
							{"date": "2026-03-05", "contributionCount": 11, "weekday": 4, "color": "#216e39"},
							{"date": "2026-03-06", "contributionCount": 12, "weekday": 5, "color": "#216e39"},
							{"date": "2026-03-07", "contributionCount": 13, "weekday": 6, "color": "#216e39"}
						]},
						{"firstDay": "2026-03-08", "contributionDays": [
							{"date": "2026-03-08", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-03-09", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-03-10", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-03-11", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-03-12", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-03-13", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-03-14", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-03-15", "contributionDays": [
							{"date": "2026-03-15", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-03-16", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-03-17", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-03-18", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-03-19", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-03-20", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-03-21", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-03-22", "contributionDays": [
							{"date": "2026-03-22", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-03-23", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-03-24", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-03-25", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-03-26", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-03-27", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-03-28", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-03-29", "contributionDays": [
							{"date": "2026-03-29", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-03-30", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-03-31", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-04-01", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-04-02", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-04-03", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-04-04", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-04-05", "contributionDays": [
							{"date": "2026-04-05", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-04-06", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-04-07", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-04-08", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-04-09", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-04-10", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-04-11", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-04-12", "contributionDays": [
							{"date": "2026-04-12", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-04-13", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-04-14", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-04-15", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-04-16", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-04-17", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-04-18", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-04-19", "contributionDays": [
							{"date": "2026-04-19", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-04-20", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-04-21", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-04-22", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-04-23", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-04-24", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-04-25", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-04-26", "contributionDays": [
							{"date": "2026-04-26", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-04-27", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-04-28", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-04-29", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-04-30", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-05-01", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-05-02", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-05-03", "contributionDays": [
							{"date": "2026-05-03", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-05-04", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-05-05", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-05-06", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-05-07", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-05-08", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-05-09", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-05-10", "contributionDays": [
							{"date": "2026-05-10", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
							{"date": "2026-05-11", "contributionCount": 1, "weekday": 1, "color": "#9be9a8"},
							{"date": "2026-05-12", "contributionCount": 1, "weekday": 2, "color": "#9be9a8"},
							{"date": "2026-05-13", "contributionCount": 1, "weekday": 3, "color": "#9be9a8"},
							{"date": "2026-05-14", "contributionCount": 1, "weekday": 4, "color": "#9be9a8"},
							{"date": "2026-05-15", "contributionCount": 1, "weekday": 5, "color": "#9be9a8"},
							{"date": "2026-05-16", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
						]},
						{"firstDay": "2026-05-17", "contributionDays": [
							{"date": "2026-05-17", "contributionCount": 99, "weekday": 0, "color": "#216e39"},
							{"date": "2026-05-18", "contributionCount": 99, "weekday": 1, "color": "#216e39"},
							{"date": "2026-05-19", "contributionCount": 99, "weekday": 2, "color": "#216e39"},
							{"date": "2026-05-20", "contributionCount": 99, "weekday": 3, "color": "#216e39"}
						]}
					]
				}
			}
		}
	}
}`

// userPage builds a JSON userRepositories response with `count` nodes,
// the given pageInfo bits, and a known cursor signature so tests can
// assert what was sent.
func userPage(nodes int, hasNext bool, endCursor string) string {
	parts := make([]string, 0, nodes)
	for i := 0; i < nodes; i++ {
		parts = append(parts, fmt.Sprintf(
			`{"databaseId": %d, "id": "R_%d", "name": "r%d", "nameWithOwner": "octocat/r%d", "url": "https://github.com/octocat/r%d", "isPrivate": false, "isFork": false, "stargazerCount": %d, "forkCount": %d, "watchers": {"totalCount": %d}}`,
			i+1, i+1, i+1, i+1, i+1, i+1, i, 0,
		))
	}
	cursor := "null"
	if endCursor != "" {
		cursor = fmt.Sprintf("%q", endCursor)
	}
	return fmt.Sprintf(`{
		"data": {
			"user": {
				"repositories": {
					"totalCount": 250,
					"pageInfo": {"hasNextPage": %t, "endCursor": %s},
					"nodes": [%s]
				}
			}
		}
	}`, hasNext, cursor, strings.Join(parts, ","))
}

// TestPaging_BatchHalving asserts the batch-halving behavior: a 502 on
// the first call halves the batch (100 → 50) and the next call
// succeeds with the new batch; a third call completes the loop. The
// captured batch sizes (via the per-call request variables) verify the
// shrink path.
func TestPaging_BatchHalving(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})

	// Per-call cursor-aware handler:
	//   call 1: batch=100, no cursor    -> 502 (transient)
	//   call 2: batch=50 (halved)       -> success, hasNextPage=true,  endCursor="c1"
	//   call 3: batch=50, after="c1"    -> success, hasNextPage=false, endCursor=null
	type captured struct {
		batch  int
		cursor string
	}
	var seen []captured
	// Per-call handler that captures the batch + cursor from each call.
	// The first attempt returns a 200 OK with a GraphQL `errors` payload
	// whose message contains "Internal Server Error"; our isTransient
	// helper treats that as a 5xx-equivalent and halves the batch. We
	// avoid the 502 HTTP status so we don't get caught by httpx's
	// retryablehttp middleware (which would amplify the mux hits).
	mux.OnFunc("UserRepositories", func(vars map[string]any) gqlResp {
		bf, _ := vars["first"].(float64)
		cursor := ""
		if v, ok := vars["after"].(string); ok {
			cursor = v
		}
		c := captured{batch: int(bf), cursor: cursor}
		seen = append(seen, c)
		switch len(seen) {
		case 1:
			return gqlResp{Body: `{"errors":[{"message":"Internal Server Error 502"}]}`}
		case 2:
			return gqlResp{Body: userPage(50, true, "c1")}
		default:
			return gqlResp{Body: userPage(50, false, "")}
		}
	})

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat", "base": "header, repositories"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run: %v", err)
	}

	if len(seen) != 3 {
		t.Fatalf("expected 3 GraphQL calls, got %d (%+v)", len(seen), seen)
	}
	if seen[0].batch != 100 || seen[0].cursor != "" {
		t.Errorf("call#1: expected batch=100 cursor=\"\", got %+v", seen[0])
	}
	if seen[1].batch != 50 || seen[1].cursor != "" {
		t.Errorf("call#2: expected batch=50 cursor=\"\" (retry same cursor), got %+v", seen[1])
	}
	if seen[2].batch != 50 || seen[2].cursor != "c1" {
		t.Errorf("call#3: expected batch=50 cursor=c1, got %+v", seen[2])
	}

	if got := len(pc.Data.Computed.RepositoryList); got != 100 {
		t.Errorf("RepositoryList = %d nodes, want 100 (50 + 50)", got)
	}
	if len(pc.Data.SnapshotErrors()) != 0 {
		t.Errorf("unexpected errors: %v", pc.Data.SnapshotErrors())
	}
}

// TestPopulateRepositories_AggregatesNewFields anchors #429 Phase 2:
// the per-repo `releases`, `packages`, `diskUsage`, and `licenseInfo`
// data is summed (releases / packages / disk) or bucketed (license
// preference top-N) into Data.Computed.Repositories. Also verifies that
// User.ContributedTo is populated from user.repositoriesContributedTo.
func TestPopulateRepositories_AggregatesNewFields(t *testing.T) {
	t.Parallel()

	// Three repos: two MIT, one Apache, one with no license. diskUsage
	// in KB. The license preference percentage is computed against
	// repositories that actually reported a license (3, not 4) so MIT
	// is 2/3 ≈ 67% and Apache is 1/3 ≈ 33%.
	body := `{
		"data": {
			"user": {
				"repositories": {
					"totalCount": 4,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": [
						{"databaseId": 1, "id": "R_1", "name": "a", "nameWithOwner": "octocat/a", "url": "https://github.com/octocat/a", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 1024, "releases": {"totalCount": 3}, "packages": {"totalCount": 1}, "licenseInfo": {"name": "MIT License", "key": "mit"}},
						{"databaseId": 2, "id": "R_2", "name": "b", "nameWithOwner": "octocat/b", "url": "https://github.com/octocat/b", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 512, "releases": {"totalCount": 2}, "packages": {"totalCount": 0}, "licenseInfo": {"name": "MIT License", "key": "mit"}},
						{"databaseId": 3, "id": "R_3", "name": "c", "nameWithOwner": "octocat/c", "url": "https://github.com/octocat/c", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 256, "releases": {"totalCount": 5}, "packages": {"totalCount": 1}, "licenseInfo": {"name": "Apache License 2.0", "key": "apache-2.0"}},
						{"databaseId": 4, "id": "R_4", "name": "d", "nameWithOwner": "octocat/d", "url": "https://github.com/octocat/d", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 100, "releases": {"totalCount": 0}, "packages": {"totalCount": 0}, "licenseInfo": null}
					]
				}
			}
		}
	}`

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})
	mux.OnSequence("UserRepositories", gqlResp{Body: body})

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat", "base": "header, repositories"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run: %v", err)
	}

	r := pc.Data.Computed.Repositories
	if r.Count != 4 {
		t.Errorf("Repositories.Count = %d, want 4", r.Count)
	}
	if r.Releases != 10 {
		t.Errorf("Repositories.Releases = %d, want 10 (3+2+5+0)", r.Releases)
	}
	if r.Packages != 2 {
		t.Errorf("Repositories.Packages = %d, want 2 (1+0+1+0)", r.Packages)
	}
	if r.DiskUsage != 1892 {
		t.Errorf("Repositories.DiskUsage = %d, want 1892 KB (1024+512+256+100)", r.DiskUsage)
	}
	if len(r.LicensePreference) != 2 {
		t.Fatalf("LicensePreference len = %d, want 2 (%v)", len(r.LicensePreference), r.LicensePreference)
	}
	if r.LicensePreference[0].Name != "MIT License" || r.LicensePreference[0].Count != 2 {
		t.Errorf("LicensePreference[0] = %+v, want {MIT License, 2}", r.LicensePreference[0])
	}
	if pct := r.LicensePreference[0].Percent; pct < 66 || pct > 67 {
		t.Errorf("MIT percent = %v, want ~66.6", pct)
	}
	if r.LicensePreference[1].Name != "Apache License 2.0" || r.LicensePreference[1].Count != 1 {
		t.Errorf("LicensePreference[1] = %+v, want {Apache License 2.0, 1}", r.LicensePreference[1])
	}
	if pc.Data.User == nil || pc.Data.User.ContributedTo != 37 {
		t.Errorf("User.ContributedTo = %v, want 37 (got user=%+v)", pc.Data.User.ContributedTo, pc.Data.User)
	}
}

// TestPopulateRepositories_AllLicensesNil keeps the License-preference
// path quiet when no repo reported a licenseInfo, so the partial hides
// the row.
func TestPopulateRepositories_AllLicensesNil(t *testing.T) {
	t.Parallel()

	body := `{
		"data": {
			"user": {
				"repositories": {
					"totalCount": 2,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": [
						{"databaseId": 1, "id": "R_1", "name": "a", "nameWithOwner": "octocat/a", "url": "https://github.com/octocat/a", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 0, "releases": {"totalCount": 0}, "packages": {"totalCount": 0}, "licenseInfo": null},
						{"databaseId": 2, "id": "R_2", "name": "b", "nameWithOwner": "octocat/b", "url": "https://github.com/octocat/b", "isPrivate": false, "isFork": false, "stargazerCount": 0, "forkCount": 0, "watchers": {"totalCount": 0}, "diskUsage": 0, "releases": {"totalCount": 0}, "packages": {"totalCount": 0}, "licenseInfo": null}
					]
				}
			}
		}
	}`
	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})
	mux.OnSequence("UserRepositories", gqlResp{Body: body})

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat", "base": "header, repositories"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run: %v", err)
	}
	if got := pc.Data.Computed.Repositories.LicensePreference; got != nil {
		t.Errorf("LicensePreference = %v, want nil when no repo has a license", got)
	}
}

// TestPaging_PartialFailure asserts the degraded path when even
// batch=1 keeps failing: the accumulator collected so far is preserved
// on Computed.RepositoryList, a *RetryableError is recorded on
// Data.Errors, and Run still completes with (nil, nil).
func TestPaging_PartialFailure(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})

	// 1st call (batch=100, no cursor) succeeds with 2 nodes +
	// hasNextPage=true. Subsequent calls (with cursor) return a 200 OK
	// GraphQL error payload containing "Internal Server Error" so the
	// helper halves the batch all the way down to 1 before giving up.
	mux.OnFunc("UserRepositories", func(vars map[string]any) gqlResp {
		if _, hasCursor := vars["after"].(string); !hasCursor {
			return gqlResp{Body: userPage(2, true, "c1")}
		}
		return gqlResp{Body: `{"errors":[{"message":"Internal Server Error: upstream blip"}]}`}
	})

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat", "base": "header, repositories"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run should not surface paging error: %v", err)
	}

	if got := len(pc.Data.Computed.RepositoryList); got != 2 {
		t.Errorf("RepositoryList = %d nodes, want 2 (partial accumulator)", got)
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) == 0 {
		t.Fatalf("expected at least one *RetryableError on Data.Errors")
	}
	var retry *xerrors.RetryableError
	if !xerrors.As(errs[0], &retry) {
		t.Errorf("Data.Errors[0] not *RetryableError: %T (%v)", errs[0], errs[0])
	}
	if !strings.Contains(errs[0].Error(), "batch=1") {
		t.Errorf("expected error message to mention batch=1, got: %v", errs[0])
	}
}
