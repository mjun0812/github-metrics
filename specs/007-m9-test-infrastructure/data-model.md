# Data Model: M9 — test infrastructure consolidation

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

This document enumerates the entities M9 introduces. All entities
live under `internal/testutil/` and are consumed only by `*_test.go`
files. No production-code entity changes.

## E-001: `mocks.RESTMux`

- **Source package**: `internal/testutil/mocks`
- **Source file**: `rest.go`
- **Type**: implements `net/http.RoundTripper`
- **Public methods**:
  - `NewRESTMux(t *testing.T) *RESTMux` — constructor with
    `t.Cleanup`-registered handler reset
  - `(*RESTMux).OnFile(path, fixtureRelativePath string)` —
    register a fixture-file-backed handler for the path
  - `(*RESTMux).OnBody(path string, status int, body string)` —
    inline-body handler
  - `(*RESTMux).OnHeader(path string, status int, body string, header http.Header)` —
    inline handler with custom response header
  - `(*RESTMux).OnFunc(path string, fn func(req *http.Request) (status int, body string, header http.Header))` —
    per-call dynamic handler
  - `(*RESTMux).Calls(path string) int` — count invocations
  - `(*RESTMux).RoundTrip(req *http.Request) (*http.Response, error)` —
    interface satisfaction
- **Lookup**: handler resolution is by `req.URL.Path` (query string
  stripped). Unknown path → 404 with `{"message":"Not Found"}` body.
- **Synchronization**: `sync.RWMutex` around the handler map; RoundTrip
  takes a read lock per call, On* takes a write lock per registration

## E-002: `mocks.GraphQLMux`

- **Source package**: `internal/testutil/mocks`
- **Source file**: `graphql.go`
- **Type**: implements `github.com/Khan/genqlient/graphql.Doer`
- **Public methods**:
  - `NewGraphQLMux(t *testing.T) *GraphQLMux` — constructor with
    `t.Cleanup`-registered handler reset
  - `(*GraphQLMux).OnFile(opName, fixtureRelativePath string)` —
    fixture-file-backed handler keyed by operationName
  - `(*GraphQLMux).OnBody(opName string, status int, body string)` —
    inline-body handler
  - `(*GraphQLMux).OnFunc(opName string, fn func(vars map[string]any) (status int, body string))` —
    variables-aware dynamic handler
  - `(*GraphQLMux).Calls(opName string) int` — count invocations
  - `(*GraphQLMux).MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error` —
    Doer interface satisfaction
- **Lookup**: parse the request body's `operationName`, look up the
  registered handler, return the canned response. Unknown
  operationName → `t.Fatalf("graphql mock: no handler for opName %q", opName)`
- **Variable handling**: `OnFunc` receives the decoded `variables`
  map so paging tests can branch on cursor values
- **Synchronization**: same RWMutex pattern as RESTMux

## E-003: Fixture file convention

- **Storage tree**: `tests/fixtures/github/{rest,graphql}/<name>.json`
- **Resolution**: relative paths from `On*File(...)` calls are
  resolved against `<repo-root>/tests/fixtures/` via the existing
  `mustRepoRoot` walker pattern
- **Naming convention**: lowercase + underscore-separated
  (`user_octocat.json`, `repository_hello_world.json`,
  `contributors_hello_world.json`)
- **File contents**: pure JSON; the mocks return the raw bytes as
  the HTTP response body
- **M9 seed set** (initial files):
  - `github/graphql/user_octocat.json` — existing inline `userOctocat`
    from `tests/integration/foundation_test.go`
  - `github/graphql/user_repositories_250.json` — existing inline
    `userRepositories250`
  - `github/graphql/repository_hello_world.json` — existing inline
    `repositoryHelloWorld` from M7
  - `github/graphql/repository_organization.json` — existing inline
    Organization-owner fixture from M7 base/repository_test
  - `github/graphql/repository_not_found.json` — `{"data":{"repository":null}}`
  - `github/rest/contributors_hello_world.json` — `[{"login":"octocat"}]`
  - `github/rest/contributors_with_link_header.json` — empty body
    (Link header lives in the test, not the file)
  - `github/rest/commits_3.json` — `[{"sha":"a"},{"sha":"b"},{"sha":"c"}]`
  - `github/rest/commits_empty.json` — `[]`
  - `github/rest/rate_limit_5000.json` — existing inline rate limit

## E-004: `golden.Compare* ` (Comparator family)

- **Source package**: `internal/testutil/golden`
- **Source file**: `golden.go`
- **Public functions**:
  - `Compare(t *testing.T, got []byte, goldenRelativePath string)` —
    byte-exact comparison; rewrites the golden when `-update` flag is
    set
  - `CompareSVG(t *testing.T, got []byte, goldenRelativePath string)` —
    applies `NormalizeSVG` (attribute-sort + whitespace collapse +
    dynamic-footer mask) to both sides before comparing
  - `CompareJSON(t *testing.T, got []byte, goldenRelativePath string)` —
    re-marshals both sides through `json.MarshalIndent(_, "", "  ")`
    so per-key whitespace drift between Go versions doesn't break
    the comparison
- **Path resolution**: `goldenRelativePath` is relative to
  `<repo-root>/tests/golden/` (mirrors fixture convention)
- **Update flag**: `golden.Compare*` reads `flag.Lookup("update")`
  per R-007. The flag is declared once in
  `tests/integration/output_json_test.go` (existing M2) and shared
  across all golden tests in the project
- **Failure message format** (R-003):
  ```text
  golden drift: <relative-path>
    first divergent byte at offset N (got len=N1, want len=N2)
      got  [N-40:N+40] = <40 bytes of got, \xNN-escaped>
      want [N-40:N+40] = <40 bytes of want, \xNN-escaped>
    (run with -update to seed)
  ```

## E-005: `mocks.PluginContext` (helper builder)

- **Source package**: `internal/testutil/mocks`
- **Source file**: `plugin_context.go`
- **Type**: factory function
- **Public function**:
  - `NewPluginContext(t *testing.T, opts ...PluginContextOption) *plugins.PluginContext`
- **Options**:
  - `WithGraphQL(*GraphQLMux)` — installs the mux as the
    GraphQL Doer
  - `WithREST(*RESTMux)` — installs the mux as the REST RoundTripper
  - `WithInputs(map[string]any)` — populates `pc.Inputs`
  - `WithSettings(*config.Settings)` — overrides default Settings
    (default: `&config.Settings{Repositories: 100}`)
  - `WithData(*plugins.Data)` — overrides the default empty Data
  - `WithLogger(*slog.Logger)` — overrides the default no-op logger
- **Defaults** (when options omitted):
  - Settings: `&config.Settings{Repositories: 100}`
  - Inputs: `map[string]any{"user": "octocat"}`
  - Data: `plugins.NewData()`
  - Logger: `slog.New(slog.NewTextHandler(io.Discard, nil))`
- **Cleanup**: per-mux `t.Cleanup` handles handler reset; no
  additional Cleanup hook needed
