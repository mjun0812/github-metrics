package base_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	basepkg "github.com/mjun0812/github-metrics/internal/plugins/base"
)

const orgGitHubBody = `{
	"data": {
		"organization": {
			"databaseId": 9919,
			"id": "MDEyOk9yZ2FuaXphdGlvbjk5MTk=",
			"login": "github",
			"name": "GitHub",
			"location": null,
			"createdAt": "2008-04-10T00:00:00Z",
			"avatarUrl": "https://avatars.githubusercontent.com/u/9919?v=4",
			"websiteUrl": "https://github.com",
			"email": null,
			"description": "How people build software."
		}
	}
}`

// orgMembersPage builds a one-page OrganizationMembers response.
func orgMembersPage(count int, hasNext bool, endCursor string) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf(
			`{"login":"member%d","name":"Member %d","avatarUrl":"https://example.com/m%d.png"}`,
			i+1, i+1, i+1,
		))
	}
	cursor := "null"
	if endCursor != "" {
		cursor = fmt.Sprintf("%q", endCursor)
	}
	return fmt.Sprintf(`{
		"data": {
			"organization": {
				"membersWithRole": {
					"totalCount": 100,
					"pageInfo": {"hasNextPage": %t, "endCursor": %s},
					"nodes": [%s]
				}
			}
		}
	}`, hasNext, cursor, strings.Join(parts, ","))
}

func orgRepositoriesPage(count int, hasNext bool, endCursor string) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf(
			`{"databaseId":%d,"id":"R_%d","name":"r%d","nameWithOwner":"github/r%d","url":"https://github.com/github/r%d","isPrivate":false,"isFork":false,"stargazerCount":%d,"forkCount":0,"watchers":{"totalCount":0}}`,
			i+1, i+1, i+1, i+1, i+1, i+1,
		))
	}
	cursor := "null"
	if endCursor != "" {
		cursor = fmt.Sprintf("%q", endCursor)
	}
	return fmt.Sprintf(`{
		"data": {
			"organization": {
				"repositories": {
					"totalCount": 250,
					"pageInfo": {"hasNextPage": %t, "endCursor": %s},
					"nodes": [%s]
				}
			}
		}
	}`, hasNext, cursor, strings.Join(parts, ","))
}

// TestRun_Organization asserts the organization-branch end-to-end:
//   - Data.Account flips to AccountOrganization
//   - Data.Organization is populated (Login + Description + counts)
//   - Members paging walks 3 pages and accumulates 100 entries
//   - Repository paging walks 3 pages and accumulates 250 entries
func TestRun_Organization(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("Organization", gqlResp{Body: orgGitHubBody})
	mux.OnSequence(
		"OrganizationMembers",
		gqlResp{Body: orgMembersPage(50, true, "m1")},
		gqlResp{Body: orgMembersPage(50, false, "")},
	)
	mux.OnSequence(
		"OrganizationRepositories",
		gqlResp{Body: orgRepositoriesPage(100, true, "r1")},
		gqlResp{Body: orgRepositoriesPage(100, true, "r2")},
		gqlResp{Body: orgRepositoriesPage(50, false, "")},
	)

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountOrganization
	pc.Inputs = map[string]any{"user": "github"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run organization: %v", err)
	}

	if pc.Data.Account != plugins.AccountOrganization {
		t.Errorf("Account = %q", pc.Data.Account)
	}
	if pc.Data.Organization == nil {
		t.Fatalf("Data.Organization is nil")
	}
	org := pc.Data.Organization
	if org.Login != "github" {
		t.Errorf("Organization.Login = %q", org.Login)
	}
	if org.Description != "How people build software." {
		t.Errorf("Organization.Description = %q", org.Description)
	}
	if org.MembersCount != 100 {
		t.Errorf("Organization.MembersCount = %d, want 100", org.MembersCount)
	}
	if len(org.Members) != 100 {
		t.Errorf("Organization.Members count = %d, want 100", len(org.Members))
	}
	if got := pc.Data.Computed.Repositories.Count; got != 250 {
		t.Errorf("Computed.Repositories.Count = %d, want 250", got)
	}
	if got := len(pc.Data.Computed.RepositoryList); got != 250 {
		t.Errorf("Computed.RepositoryList = %d, want 250", got)
	}
	if mux.Calls("OrganizationMembers") != 2 {
		t.Errorf("OrganizationMembers calls = %d, want 2", mux.Calls("OrganizationMembers"))
	}
	if mux.Calls("OrganizationRepositories") != 3 {
		t.Errorf("OrganizationRepositories calls = %d, want 3", mux.Calls("OrganizationRepositories"))
	}
}

// TestRun_User asserts the user-branch end-to-end: standard user
// payload + full repository paging (T018).
func TestRun_User(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})
	mux.OnSequence(
		"UserRepositories",
		gqlResp{Body: userPage(100, true, "u1")},
		gqlResp{Body: userPage(100, true, "u2")},
		gqlResp{Body: userPage(50, false, "")},
	)

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run user: %v", err)
	}

	if pc.Data.User == nil || pc.Data.User.Login != "octocat" {
		t.Fatalf("Data.User = %+v", pc.Data.User)
	}
	// 429 Phase 1: User struct picks up the foundational counters
	// surfaced by base.header / base.repositories partials.
	if pc.Data.User.Followers != 1555 {
		t.Errorf("Data.User.Followers = %d, want 1555", pc.Data.User.Followers)
	}
	if pc.Data.User.Following != 617 {
		t.Errorf("Data.User.Following = %d, want 617", pc.Data.User.Following)
	}
	if pc.Data.User.Watching != 16 {
		t.Errorf("Data.User.Watching = %d, want 16", pc.Data.User.Watching)
	}
	if pc.Data.User.SponsorshipsAsMaintainer != 4 {
		t.Errorf("Data.User.SponsorshipsAsMaintainer = %d, want 4", pc.Data.User.SponsorshipsAsMaintainer)
	}
	if pc.Data.User.CreatedAt.Year() != 2008 {
		t.Errorf("Data.User.CreatedAt = %v, want 2008-01-14", pc.Data.User.CreatedAt)
	}
	if got := pc.Data.Computed.Repositories.Count; got != 250 {
		t.Errorf("Computed.Repositories.Count = %d, want 250", got)
	}
	if got := len(pc.Data.Computed.RepositoryList); got != 250 {
		t.Errorf("Computed.RepositoryList = %d, want 250 (post paging)", got)
	}
	if mux.Calls("UserRepositories") != 3 {
		t.Errorf("UserRepositories calls = %d, want 3", mux.Calls("UserRepositories"))
	}
	// No indepth-dependent flags set → indepth must NOT fire.
	if mux.Calls("UserIndepth") != 0 {
		t.Errorf("UserIndepth fired despite no trigger flags: %d", mux.Calls("UserIndepth"))
	}
}

// TestRun_User_PopulatesRecentContributions anchors #436: runUser must
// flatten `contributionsCollection.contributionCalendar.weeks` into a
// chronological day list and keep only the trailing 14 days on
// Data.User.RecentContributions (mirroring upstream `slice(-14)`),
// discarding the older days. The fake payload carries 13 weeks (88 days,
// the last week a 4-day partial), so the trailing-14 window spans the
// last 3 days of the 2026-05-03 week, the full 2026-05-10 week, and the
// 4-day 2026-05-17 partial week.
//
// The trigger semantics matter: indepth is NOT enabled here, so the
// calendar must land on Data.User regardless of which other plugins
// are active — this is the headline guarantee of the Phase 3 design.
func TestRun_User_PopulatesRecentContributions(t *testing.T) {
	t.Parallel()

	mux := newGraphQLMux()
	mux.OnSequence("User", gqlResp{Body: userOctocatBody})
	mux.OnSequence(
		"UserRepositories",
		gqlResp{Body: userPage(100, true, "u1")},
		gqlResp{Body: userPage(100, true, "u2")},
		gqlResp{Body: userPage(50, false, "")},
	)

	pc := newPCWithGraphQL(t, mux)
	pc.Data.Account = plugins.AccountUser
	pc.Inputs = map[string]any{"user": "octocat"}

	if _, err := basepkg.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("base.Run user: %v", err)
	}

	if pc.Data.User == nil {
		t.Fatalf("Data.User nil")
	}
	days := pc.Data.User.RecentContributions
	if len(days) != 14 {
		t.Fatalf("RecentContributions len = %d, want 14 (trailing-14-day slice)", len(days))
	}
	// First retained day is 14 days back from the end: the 2026-05-03
	// week is the boundary, so its 2026-05-07 day leads the window.
	if got := days[0].Date; got != "2026-05-07" {
		t.Errorf("first retained day = %q, want 2026-05-07", got)
	}
	// Last retained day is the final day of the 4-day partial week, with
	// color/count fidelity preserved from the GraphQL payload.
	last := days[len(days)-1]
	if last.Date != "2026-05-20" {
		t.Errorf("last day Date = %q, want 2026-05-20", last.Date)
	}
	if last.Color != "#216e39" {
		t.Errorf("last day Color = %q, want #216e39", last.Color)
	}
	if last.ContributionCount != 99 {
		t.Errorf("last day ContributionCount = %d, want 99", last.ContributionCount)
	}
	// indepth must NOT fire — base now sources calendar on its own.
	if mux.Calls("UserIndepth") != 0 {
		t.Errorf("UserIndepth fired despite no trigger flags: %d", mux.Calls("UserIndepth"))
	}
}
