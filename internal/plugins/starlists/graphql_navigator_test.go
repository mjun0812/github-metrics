package starlists_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
)

// fakeGQL implements the minimal graphqlClient surface used by
// graphqlNavigator (which is unexported, hence the interface is also
// unexported). We exercise the navigator via NewGraphQLNavigator.
type fakeGQL struct {
	calls int
	resp  *githubapi.UserListsResponse
	err   error
}

func (f *fakeGQL) UserLists(_ context.Context, _ string, _, _ int) (*githubapi.UserListsResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func strPtr(s string) *string { return &s }

func sampleResponse() *githubapi.UserListsResponse {
	mkRepo := func(nwo string, stars int) *githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnectionNodesRepository {
		return &githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnectionNodesRepository{
			NameWithOwner:  nwo,
			StargazerCount: stars,
		}
	}
	return &githubapi.UserListsResponse{
		User: &githubapi.UserListsUser{
			Lists: &githubapi.UserListsUserListsUserListConnection{
				TotalCount: 2,
				Nodes: []*githubapi.UserListsUserListsUserListConnectionNodesUserList{
					{
						Name:        "👍 Nice",
						Description: strPtr("My picks"),
						Items: &githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnection{
							TotalCount: 2,
							Nodes: []githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnectionNodesUserListItems{
								mkRepo("octocat/repo-a", 10),
								mkRepo("octocat/repo-b", 5),
							},
						},
					},
					{
						Name: "Backend",
						Items: &githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnection{
							TotalCount: 1,
							Nodes: []githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnectionNodesUserListItems{
								mkRepo("octocat/svc", 7),
							},
						},
					},
				},
			},
		},
	}
}

// TestGraphQLNavigator_FetchListsAndRepos drives both Navigator methods
// off a single fake GraphQL response and verifies the cache: the
// second call must not hit the client again.
func TestGraphQLNavigator_FetchListsAndRepos(t *testing.T) {
	t.Parallel()
	gql := &fakeGQL{resp: sampleResponse()}
	nav := starlists.NewGraphQLNavigator(gql, "octocat", 10, 50)

	lists, err := nav.FetchLists(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchLists: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("len(lists) = %d, want 2; %+v", len(lists), lists)
	}
	if lists[0].Name != "👍 Nice" {
		t.Errorf("lists[0].Name = %q, want emoji-prefixed", lists[0].Name)
	}
	if lists[0].Count != 2 {
		t.Errorf("lists[0].Count = %d, want 2", lists[0].Count)
	}
	if lists[0].Description != "My picks" {
		t.Errorf("lists[0].Description = %q, want 'My picks'", lists[0].Description)
	}

	repos, err := nav.FetchRepos(context.Background(), lists[0].URL)
	if err != nil {
		t.Fatalf("FetchRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}
	if repos[0] != "octocat/repo-a" {
		t.Errorf("repos[0] = %q, want octocat/repo-a", repos[0])
	}

	// Second FetchLists must not re-call the client (cache hit).
	_, _ = nav.FetchLists(context.Background(), "")
	_, _ = nav.FetchRepos(context.Background(), lists[1].URL)
	if gql.calls != 1 {
		t.Errorf("graphql client called %d times, want 1 (cache miss)", gql.calls)
	}
}

// TestGraphQLNavigator_Error propagates the GraphQL error to the
// caller and caches the failure (do not retry).
func TestGraphQLNavigator_Error(t *testing.T) {
	t.Parallel()
	gql := &fakeGQL{err: errors.New("502 bad gateway")}
	nav := starlists.NewGraphQLNavigator(gql, "octocat", 10, 50)

	_, err := nav.FetchLists(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error")
	}
	_, err2 := nav.FetchRepos(context.Background(), "/stars/octocat/lists/whatever")
	if err2 == nil {
		t.Fatalf("expected error on second call as well")
	}
	if gql.calls != 1 {
		t.Errorf("calls = %d, want 1 (error must be cached)", gql.calls)
	}
}

// TestGraphQLNavigator_NilUser is the empty-user path: the API
// returned no error but the user has no starlists. Both methods
// return empty / miss without erroring.
func TestGraphQLNavigator_NilUser(t *testing.T) {
	t.Parallel()
	gql := &fakeGQL{resp: &githubapi.UserListsResponse{}}
	nav := starlists.NewGraphQLNavigator(gql, "octocat", 10, 50)
	lists, err := nav.FetchLists(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchLists: %v", err)
	}
	if len(lists) != 0 {
		t.Errorf("len(lists) = %d, want 0", len(lists))
	}
}
