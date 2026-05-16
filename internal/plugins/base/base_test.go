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
