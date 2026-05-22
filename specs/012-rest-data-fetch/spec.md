# Feature Specification: REST Data-Fetch Wiring for 4 Plugins

**Feature Branch**: `012-rest-data-fetch`

**Created**: 2026-05-22

**Status**: Draft

**Input**: User description: 011 で 19 plugin の partial-parity は完了したが、`traffic` / `contributors` (user/org mode) / `repositories.Starred` / `repositories.Random` の 4 系統は Run でデータ取得が未配線のため Skipped を返している。本 feature でこれらに REST 経由のデータ取得を配線する。GraphQL 系 (sponsors / sponsorships / projects / repositories.Pinned 等) は別 PR (013)、chromedp 系 (topics) は別 PR (014) に分離。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Traffic plugin renders real per-repository views (Priority: P1)

A maintainer who has `repo` scope on their token enables `plugin_traffic: yes` in their workflow. The metrics SVG renders a Traffic section that shows the actual GitHub-reported views + uniques aggregated across every repository they own, with a sorted per-repo breakdown — instead of the current empty/skipped state.

**Why this priority**: Traffic is the single most-requested missing data source on the M4 baseline; it's the only plugin that requires `repo` scope and exposes data unavailable elsewhere. Wiring it first proves the REST-fetch pattern other plugins reuse.

**Independent Test**: Set `GITHUB_TOKEN` with `repo` scope, run the action with `plugin_traffic: yes` and `base: ""`, confirm the rendered SVG includes the upstream-parity Traffic section with a non-zero `N views (M unique)` aggregate plus at least one per-repo row.

**Acceptance Scenarios**:

1. **Given** a token with `repo` scope and 3 repositories each with traffic activity, **When** the action runs with `plugin_traffic: yes`, **Then** the SVG Traffic section shows `<total views (total uniques)>` and 3 per-repo rows sorted by view count descending.
2. **Given** a token without `repo` scope, **When** the action runs with `plugin_traffic: yes`, **Then** the Traffic section is suppressed (Result.Skipped=true) and a `missing repo scope` warning is logged once at the engine boundary.
3. **Given** one of the user's repositories returns 404 (e.g., recently transferred), **When** the traffic fetch runs, **Then** that single repo is skipped, a *RetryableError* lands on `pc.Data.Errors`, and the remaining repos still contribute to the aggregate.

---

### User Story 2 - Contributors plugin aggregates real per-repo contributors (Priority: P2)

A user enables `plugin_contributors: yes` in user-or-org rendering mode. The metrics SVG renders a Contributors section listing top contributors across their Featured repositories with login, avatar, and commit count — even though the metrics action is rendering a USER profile (not a single-repo profile).

**Why this priority**: Contributors had a partial wired for repo-mode (M7) but user/org-mode kept returning Skipped. This story closes a visible gap reviewers can verify by enabling the plugin alone.

**Independent Test**: Run the action against a user with 3+ Featured repositories, confirm the Contributors section shows ≥1 contributor row aggregated across all those repos (each row has avatar + login + total commits).

**Acceptance Scenarios**:

1. **Given** a user with 3 Featured repositories, **When** the action runs with `plugin_contributors: yes` in user mode, **Then** the Contributors section lists each distinct contributor login with their total commit count summed across the 3 repos.
2. **Given** one repository has 502/transient errors during the contributors fetch, **When** the engine continues, **Then** the other repos' contributors still appear and an error is recorded for the failed repo.
3. **Given** an empty result (no repositories or no contributors), **When** the partial renders, **Then** the section is suppressed (Result.Skipped or empty List → partial returns "").

---

### User Story 3 - Repositories plugin exposes Starred + Random lists (Priority: P3)

A user wants their metrics SVG to show repositories they have starred (recently) and a randomly-selected subset of their Featured repos, on top of the existing Featured list.

**Why this priority**: Lower-impact than Traffic/Contributors because the partial already renders Featured. Starred + Random are additive — the user enables them via `plugin_repositories_starred: yes` / `plugin_repositories_random: N`. Pure-REST + pure-Go work, no new scope needed.

**Independent Test**: Run with `plugin_repositories_starred: yes`, confirm Result.Starred has up to N entries from the user's starred list; run with `plugin_repositories_random: 3` and a fixed `plugin_repositories_random_seed: 42`, confirm Result.Random has exactly 3 entries chosen deterministically from Featured.

**Acceptance Scenarios**:

1. **Given** a user with starred repositories, **When** the action runs with `plugin_repositories_starred: yes` and `plugin_repositories_limit: 4`, **Then** Result.Starred holds 4 entries with the same `plugins.Repository` shape as Featured.
2. **Given** Featured has 8 repos and a `plugin_repositories_random: 3` + `plugin_repositories_random_seed: 42` config, **When** Run executes twice in a row, **Then** the same 3 repos are returned both times in the same order (determinism).
3. **Given** Featured is empty, **When** `plugin_repositories_random` is requested, **Then** Result.Random is an empty slice (no panic, no error).

---

### Edge Cases

- **Rate-limit response (429 or X-RateLimit-Remaining=0)**: the fetch records a *RetryableError* and surfaces the rate-limit reset timestamp; subsequent per-repo fetches in the same Run still execute (one slow repo doesn't take down the whole plugin).
- **Empty `Computed.RepositoryList`**: traffic/contributors return an empty (non-Skipped) Result with no rows; the partial then suppresses the section per its existing empty-state contract.
- **Timeout exhaustion**: the per-plugin overall timeout is honored — if it elapses mid-fetch, partial results already collected are surfaced and remaining work is cancelled.
- **Network failure during pagination of `/users/{login}/starred`**: the page that succeeded contributes its entries to Result.Starred; the failed page records an error and pagination stops (no infinite retry).
- **Decisive shuffle with seed=0 or unset**: Random uses `time.Now().UnixNano()` as a fallback seed so each render produces a fresh subset; with a non-zero seed, output is reproducible.
- **Same contributor across multiple repos**: their commit counts are SUMMED across repos (per upstream behavior), and a single deduped row is emitted.

## Requirements *(mandatory)*

### Functional Requirements

#### Traffic (US1)

- **FR-001**: System MUST fetch `/repos/{owner}/{repo}/traffic/views` for every entry in `Computed.RepositoryList` when `plugin_traffic` is truthy and the token carries `repo` scope.
- **FR-002**: System MUST aggregate the per-repo `count` + `uniques` totals into `Result.Total`, and surface the per-repo breakdown via `Result.Views[NameWithOwner]`.
- **FR-003**: System MUST return `Skipped=true` (no error logged) when the token is missing the `repo` scope, and MUST surface a single human-readable warning at the engine boundary so the user understands why traffic is missing.
- **FR-004**: System MUST limit concurrent per-repo HTTP requests to a configurable concurrency cap (default 4) and apply a per-request timeout (default 30s).

#### Contributors (US2)

- **FR-005**: When invoked in user-or-org mode, system MUST fetch `/repos/{owner}/{repo}/contributors` for every entry in `Computed.RepositoryList` (or just `Result.Featured` when populated) and aggregate contributor commits per `login` into `Result.List`.
- **FR-006**: System MUST sort the aggregated contributor list by total `commits` descending, then by `login` ascending for ties, and truncate to a configurable `plugin_contributors_limit` (default 14, matching upstream metadata).
- **FR-007**: System MUST preserve the existing repo-mode behavior unchanged — only the user-and-org modes (where Run currently returns Skipped) are wired.

#### Repositories.Starred + Random (US3)

- **FR-008**: System MUST fetch `/users/{login}/starred` paginated up to `plugin_repositories_limit` entries (default 8 per upstream) when `plugin_repositories_starred` is truthy, populating `Result.Starred` with `plugins.Repository` entries.
- **FR-009**: System MUST populate `Result.Random` from `Result.Featured` via a deterministic shuffle when `plugin_repositories_random` is truthy; the requested count comes from `plugin_repositories_random` (integer ≥1) and the shuffle is reproducible across runs via `plugin_repositories_random_seed`.
- **FR-010**: System MUST default the random seed to `time.Now().UnixNano()` (per-run fresh selection) when `plugin_repositories_random_seed` is unset or 0.

#### Shared (all USs)

- **FR-011**: Each per-repo / per-page HTTP failure MUST be recorded as a `*RetryableError` on `pc.Data.Errors`, never returned as a plugin-fatal error. Subsequent fetches in the same Run continue.
- **FR-012**: System MUST never call the network when the underlying input is empty (e.g., empty `RepositoryList`, missing `User.Login`) — the plugin returns an empty (non-Skipped) Result and the partial suppresses its section.
- **FR-013**: System MUST honor the existing per-plugin overall timeout (no new global timeout introduced), cancelling outstanding HTTP work and surfacing partial results on deadline expiry.

### Key Entities

- **TrafficViewSet**: per-repo time-series of (date, count, uniques) returned by GitHub. Only the totals are retained — the per-day breakdown is dropped because the upstream partial uses aggregates only.
- **ContributorAggregate**: per-login total of `commits + additions + deletions` summed across the user's repositories. Aggregation is independent of which repo a contributor appeared in (we just track totals per login).
- **StarredRepo**: a `plugins.Repository` entry corresponding to one entry in `/users/{login}/starred`. The plugin reuses the existing `Repository` shape so the partial renders Starred + Featured identically.
- **RandomPick**: a deterministic-shuffled subset of `Result.Featured`. The seed is the only stateful input.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Setting `plugin_traffic: yes` on an account with `repo` scope produces a Traffic section with a non-zero `N views (M unique)` aggregate within the existing per-plugin timeout (no degradation beyond the underlying API latency).
- **SC-002**: A user with 3+ Featured repositories enabling `plugin_contributors: yes` sees ≥1 contributor row with non-zero commits, **without** changing the rendered output of any other plugin.
- **SC-003**: Enabling `plugin_repositories_starred: yes` adds Starred entries identical in shape to Featured, verifiable by snapshotting the Result JSON (`data.plugins.repositories.starred` is a non-empty array with each entry having `nameWithOwner` + `stars` + `forks`).
- **SC-004**: Two consecutive runs with the same `plugin_repositories_random_seed` produce byte-identical `data.plugins.repositories.random` arrays (reproducibility).
- **SC-005**: A single transient HTTP failure on any one repo / page does NOT prevent the other repos / pages from rendering — the partial output still includes successful entries, and the count of `*RetryableError` entries on `Data.Errors` is bounded by the failure count.
- **SC-006**: No regression: every existing partial output (sponsors / topics / habits / activity / repositories.Featured / etc.) renders identically before and after this feature lands (golden diff = 0 outside the 4 touched Result types).

## Assumptions

- The existing `internal/githubapi` REST client (`pc.REST`) already handles authentication, rate-limit headers, base-URL injection, and retry on 5xx; this feature only wires plugin-specific endpoints into it.
- The token-scope check is available via `pc.REST.Scopes(ctx)` (mirrors the existing pattern in `sponsors.Run` / `projects.Run`).
- The `plugins.Repository` struct doesn't need new fields — `NameWithOwner / URL / Description / Visibility / IsFork / Stars / Forks / Watchers / Language / Languages` already cover what the REST `/users/{login}/starred` payload exposes.
- Concurrency = 4 matches the established pattern in `languages.indepth.Run` and remains under GitHub's per-user concurrent-request soft limit.
- Per-request HTTP timeout = 30s is conservative enough for slow-responding repositories without holding up the per-plugin overall timeout.
- `plugin_repositories_random_seed: 0` is treated as "no seed provided" (fall back to `time.Now().UnixNano()`); seed=0 is a documented sentinel value, not a literal seed.
- Random shuffle reuses the existing Featured slice — no separate network call.
- The 4 plugins' partials (already shipped in 011) need NO changes; they already render correctly when Result fields are populated.
