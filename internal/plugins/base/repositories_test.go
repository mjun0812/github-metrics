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
			"sponsorshipsAsMaintainer": {"totalCount": 4}
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
	pc.Inputs = map[string]any{"user": "octocat"}

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
	pc.Inputs = map[string]any{"user": "octocat"}

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
