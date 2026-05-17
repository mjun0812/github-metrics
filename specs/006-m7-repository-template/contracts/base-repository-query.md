# Contract: base.Repository GraphQL query

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-008

This contract defines the new GraphQL query M7 introduces to fetch
single-repository data for the `repository` template.

## 1. Operation

- **File**: `internal/githubapi/queries/repository.graphql`
- **Operation name**: `Repository`
- **Endpoint**: GitHub GraphQL v4 (`POST /graphql`)
- **Auth**: same `Authorization: bearer <PAT>` header chain as existing
  queries (no new auth code path)

```graphql
query Repository($login: String!, $repo: String!) {
  repository(owner: $login, name: $repo) {
    databaseId
    name
    nameWithOwner
    description
    stargazerCount
    forkCount
    isArchived
    primaryLanguage {
      name
      color
    }
    licenseInfo {
      name
      spdxId
    }
    defaultBranchRef {
      name
    }
    owner {
      login
      avatarUrl
    }
    issues(states: OPEN) {
      totalCount
    }
    pullRequests(states: OPEN) {
      totalCount
    }
  }
}
```

## 2. genqlient generation

- `genqlient.yaml` includes this file in its `operations` glob (no
  config change needed — the glob already covers
  `internal/githubapi/queries/*.graphql`)
- `make gen-graphql` produces `internal/githubapi/graphql_repository.go`
  containing:

  ```go
  func Repository(ctx context.Context, client graphql.Client,
      login string, repo string) (*RepositoryResponse, error)
  type RepositoryResponse struct { Repository RepositoryRepository }
  // ...nested response struct mirroring the GraphQL response shape
  ```

- The `lefthook.yml` `gen-graphql-drift` hook (or equivalent) re-runs
  genqlient on commit and fails if the generated file drifts from the
  current source

## 3. Invocation site

- Called from `internal/plugins/base/repository.go::FetchRepo(ctx, login, repo)`
- Invoked once per run when `engine.Request.Account == AccountRepository`
  AND `engine.Request.Repo != ""`
- Result populates `data.Repo` (E-003) before plugin dispatch
- Sequencing diagram:

  ```
  engine.Compute(req, deps)
    ├─ ParseInputs                          (M6)
    ├─ Account = AccountRepository (M7 new) ← inv.Inputs["repo"] != ""
    ├─ base.Compute(ctx, data)
    │   ├─ FetchUser (M1)                    ← always
    │   └─ FetchRepo (M7 new)                ← only when AccountRepository
    │       ├─ GraphQL Repository(login, repo)
    │       ├─ REST listContributors          ← contributors count
    │       └─ REST listCommits (per_page=100) ← recent commits
    ├─ core.RunPlugins(ctx, data)            ← repo-mode-aware per R-003
    └─ templates.MustGet("repository").Run    ← E-001
  ```

## 4. REST fallback fields

Three fields need REST round-trips because the GraphQL API does not
expose them directly:

| Field                   | REST endpoint                                       | Mapped to              |
|-------------------------|-----------------------------------------------------|------------------------|
| Contributors count      | `GET /repos/{owner}/{repo}/contributors`            | `Repo.Contributors`    |
| RecentCommits           | `GET /repos/{owner}/{repo}/commits?per_page=100&since=<30-days-ago>` | `Repo.Activity.RecentCommits` |

These calls reuse the existing `internal/githubapi/rest.go` client
(no new HTTP code). They run sequentially after the GraphQL query.

## 5. Failure modes

| Failure                          | Surfaced as                       | Retry behavior                 |
|----------------------------------|----------------------------------|-------------------------------|
| GraphQL response `errors[]` populated | `*plugins.PluginError` (base plugin) | None (M2 plugin contract) |
| GraphQL HTTP 5xx                | `*xerrors.RetryableError`         | M6 RetryPolicy (default 3 retries × 300ms) |
| REST 404 on contributors        | `Repo.Contributors = 0` + slog.Warn | None (best-effort; renders normally) |
| REST 5xx on commits             | `Repo.Activity.RecentCommits = 0` + slog.Warn | None (best-effort) |
| Repo not found (GraphQL `repository` returns null) | `*xerrors.InputError` on `repo` | None (fail-fast, exit 1) |

## 6. Test fixtures

- `tests/fixtures/graphql/repository_hello_world.json` — canned
  successful response for `Repository($login: "octocat", $repo:
  "hello-world")` with a deterministic-shape payload that the M7 unit
  + integration tests share
- `tests/fixtures/graphql/repository_not_found.json` — `data: {
  repository: null }` for the failure-mode test
- The existing httptest pattern from
  `tests/integration/cli_test.go::startGitHubMock` extends to mount
  this fixture under the `/graphql` endpoint by `operationName`

## 7. Test plan

- `internal/githubapi/graphql_repository_test.go`:
  - `TestRepositoryQuery_HappyPath` — fixture-backed roundtrip
  - `TestRepositoryQuery_NotFound` — repo null → typed input error
  - `TestRepositoryQuery_5xxRetryable` — 503 → RetryableError
- `internal/plugins/base/repository_test.go`:
  - `TestFetchRepo_PopulatesDataRepo` — happy path E2E
  - `TestFetchRepo_ContributorsRESTFailure_BestEffort` — REST 5xx →
    `Repo.Contributors == 0` + warn, GraphQL portion succeeds
