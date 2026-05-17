# Data Model: M7 — repository template

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

This document enumerates the entities M7 introduces or extends.
M1-M6 entities (Invocation, CLIFlags, Committer, RetryPolicy, etc.)
are referenced but not re-described.

## E-001: `templates.Template` instance "repository"

- **Source package**: `internal/templates/repository`
- **Source file**: `repository.go`
- **Type**: implements `internal/templates.Template`
- **Public fields / methods**:
  - `Name() string` → returns `"repository"`
  - `Check(req engine.Request) error` → returns non-nil when
    `req.Account != AccountRepository` OR `req.Inputs["repo"] == ""`
  - `Run(ctx, req, data) (string, error)` → assembles partials per
    `assets/templates/repository/partials/_.json` ∩ registered partials
  - `Metadata() *config.TemplateMetadata` → loaded from
    `assets/templates/repository/metadata.yml`
  - `SupportedFormats() []string` → `["svg", "png", "jpeg", "json"]`
- **Registration**: `init()` registers via `templates.Register(t)`;
  consumed by `cmd/metrics-action/plugins.go` side-effect import
- **Validation**: account != repository → typed `*xerrors.InputError`
  with field `"account"`; missing repo → typed `*xerrors.InputError`
  with field `"repo"`. Both surfaced before plugin dispatch (R-006)

---

## E-002: `engine.Request` field extension

- **Source package**: `internal/engine`
- **Source file**: `engine.go`
- **New field** (additive — no existing field renamed):
  - `Repo string` — repository name. Populated from `inv.Inputs["repo"]`
    by `internal/action::newInvocation`. Combined with `Login` to form
    the GitHub identifier `<Login>/<Repo>` consumed by the
    `base.Repository` query
- **Validation rules**:
  - Required when `Template == "repository"` (enforced in `action.go`
    pre-dispatch, R-006)
  - Ignored (with `slog.Debug` log) when `Template != "repository"`
    (FR-007: backward compat with `classic` + stray repo input)
- **State transitions**: none (immutable per Request)

---

## E-003: `plugins.Repo` (new entity)

- **Source package**: `internal/plugins`
- **Source file**: `repo.go` (new)
- **Public fields** (mirrors GraphQL response shape, R-001):
  - `Owner string` — owner login (e.g., `"octocat"`)
  - `OwnerAvatar string` — owner avatar URL
  - `Name string` — repository name (e.g., `"hello-world"`)
  - `Description string` — repo description (may be empty)
  - `Stargazers int` — `stargazerCount`
  - `Forks int` — `forkCount`
  - `Contributors int` — REST `listContributors` length (committer.mjs:20)
  - `PrimaryLanguage string` — primary language name (may be empty)
  - `LicenseName string` — `licenseInfo.name` (may be empty)
  - `DefaultBranch string` — `defaultBranchRef.name`
  - `Activity RepoActivity` — recent commits / issues / PRs
    (sub-struct embedded for partial use)
  - `SponsorshipsAsMaintainer int` — copied from `data.User.SponsorshipsAsMaintainer`
    (matches `template.mjs:21` behavior)
- **Sub-struct `RepoActivity`** (declared in same file):
  - `RecentCommits int` — count of commits in last 30 days
  - `OpenIssues int` — open issue count
  - `OpenPullRequests int` — open PR count
- **Lifecycle**:
  - Populated by `base.Compute` when `data.Account == AccountRepository`
    AND `data.Repo == nil` (first call)
  - Read-only after population — no mutation by downstream plugins
  - nil when the `repository` template was not requested (i.e., classic
    template runs leave `data.Repo == nil`)

---

## E-004: `plugins.Data` field extension

- **Source package**: `internal/plugins`
- **Source file**: `data.go`
- **New field** (additive):
  - `Repo *Repo` — pointer to E-003 entity; nil when no repository
    template is active
- **Goroutine safety**: read via `data.RepoRef()` (new helper) — same
  RLock pattern as `data.GetPlugin`. Writes happen exactly once during
  `base.Compute` before plugin dispatch fans out, so the lock is needed
  for the read path only
- **Migration impact on existing plugins**: 14 non-repository plugins
  (achievements, calendar, habits, isocalendar, notable, reactions,
  repositories, sponsorships, starlists, stars, topics, traffic, and
  the language sub-modes) continue to read only `data.User` —
  unaffected. The 7 repo-mode plugins (R-003) gain a `if data.Repo !=
  nil` branch at the top of their `Compute`

---

## E-005: Per-plugin repo-mode flag (logical — no new exported type)

- **Source packages**: `internal/plugins/{activity,contributors,languages,people,projects,sponsors,stargazers}`
- **Source file**: each plugin's existing `*.go` + helper file
  `repo_mode.go` (new, per plugin)
- **Mechanism**: internal-only — a package-private function
  `computeRepoMode(ctx context.Context, pc *plugins.PluginContext) error`
  paired with the existing `computeUserMode` (refactored from the
  current `Compute` body). The exported `Compute` becomes:

  ```go
  func (p *Plugin) Compute(ctx context.Context, pc *plugins.PluginContext) error {
      if pc.Data.Repo != nil {
          return computeRepoMode(ctx, pc)
      }
      return computeUserMode(ctx, pc)
  }
  ```

- **No exported API change**: the `plugins.Plugin` interface stays
  identical; the registry signature stays identical; only the *inside*
  of 7 plugins gains a branch
- **Output shape per mode**: each plugin's `data.Plugins["<name>"]`
  result struct gains a `Mode string` field (`"user"` or `"repo"`)
  that downstream partials can branch on if needed. Existing
  user-mode tests assert `Mode == "user"`; new repo-mode tests assert
  `Mode == "repo"`

---

## E-006: `action.CLIFlags` field extension

- **Source package**: `internal/action`
- **Source file**: `cli.go`
- **New field** (additive):
  - `Repo string` — populated by `--repo <name>` flag
- **`ToInvocation` wiring**: emits `inputs["repo"] = cf.Repo` when
  `cf.Repo != ""` (priority above YAML config, env, and metadata
  default — same precedence as `cf.User` and `cf.Template`)
- **Validation**: deferred to the unified `runWith` / `runCLIWith`
  validator (R-006) — `ParseFlags` itself does not assert that
  `--repo` is set; that's a template-conditional requirement

---

## E-007: action.yml input entry (additive)

- **Source file**: `action.yml` (generated by `internal/tools/gen-action-yml`)
- **New entry** (alongside existing `user`, `template`, `token`, etc.):

  ```yaml
  repo:
    description: |
      Repository name when template is 'repository'. Combined with `user`
      to form the GitHub identifier `<user>/<repo>`. Ignored for other
      templates.
    required: false
    default: ''
  ```

- **Source-of-truth**: `internal/tools/gen-action-yml/main.go`'s
  top-level inputs list — currently emits `user`, `template`, `token`,
  `committer_*`, etc. M7 adds `repo`. The lefthook hook
  `action-yml-drift` then asserts the regenerated file matches the
  committed `action.yml`

---

## E-008: GraphQL query `Repository` (operation name)

- **Source file**: `internal/githubapi/queries/repository.graphql`
- **Operation**: `query Repository($login: String!, $repo: String!)`
- **Response fragment** (mirrors upstream template.mjs:13-21 + R-001):

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
      primaryLanguage { name color }
      licenseInfo { name spdxId }
      defaultBranchRef { name }
      owner { login avatarUrl }
      issues(states: OPEN) { totalCount }
      pullRequests(states: OPEN) { totalCount }
    }
  }
  ```

- **Codegen**: `genqlient` produces `internal/githubapi/graphql_repository.go`
  with `Repository(ctx, login, repo) (*RepositoryResponse, error)`
- **REST fallback**: `Contributors` count requires REST
  (`/repos/{owner}/{repo}/contributors` list length, per
  `template.mjs:20`) — handled in `base/repository.go` as a separate
  call after the GraphQL response lands
- **Recent commits**: REST `/repos/{owner}/{repo}/commits?per_page=100&since=...`
  for the activity sub-struct (RepoActivity.RecentCommits) — also
  handled in `base/repository.go`
