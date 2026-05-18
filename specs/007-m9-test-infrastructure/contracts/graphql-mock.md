# Contract: GraphQLMux

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002

The shared GraphQL mock for tests. Implements `net/http.RoundTripper`
so it plugs into the project's `internal/githubapi.NewGraphQL(...)`
constructor via `httpx.Options.Transport`. The mock dispatches on the
operationName extracted from the request body's JSON payload.

## 1. Public API

```go
package mocks

// NewGraphQLMux constructs an empty mux. The returned value is
// goroutine-safe and registers a t.Cleanup that resets the handler
// map at end of test.
func NewGraphQLMux(t *testing.T) *GraphQLMux

// OnFile registers a fixture-file-backed 200 OK handler for the
// given operationName. fixturePath is relative to `tests/fixtures/`.
func (*GraphQLMux) OnFile(opName, fixturePath string)

// OnBody registers an inline-body handler.
func (*GraphQLMux) OnBody(opName string, status int, body string)

// OnFunc registers a per-call dynamic handler. `vars` is the
// decoded request `variables` map (cursor-aware paging mocks
// branch on this).
func (*GraphQLMux) OnFunc(opName string,
    fn func(vars map[string]any) (status int, body string))

// Calls returns the number of times RoundTrip dispatched to
// `opName`.
func (*GraphQLMux) Calls(opName string) int

// RoundTrip satisfies net/http.RoundTripper. The genqlient
// `graphql.Doer` interface is satisfied indirectly: `httpx.Client`
// wraps this RoundTripper into an `http.Client` (its `Do(req)` then
// satisfies `Doer.Do`).
func (*GraphQLMux) RoundTrip(req *http.Request) (*http.Response, error)
```

**Notes** — the genqlient `Doer` interface (v0.8.x) is:

```go
type Doer interface {
    Do(req *http.Request) (*http.Response, error)
}
```

`GraphQLMux` is wired through `httpx.Options.Transport`, so its
RoundTripper surface is wrapped by `httpx.Client` (an `*http.Client`)
to satisfy `Doer.Do`. There is no `MakeRequest` method on the mux —
genqlient v0.8.x does not require one.

## 2. Dispatch rules

| Condition | Behavior |
|-----------|----------|
| Request body's `operationName` matches a registered handler | invoke the handler (file read lazy, vars decoded for OnFunc) |
| `operationName` is empty or missing | `t.Fatalf("graphql mock: request missing operationName")` |
| `operationName` not registered | `t.Fatalf("graphql mock: no handler for %q (registered: %v)", opName, knownOps())` |
| Fixture file does not exist | `t.Fatalf("graphql mock: fixture not found: %s", absPath)` at first dispatch |

The `t.Fatalf` paths are intentional — missing handlers should
surface immediately so tests fail fast with an actionable message
rather than silently passing on empty responses.

## 3. Wiring with `internal/githubapi.NewGraphQL`

```go
mux := mocks.NewGraphQLMux(t)
mux.OnFile("User", "github/graphql/user_octocat.json")
mux.OnFile("UserRepositories", "github/graphql/user_repositories_250.json")

gql, err := githubapi.NewGraphQL(
    config.NewToken("MOCKED_TOKEN"),
    "http://mock.localhost/graphql",
    httpx.Options{Transport: mux, DisableRetries: true},
)
```

## 4. Variables-aware dispatch (paging)

Cursor-aware paging tests use `OnFunc`:

```go
mux.OnFunc("UserRepositories", func(vars map[string]any) (int, string) {
    cursor, _ := vars["after"].(string)
    if cursor == "" {
        return 200, page1Body
    }
    return 200, page2Body
})
```

## 5. Migration from existing ad-hoc mocks

The following per-package GraphQL mocks get removed or thinned after
migration:

- `internal/plugins/base/testhelper_test.go::graphQLMux` (most
  feature-complete existing implementation — the new package's API
  mirrors it)
- `internal/action/action_test.go::fakeGraphQL`
- `tests/integration/foundation_test.go::graphQLFixture`

M9 explicitly migrates `repository_test.go` (FR-012); the rest land
in follow-up PRs.

## 6. Test plan (for the helper itself)

- `internal/testutil/mocks/graphql_test.go`:
  - `TestGraphQLMux_UnknownOpName_TFatalf`
  - `TestGraphQLMux_MissingOperationName_TFatalf`
  - `TestGraphQLMux_OnFile_HappyPath`
  - `TestGraphQLMux_OnFile_MissingFile_TFatalf`
  - `TestGraphQLMux_OnBody_StatusAndBody`
  - `TestGraphQLMux_OnFunc_DecodesVariables`
  - `TestGraphQLMux_Calls_CountsDispatchesPerOpName`
  - `TestGraphQLMux_Concurrent_NoRace`
