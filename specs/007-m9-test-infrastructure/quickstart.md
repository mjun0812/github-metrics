# Quickstart: M9 — test infrastructure consolidation

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md)

Concrete examples of using the M9 shared test infrastructure
(`internal/testutil/mocks` + `internal/testutil/golden`).

## 1. Plugin unit test — minimal setup

Wire a plugin test in under 30 lines using mocked GraphQL + REST:

```go
package myplugin_test

import (
    "context"
    "testing"

    "github.com/mjun0812/github-metrics/internal/plugins/myplugin"
    "github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestRun_HappyPath(t *testing.T) {
    t.Parallel()
    gql := mocks.NewGraphQLMux(t)
    gql.OnFile("User", "github/graphql/user_octocat.json")

    pc := mocks.NewPluginContext(t,
        mocks.WithGraphQL(gql),
        mocks.WithInputs(map[string]any{"user": "octocat"}),
    )

    got, err := myplugin.Plugin.Run(context.Background(), pc)
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    if got == nil {
        t.Fatal("Run returned nil result")
    }
    // ... assertions on got's typed fields
}
```

No per-test JSON literal strings, no per-test RoundTripper struct,
no manual `t.Cleanup`. The mux automatically cleans handler state at
end of test.

## 2. Variables-aware GraphQL paging mock

For cursor-aware paging tests:

```go
gql.OnFunc("UserRepositories", func(vars map[string]any) (int, string) {
    cursor, _ := vars["after"].(string)
    if cursor == "" {
        return 200, `{"data":{"user":{"repositories":{"nodes":[...],"pageInfo":{"endCursor":"cur1","hasNextPage":true}}}}}`
    }
    return 200, `{"data":{"user":{"repositories":{"nodes":[...],"pageInfo":{"endCursor":null,"hasNextPage":false}}}}}`
})
```

## 3. REST mock with Link header

For paginated REST endpoints (e.g., contributors count from the
Link header):

```go
rest := mocks.NewRESTMux(t)
rest.OnHeader(
    "/repos/octocat/hello-world/contributors",
    200,
    `[{"login":"octocat"}]`,
    http.Header{
        "Link": []string{
            `<https://api.github.com/repos/octocat/hello-world/contributors?per_page=1&page=42>; rel="last"`,
        },
    },
)
```

## 4. Asserting call counts

```go
gql.OnFile("User", "github/graphql/user_octocat.json")
gql.OnFile("UserRepositories", "github/graphql/user_repositories_250.json")

// ... run the code under test ...

if got := gql.Calls("User"); got != 1 {
    t.Errorf("User query called %d times, want 1", got)
}
if got := gql.Calls("UserRepositories"); got != 3 {
    t.Errorf("UserRepositories called %d times, want 3 (paging)", got)
}
```

## 5. Golden file workflow

### 5.1 Byte-exact comparison

```go
import "github.com/mjun0812/github-metrics/internal/testutil/golden"

func TestComputeJSON_OctocatGolden(t *testing.T) {
    t.Parallel()
    out := computeSomething()
    golden.Compare(t, out, "json/octocat.json")
    // Compares against tests/golden/json/octocat.json
}
```

### 5.2 SVG comparison (normalization-tolerant)

```go
golden.CompareSVG(t, svgBytes, "classic/octocat.svg")
// Both sides go through NormalizeSVG (attr-sort + whitespace collapse
// + dynamic-footer mask) before byte-diff.
```

### 5.3 JSON comparison (key-ordering tolerant)

```go
golden.CompareJSON(t, jsonBytes, "repository/octocat_hello-world.json")
// Both sides go through json.MarshalIndent(_, "", "  ") before diff.
```

### 5.4 Updating goldens

```sh
# Update all goldens (M2 convention preserved).
go test -update ./...

# Update one test's golden.
go test -update -run TestComputeJSON_OctocatGolden ./tests/integration/...
```

## 6. Drift failure example

When a golden test fails, the new output format makes the source of
drift immediately obvious:

```text
--- FAIL: TestRepositoryTemplate_HelloWorld_SVG_Golden (0.05s)
    repository_test.go:42: golden drift: tests/golden/repository/octocat_hello-world.svg
        first divergent byte at offset 1247 (got len=33451, want len=33101)
          got  [1207:1287] = ...<text>octocat\xe2\x80\x99s repository...
          want [1207:1287] = ...<text>octocat's repository...
        (run with -update to seed)
```

Single eyeball — you see exactly what changed.

## 7. Seeding a new fixture

```sh
# 1. Add the JSON file under tests/fixtures/github/{rest,graphql}/.
$EDITOR tests/fixtures/github/graphql/my_new_query.json

# 2. Reference it from the test.
mux.OnFile("MyNewQuery", "github/graphql/my_new_query.json")

# 3. Run the test — fixture loads on first dispatch.
go test ./internal/plugins/myplugin/...
```

The mock reads the file lazily on first matching dispatch. If the
file is missing, you get a clear `t.Fatalf` message with the
absolute path it tried.

## 8. Common pitfalls

- **Forgot `DisableRetries: true` in `httpx.Options`** — the inner
  retryablehttp layer adds retry semantics that conflict with the
  mock's once-per-call assertion model. Always pass
  `DisableRetries: true` when wiring the mock into
  `internal/githubapi.NewREST` or `NewGraphQL`.
- **Test reads `gql.Calls("OpName")` BEFORE the code under test
  ran** — no error, just `0`. Run the code under test first, then
  assert counts.
- **Two tests share one mux via package-level `var`** — the per-test
  `t.Cleanup` resets handlers, but a top-level mux outlives the
  cleanup. Always construct one mux per `t.Run` / per `func Test*`.

## 9. Migrating an existing test

The canonical M9 demonstration:
`internal/plugins/base/repository_test.go`. Before / after diff:

```diff
- type restRouter struct { ... 30 LOC ...  }
- const repositoryHelloWorldFixture = ` ... 18-line JSON literal ... `
- const repositoryOrgOwnerFixture = ` ... another 18-line JSON literal ... `
- // ... 60 more LOC of mkResp / readCloser helpers ...

+ import "github.com/mjun0812/github-metrics/internal/testutil/mocks"
+
+ mux := mocks.NewGraphQLMux(t)
+ mux.OnFile("Repository", "github/graphql/repository_hello_world.json")
+ rest := mocks.NewRESTMux(t)
+ rest.OnFile("/repos/octocat/hello-world/contributors",
+     "github/rest/contributors_with_link.json")
```

Result: ~150 LOC removed, fixtures moved to versioned JSON files,
zero behavior change.

## 10. Validation matrix

These quick commands verify M9 is operational:

| Step | Command | Expected outcome |
|------|---------|------------------|
| Mocks compile | `go build ./internal/testutil/...` | exit 0 |
| Mocks self-test | `go test ./internal/testutil/...` | all green |
| Golden re-seed round-trip | `go test -update ./tests/integration/...` then `go test ./tests/integration/...` | both green |
| Migration smoke | `go test ./internal/plugins/base/...` after FR-012 migration | green + LOC dropped per SC-001 |
| Lint additions | `golangci-lint run --timeout=10m ./...` | zero hits (after fixup in lint-config commit) |
| Full sweep | `make test && make test-chromedp && make test-heavy && make lint` | all green |
