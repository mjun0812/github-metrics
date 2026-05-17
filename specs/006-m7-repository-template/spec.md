# Feature Specification: M7 — repository template

**Feature Branch**: `006-m7-repository-template`

**Created**: 2026-05-17

**Status**: Draft

**Input**: User description: "M7"

## Overview

M7 ships the second adopted template (`repository`) alongside the existing
`classic` template (M2). The repository template re-centers the rendered
SVG on a single GitHub repository (`<owner>/<repo>`) instead of a user
profile. It reuses the M3 chromedp render pipeline and the subset of M4
plugins that already operate on repository data — no new plugin slugs
land in this phase.

Source of truth: [`docs/design/16-tasks-mvp.md` Phase M7 T-089](../../docs/design/16-tasks-mvp.md#phase-m7-repository-テンプレート-1-タスク),
[`docs/design/07-templates.md` §6](../../docs/design/07-templates.md#6-repository-テンプレート),
[`docs/design/15-selection-answer.md` §3 Q3](../../docs/design/15-selection-answer.md#3-q3-テンプレート).
The M4 baseline already adopted the 7 plugins (`languages`, `projects`,
`stargazers`, `people`, `activity`, `contributors`, `sponsors`) that the
upstream `repository` template's `_.json` calls out.

## Clarifications

### Session 2026-05-17

- Q: `data.Repo` のデータ取得は新規 GraphQL クエリ `base.repository(login, repo)` を追加するか? → A: Option A — 新規クエリを追加 (upstream 互換、community/contributors 等の単一-repo 専用フィールドを取得)
- Q: 既存 7 plugin (languages / projects / stargazers / people / activity / contributors / sponsors) は repository template 下で集計対象を切替えるか? → A: Option A — plugin 側に「repo-mode 切替」を実装し、`data.Repo != nil` のとき集計対象を `data.Repo` ベースに切替 (upstream 互換、template が意味を持つ)
- Q: repo input の key 名 / CLI flag 名は? → A: Option B — top-level `repo` (env: `INPUT_REPO`、CLI: `--repo <name>`、action.yml の新 input)。upstream の `q.repo` 直系で、M6 の `plugin_*` namespace を汚染しない

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Render a repository's metrics SVG via Action mode (Priority: P1)

A repository maintainer wires `mjun0812/github-metrics@vX.Y.Z` into a
`metrics.yml` workflow that runs nightly. They set `template: repository`
and `repo: <name>` so the produced SVG showcases that repository's
identity (avatar, description), community health, and recent activity
instead of the maintainer's user-level totals. The workflow commits the
file to the repository so the README can embed it.

**Why this priority**: P1 because the entire M7 milestone exists to
deliver this output. Without the repository template the M4 plugin set
remains user-focused only — repository owners cannot showcase a single
repo via this project today.

**Independent Test**: With mocked GraphQL/REST deps, run the binary
in Action mode with `INPUT_TEMPLATE=repository INPUT_USER=octocat
INPUT_REPO=hello-world INPUT_DRYRUN=yes` and assert the produced
SVG starts with `<svg ...>`, contains the repo identity (`hello-world`,
owner avatar), and a community + activity panel. Total time under 30s.

**Acceptance Scenarios**:

1. **Given** `template: repository` + `repo: hello-world` + valid
   token, **When** the action runs, **Then** the rendered SVG includes
   the repository name, owner avatar, description, community health
   (contributors / stargazers / forks), and recent activity panes; no
   user-profile-only sections (e.g., achievements) appear.
2. **Given** the same inputs without `repo`, **When** the action
   runs, **Then** it fails fast with a clear error before any GraphQL
   call (mirrors upstream's "you must pass a `repo` argument" error).
3. **Given** `template: repository` is selected but the workflow's
   `output_action: commit` writes back to the same repo, **When** the
   action runs, **Then** the metrics_url output points at the committed
   SVG and the next run's data-changed check skips identical bytes (M6
   FR-013 invariant carries over).

---

### User Story 2 - CLI mode and multi-format support (Priority: P2)

A developer running locally wants to preview the repository template
before wiring it into a workflow. They invoke
`metrics-action --user octocat --repo hello-world --template repository
--output svg --dryrun --filename -` and pipe the result to `xmllint` for a
formatted view. They also want PNG / JPEG / JSON for downstream tooling
(README badges, dashboards, programmatic consumers).

**Why this priority**: P2 because the M6 CLI surface already supports
arbitrary template names; this story is "no regressions" plus the four
existing formats need to work for `repository` the same way they work
for `classic`.

**Independent Test**: With mocked deps, run the CLI four times — once
per format — and assert the byte output is non-empty, the SVG/JSON
validates against a basic shape check, and the PNG/JPEG header bytes
match the expected magic numbers. Each format under 30s.

**Acceptance Scenarios**:

1. **Given** the CLI invocation above with `--output svg`, **When** it
   runs, **Then** stdout receives a valid SVG that starts with `<svg`
   and ends with `</svg>`.
2. **Given** the same invocation with `--output json`, **When** it
   runs, **Then** stdout receives valid JSON whose top-level shape
   includes `repo` (owner, name, description, stargazers, forks,
   activity) alongside the existing user metadata.
3. **Given** the same invocation with `--output png` and an absolute
   `--filename`, **When** it runs, **Then** the file exists and begins
   with the 8-byte PNG signature (`89 50 4E 47 0D 0A 1A 0A`).

---

### User Story 3 - Template / account validation (Priority: P3)

A user mistakenly applies the repository template to a workflow whose
`account` resolution returns `user` or `organization` (no repo bound).
The system should detect the mismatch and fail fast with a clear
message rather than silently producing an SVG that looks like the
classic template with missing panes.

**Why this priority**: P3 because it's a guard rail, not a feature.
The system can still deliver value (US1, US2) without this — but the
upstream behavior (HTTP 406 in the web flow) maps to a CLI-friendly
error here, and shipping without it would invite confused bug reports.

**Independent Test**: Run the binary with `--template repository
--user octocat` (no `--repo`) and assert exit code 1 + stderr
contains "repository template requires repo input" + no GraphQL call
hit the mock backend.

**Acceptance Scenarios**:

1. **Given** `template: repository` without `repo`, **When**
   the action runs, **Then** it exits with code 1 before invoking
   `engine.Compute`, the stderr message names the missing input, and
   no API request reaches the GitHub mock.
2. **Given** `template: classic` with `repo: hello-world`,
   **When** the action runs, **Then** the repo input is ignored
   (logged at debug level) and the classic user-centric output is
   produced as in M2 — backward compatibility is preserved.

---

### Edge Cases

- **Repo not found**: `repo: nonexistent` against a valid user
  → GitHub API returns 404; system surfaces the error as a non-retryable
  failure with the M6 retry policy (no infinite loop).
- **Repo exists but archived**: the SVG renders normally; the activity
  pane shows the most recent activity (which may be old) without
  special-casing — same behavior as upstream.
- **Token lacks repo scope**: TokenValidator's scope warning already
  fires per M6 FR-014; the partial that needs the missing data renders
  empty (M6 plugin contract: nil-safe partials emit "").
- **Repo input contains `/`**: e.g. `repo: owner/name`. System
  treats the part after `/` as the repo name and warns once; the canonical
  form is `--user owner --repo name`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST register a template named `repository` in the
  templates registry such that `INPUT_TEMPLATE=repository` and
  `--template repository` both resolve it.
- **FR-002**: System MUST require a non-empty `repo` input
  (`INPUT_REPO` / `--repo <name>`) when the selected
  template is `repository`; missing repo MUST cause fail-fast exit 1
  before any GraphQL or REST call is issued.
- **FR-003**: System MUST render at least the four upstream-equivalent
  repository partials (`base.header` repository variant, `introduction`,
  `base.community`, `base.activity`) populated with repo-scoped data
  (owner avatar, repo name, description, contributors, stargazers,
  forks, commit / issue / PR activity).
- **FR-004**: System MUST support the same four output formats the
  classic template ships with (`svg`, `png`, `jpeg`, `json`) and route
  PNG / JPEG through the existing M3 chromedp render pipeline without
  introducing a new render code path.
- **FR-005**: System MUST reuse the 7 M4 plugins listed in upstream's
  `assets/templates/repository/partials/_.json` that overlap with the
  adopted 21 (`languages`, `projects`, `stargazers`, `people`,
  `activity`, `contributors`, `sponsors`) without re-adopting any
  M8-skipped plugin (no new slug enters `internal/plugins/`).
- **FR-005a**: Each of the 7 reused plugins MUST switch its aggregation
  target to `data.Repo` (single-repository) when that field is non-nil,
  and continue to aggregate over `data.User` otherwise. The switch MUST
  be a per-plugin internal mode flag — no new plugin slug and no new
  plugin package may be added. Mocked-data tests for each plugin MUST
  cover both modes (user-centric and repo-centric).
- **FR-006**: System MUST emit the partial dispatch order from
  `assets/templates/repository/partials/_.json` (intersected with the
  21 adopted plugins) so the rendered DOM ordering matches upstream
  semantic structure.
- **FR-007**: System MUST treat `template: classic` + `repo`
  inputs as backward-compatible: the repo value is logged at debug
  level and ignored; no behavior change to the existing classic
  output.
- **FR-008**: System MUST surface a non-retryable error (single attempt,
  no exponential backoff) when the repo is missing (404 from GitHub),
  matching the M6 RetryPolicy classification.
- **FR-009**: System MUST keep the M6 `output_action` pipeline (commit
  / pull-request / pull-request-merge / pull-request-squash /
  pull-request-rebase) functional for the repository template — the
  template choice MUST be orthogonal to the committer code path.
- **FR-010**: System MUST refuse to render the repository template when
  the user input is empty or the user does not have access to the
  specified repo (GitHub returns 404 / 403), surfacing the failure as
  the same error the M6 TokenValidator would surface for scope gaps.
- **FR-011**: System MUST add the `repository` slug to the compliance
  test's allowed template set so `TestCompliance_M6_NoNewPlugins` keeps
  passing while explicitly recognizing the new template directory.
- **FR-012**: System MUST add a new GraphQL query `base.repository(login, repo)`
  to `internal/githubapi/queries/` that returns the single-repository
  fields required by `data.Repo` (owner avatar, repo name, description,
  stargazers/forks count, primary language, license, default branch,
  community health, sponsorshipsAsMaintainer). The query MUST be invoked
  exactly once per run before any other repository-template partial
  reads `data.Repo`, and its response MUST populate `data.Repo` in full.

### Key Entities *(include if feature involves data)*

- **`templates.Template`** (existing): an entry in the templates
  registry. M7 adds one new instance with name=`repository`,
  supported_accounts=`[repository]`, formats=`[svg, png, jpeg, json]`,
  and a per-template partials list.
- **`engine.Request.Repo`** (new field): the repository name passed
  via `repo`. Required when `Template == "repository"`, ignored
  (with debug log) for other templates. Combined with `Login` to form
  the GitHub identifier `<Login>/<Repo>`.
- **`data.Repo`** (new top-level field on the plugin Data envelope):
  carries the resolved repository (owner, name, description,
  stargazers, forks, contributors, commit / issue / PR activity).
  Populated by a new single-repository GraphQL query
  `base.repository(login, repo)` (FR-012), executed once per run before
  any repository-template partial reads the field. All
  repository-template partials read from this field; user-template
  partials must continue to ignore it.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001** (US1): A user can run `metrics-action --user octocat
  --repo hello-world --template repository --output svg --dryrun
  --filename -` against mocked GraphQL/REST deps and receive a valid
  SVG on stdout within **30 seconds**. The SVG MUST contain the repo
  name, owner avatar, and at least one of (contributors, stargazers,
  activity) populated with mocked values.
- **SC-002** (US2): All four output formats (`svg`, `png`, `jpeg`,
  `json`) produce non-empty, format-valid output for the same input
  combination above within 30 seconds each. PNG/JPEG file headers MUST
  match their respective magic-number signatures.
- **SC-003** (US3): A request with `template: repository` but no
  `repo` MUST exit with code 1, emit a clear error message on
  stderr naming the missing input, and complete within **5 seconds**
  without contacting the GitHub API.
- **SC-004** (regression): All M2-M6 success criteria continue to
  pass on the same checkout (`make test` / `make test-chromedp` /
  `make test-heavy` all green) after the repository template lands.
  The classic template's golden snapshot does NOT drift.
- **SC-005** (compliance): The constitution 採用 21 plugin invariant
  (`TestCompliance_M4_AdoptedPlugins`) continues to pass — M7 adds a
  template, not a plugin. `internal/plugins/` directory count does not
  change.
- **SC-006** (compatibility): When the optional upstream
  `tests/fixtures/upstream/repository.json` fixture is present, the
  compatibility test (M2 SC-001 analogue) compares Go-output against
  it and tolerates only the documented dynamic fields (timestamps,
  version) — semantic content matches upstream.

## Assumptions

- The 7 plugins (`languages`, `projects`, `stargazers`, `people`,
  `activity`, `contributors`, `sponsors`) already adopted in M4 cover
  the repository template's needs; no new plugin adoption is required.
- The M3 chromedp render pipeline is template-agnostic (it consumes a
  fully-formed SVG) so PNG / JPEG support for the repository template
  is free of additional render code.
- The M6 `output_action` committer is template-agnostic (it writes a
  pre-rendered byte slice to a configured path / PR); no committer
  changes are needed.
- Upstream's `account != "repository"` HTTP-406 semantics (web flow)
  map to CLI-friendly fail-fast errors in this project, since M5 (web
  instance) is out-of-scope per `docs/design/15-selection-answer.md`.
- The upstream `repository.svg` example fixture
  (`https://github.com/lowlighter/metrics/blob/examples/metrics.repository.svg`)
  is the visual reference for partial layout but is NOT required to
  byte-match — the partial-by-partial DOM ordering match is enough.
- All plugin partials remain nil-safe per M2 contract: when invoked
  in a repository context where their data is empty, they emit "" and
  the template renders without them.
