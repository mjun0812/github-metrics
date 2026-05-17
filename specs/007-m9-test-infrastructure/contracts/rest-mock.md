# Contract: RESTMux

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-001

The shared REST mock for tests. Implements `net/http.RoundTripper`
so it plugs into the project's `internal/githubapi.NewREST(...)`
constructor via `httpx.Options.Transport`.

## 1. Public API

```go
package mocks

// NewRESTMux constructs an empty mux. The returned value is
// goroutine-safe and registers a t.Cleanup that resets the handler
// map at end of test.
func NewRESTMux(t *testing.T) *RESTMux

// OnFile registers a fixture-file-backed 200 OK handler for `path`.
// fixturePath is relative to `tests/fixtures/` (e.g. "github/rest/x.json").
// The file MUST exist; missing files trigger t.Fatalf at first
// matching dispatch.
func (*RESTMux) OnFile(path, fixturePath string)

// OnBody registers an inline-body handler.
func (*RESTMux) OnBody(path string, status int, body string)

// OnHeader is OnBody + explicit response header (e.g. Link header
// for paginated endpoints).
func (*RESTMux) OnHeader(path string, status int, body string, header http.Header)

// OnFunc registers a per-call dynamic handler.
func (*RESTMux) OnFunc(path string,
    fn func(req *http.Request) (status int, body string, header http.Header))

// Calls returns the number of times RoundTrip dispatched to `path`.
func (*RESTMux) Calls(path string) int

// RoundTrip satisfies http.RoundTripper.
func (*RESTMux) RoundTrip(req *http.Request) (*http.Response, error)
```

## 2. Dispatch rules

| Condition | Behavior |
|-----------|----------|
| `req.URL.Path` exact-matches a registered handler | invoke the handler (file read happens lazily on first match) |
| `req.URL.Path` does NOT match | return 404 with body `{"message":"Not Found"}` and `Content-Type: application/json` |
| Multiple handlers registered for same path | last-write-wins (most recent `On*` call) |
| Fixture file does not exist | `t.Fatalf("mocks.RESTMux: fixture not found: %s", absPath)` at first dispatch (NOT at registration time, so tests that register-but-don't-call don't fail) |

The query string is stripped from `req.URL.Path` before lookup. Tests
that need to assert query-string contents use `OnFunc` and inspect
`req.URL.RawQuery` directly.

## 3. Wiring with `internal/githubapi.NewREST`

```go
mux := mocks.NewRESTMux(t)
mux.OnFile("/rate_limit", "github/rest/rate_limit_5000.json")

rest, err := githubapi.NewREST(
    config.NewToken("MOCKED_TOKEN"),
    "http://mock.localhost",
    httpx.Options{Transport: mux, DisableRetries: true},
)
```

`DisableRetries: true` is REQUIRED — without it the httpx layer adds
retry semantics on top of the mock and obscures the test's intent.

## 4. Goroutine safety

Both reads (`RoundTrip`, `Calls`) and writes (`On*`) take a
`sync.RWMutex`. Multiple `t.Parallel` subtests sharing one mux do
not race. Per-test mux + per-test handler map is the more common
pattern (no shared mutation).

## 5. Migration from existing ad-hoc mocks

The following per-package implementations get removed or thinned to
a one-line re-export after migration:

- `internal/plugins/base/repository_test.go::restRouter`
- `internal/action/action_test.go::fakeREST`
- `internal/action/data_changed_test.go::contentsMock`
- `internal/action/failure_matrix_test.go::failureMatrixREST`
- `internal/action/committer_pr_test.go::prRESTMock`
- `tests/integration/cli_test.go::startGitHubMock` (httptest layer
  stays; REST handlers move to `mocks.NewRESTMux`)

M9 explicitly migrates `repository_test.go` (FR-012); the rest land
in follow-up PRs.

## 6. Test plan (for the helper itself)

- `internal/testutil/mocks/rest_test.go`:
  - `TestRESTMux_UnknownPath_Returns404`
  - `TestRESTMux_OnFile_HappyPath`
  - `TestRESTMux_OnFile_MissingFile_TFatalf`
  - `TestRESTMux_OnBody_StatusAndBody`
  - `TestRESTMux_OnHeader_LinkHeader`
  - `TestRESTMux_OnFunc_PerCallDynamic`
  - `TestRESTMux_Calls_CountsDispatchesPerPath`
  - `TestRESTMux_Concurrent_NoRace` (uses `t.Run` + `t.Parallel`
    subtests sharing one mux)
