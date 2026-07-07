package starlists

import (
	"context"
	"fmt"
	"sync"

	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// graphqlClient is the minimal interface that graphqlNavigator needs.
// *githubapi.GraphQL satisfies it; tests can supply a stub.
type graphqlClient interface {
	UserLists(ctx context.Context, login string, listsFirst, itemsFirst int) (*githubapi.UserListsResponse, error)
}

// graphqlNavigator is the production Navigator. It pulls the whole
// starlists payload (lists + their items) in a single GraphQL round
// trip via `user.lists`, then caches the result so the plugin's
// FetchLists / FetchRepos pair returns immediately on the second call.
//
// GitHub exposes `user.lists` natively (confirmed May 2026 PoC); we
// no longer need a headless browser to scrape the lists landing page
// or each detail page.
type graphqlNavigator struct {
	client     graphqlClient
	login      string
	listsFirst int
	itemsFirst int

	once  sync.Once
	mu    sync.Mutex
	lists []Starlist
	byURL map[string][]string
	err   error
}

// NewGraphQLNavigator returns a Navigator backed by the project's
// GraphQL client. login must match the user the plugin is rendering
// for. listsFirst caps the number of lists fetched; itemsFirst caps
// the items per list (GitHub's GraphQL `first` limit is 100).
func NewGraphQLNavigator(client graphqlClient, login string, listsFirst, itemsFirst int) Navigator {
	if listsFirst <= 0 {
		listsFirst = 10
	}
	if itemsFirst <= 0 {
		itemsFirst = 50
	}
	return &graphqlNavigator{
		client:     client,
		login:      login,
		listsFirst: listsFirst,
		itemsFirst: itemsFirst,
		byURL:      map[string][]string{},
	}
}

// load fetches once and populates the cache. Subsequent calls return
// the cached error (if any) or no-op.
func (n *graphqlNavigator) load(ctx context.Context) error {
	n.once.Do(func() {
		resp, err := n.client.UserLists(ctx, n.login, n.listsFirst, n.itemsFirst)
		if err != nil {
			n.err = fmt.Errorf("starlists: graphql user.lists: %w", err)
			return
		}
		if resp == nil || resp.User == nil || resp.User.Lists == nil {
			// User has no lists (or the field is null) — leave the
			// cache empty so FetchLists returns an empty slice.
			return
		}
		out := make([]Starlist, 0, len(resp.User.Lists.Nodes))
		for _, node := range resp.User.Lists.Nodes {
			if node == nil {
				continue
			}
			// Use the list name as a stable cache key. GitHub starlist
			// names are user-defined but unique within a user's lists.
			url := starlistKey(n.login, node.Name)
			desc := ""
			if node.Description != nil {
				desc = *node.Description
			}
			count := 0
			repos := []string{}
			var repositories []Repository
			if node.Items != nil {
				count = node.Items.TotalCount
				for _, item := range node.Items.Nodes {
					if item == nil {
						continue
					}
					// Only Repository fragments contribute to the repo
					// list — other concrete types (none today) are
					// silently dropped.
					if repo, ok := item.(*githubapi.UserListsUserListsUserListConnectionNodesUserListItemsUserListItemsConnectionNodesRepository); ok && repo != nil {
						if repo.NameWithOwner != "" {
							repos = append(repos, repo.NameWithOwner)
						}
						repoDesc := ""
						if repo.Description != nil {
							repoDesc = *repo.Description
						}
						repositories = append(repositories, Repository{
							Name:        repo.NameWithOwner,
							Description: repoDesc,
							IsPrivate:   repo.IsPrivate,
						})
					}
				}
			}
			n.byURL[url] = repos
			out = append(out, Starlist{
				Name:         node.Name,
				Description:  desc,
				Count:        count,
				Repositories: repositories,
				URL:          url,
			})
		}
		n.lists = out
	})
	return n.err
}

// starlistKey returns a stable URL-shaped key for a starlist. The
// shape matches the legacy chromedp navigator's output
// (`/stars/<login>/lists/<slug>`) so consumers — and tests — can keep
// using the URL field as a cache key without changes.
func starlistKey(login, name string) string {
	return fmt.Sprintf("/stars/%s/lists/%s", login, name)
}

// FetchLists implements Navigator. The url argument is ignored — the
// navigator already knows the login it was constructed for.
func (n *graphqlNavigator) FetchLists(ctx context.Context, _ string) ([]Starlist, error) {
	if err := n.load(ctx); err != nil {
		return nil, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]Starlist, len(n.lists))
	copy(out, n.lists)
	return out, nil
}

// FetchRepos implements Navigator. It looks up the cached repo list
// for the requested URL — populated by the same GraphQL call as
// FetchLists.
func (n *graphqlNavigator) FetchRepos(ctx context.Context, listURL string) ([]string, error) {
	if err := n.load(ctx); err != nil {
		return nil, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	repos, ok := n.byURL[listURL]
	if !ok {
		// The plugin always feeds URLs back that came from FetchLists,
		// so a miss here is a programming error — surface it.
		return nil, fmt.Errorf("starlists: no cached repos for %q", listURL)
	}
	out := make([]string, len(repos))
	copy(out, repos)
	return out, nil
}

// Ensure graphqlNavigator implements Navigator at compile time.
var _ Navigator = (*graphqlNavigator)(nil)
