# Contract: repository template (SVG / JSON output)

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-001

This contract defines the rendered output shape for the `repository`
template across the 4 supported formats. It is the M7 analogue to M2's
classic-template contract.

## 1. Partial dispatch order (SVG)

The repository template emits partials in the order declared by
`assets/templates/repository/partials/_.json` (upstream-mirrored).
Per FR-005 / FR-006 the rendered set is the **intersection** of:

- the 17 partial names in upstream's `_.json`, **and**
- partials registered in `internal/templates/partials.Registry` at run
  time (i.e., base.* partials + the 7 M4 plugins that overlap).

| # | Partial name        | Source                                | Adopted? | Notes |
|---|---------------------|---------------------------------------|----------|-------|
| 1 | `base.header`       | repository-template-specific          | ✓ M7      | Repo identity (owner avatar, repo name, description) |
| 2 | `introduction`      | repository-template-specific          | ✓ M7      | Repo description / about (reused name from classic; different impl) |
| 3 | `followup`          | upstream-only                         | ✗         | Skipped (M8) — registry miss → "" |
| 4 | `languages`         | M4 adopted plugin (repo-mode branch)  | ✓         | Aggregates `data.Repo.PrimaryLanguage` |
| 5 | `projects`          | M4 adopted plugin (repo-mode branch)  | ✓         | Pinned issues / PRs on this repo |
| 6 | `pagespeed`         | upstream-only                         | ✗         | Skipped (M8) |
| 7 | `stargazers`        | M4 adopted plugin (repo-mode branch)  | ✓         | Stargazer trend for this repo |
| 8 | `people`            | M4 adopted plugin (repo-mode branch)  | ✓         | Contributor avatars |
| 9 | `activity`          | M4 adopted plugin (repo-mode branch)  | ✓         | Recent commits / issues / PRs |
| 10 | `posts`            | upstream-only                         | ✗         | Skipped (M8) |
| 11 | `rss`              | upstream-only                         | ✗         | Skipped (M8) |
| 12 | `screenshot`       | upstream-only                         | ✗         | Skipped (M8) |
| 13 | `stock`            | upstream-only                         | ✗         | Skipped (M8) |
| 14 | `crypto`           | upstream-only                         | ✗         | Skipped (M8) |
| 15 | `contributors`     | M4 adopted plugin (repo-mode branch)  | ✓         | Contributor list with avatars + commit counts |
| 16 | `sponsors`         | M4 adopted plugin (repo-mode branch)  | ✓         | Sponsors of this repo's maintainer |
| 17 | `licenses`         | upstream-only                         | ✗         | Skipped (M8) — `data.Repo.LicenseName` still surfaces in JSON output |

**Rendered set (M7)**: 7 plugin partials + 2 base.* partials = 9.
Future M8 adoption may grow this set.

## 2. SVG root element

The SVG opening tag matches the classic template's shape one-for-one
(no template-conditional attributes in M7):

```xml
<svg xmlns="http://www.w3.org/2000/svg"
     width="480"
     height="99999"
     class="">
```

- `height="99999"` is the placeholder the M3 chromedp `Resize` step
  shrinks to the rendered content height.
- The `class=""` slot is reserved for the user-template-only
  `large` / `columns` modifiers; M7 always emits an empty value.
- A repository-template marker lands on the first inner section
  (`<section data-section="header" data-template="repository">`),
  not on the root `<svg>` element. Downstream consumers that need to
  distinguish templates SHOULD switch on `data.meta.template` (JSON
  output) or that section attribute (SVG output).

Differences from classic:

- The repository-template root is otherwise indistinguishable from
  the classic template root — both reuse the same shared CSS
  (`assets/templates/classic/style.css`) and chromedp Resize pipeline.
  Per-template content lives in the inner partial DOM, not the root
  envelope.

Note: future work could add `aria-label="GitHub metrics for
<owner>/<name>"`, `role="img"`, or `viewBox` to improve accessibility
and downstream tooling integration. Those attributes are NOT shipped
in M7 — see the open issue tracker if you need them.

## 3. JSON output shape

When `Format == "json"` the response carries the existing M2 shape
plus a new top-level `repo` object:

```json
{
  "data": {
    "user": { ... },              // existing M2 shape (owner of the repo)
    "repo": {                     // NEW in M7
      "owner": "octocat",
      "owner_avatar": "https://avatars.githubusercontent.com/u/...",
      "name": "hello-world",
      "name_with_owner": "octocat/hello-world",
      "description": "My first repository",
      "stargazers": 42,
      "forks": 7,
      "is_archived": false,
      "primary_language": { "name": "Go", "color": "#00ADD8" },
      "license_name": "MIT",
      "default_branch": "main",
      "contributors": 12,
      "sponsorships_as_maintainer": 3,
      "activity": {
        "recent_commits": 18,
        "open_issues": 5,
        "open_pull_requests": 2
      }
    },
    "plugins": {
      "languages": { "mode": "repo", "...": "..." },
      "activity":  { "mode": "repo", "...": "..." },
      "...": "..."
    },
    "computed": { ... },
    "config":   { ... },
    "meta":     { "version": "...", "generated": "...", "template": "repository" }
  },
  "errors":   [...],
  "warnings": [...]
}
```

**Backward compat note**: classic-template JSON does NOT change. The
`data.repo` key is only present when the repository template was the
selected template; classic-template JSON output continues to omit it.

## 4. Format support

| Format | Pipeline                                        | MIME              |
|--------|------------------------------------------------|-------------------|
| svg    | partials → SVG bytes → M3 chromedp Resize       | `image/svg+xml`   |
| png    | svg → M3 chromedp `page.CaptureScreenshot` PNG  | `image/png`       |
| jpeg   | svg → M3 chromedp `page.CaptureScreenshot` JPEG | `image/jpeg`      |
| json   | `engine.Marshal(data)` direct                   | `application/json` |

No new render code path. The M3 chromedp render is consumed
unchanged — it accepts any SVG byte slice.

## 5. Error envelope

The standard plugin error envelope from M2 applies. When the new
`base.Repository` query fails:

- **404 (repo not found)**: surfaced as a typed `*xerrors.InputError`
  on `req.Inputs["repo"]` — single attempt, no retry (per M6 retry
  policy), exit 1 within ~3 seconds
- **403 (access denied)**: same as 404 path; surfaced as an input
  error with a hint about token scopes
- **5xx (GitHub outage)**: wrapped as `*xerrors.RetryableError`, the
  M6 RetryPolicy retries up to its configured count
- **GraphQL `errors[]` array non-empty**: treated as a plugin-level
  non-fatal error per the M2 plugin contract — surfaced in
  `Result.Errors`, the partial that needed `data.Repo` emits ""

## 6. Test plan

- `internal/templates/repository/repository_test.go`:
  - `TestRun_PartialOrder_MatchesUnderscoreJson` — assert the rendered
    DOM partial-order matches the intersection of `_.json` and
    `partials.Registry`
  - `TestCheck_RejectsNonRepositoryAccount` — assert 406-equivalent
    error when `Account != AccountRepository`
  - `TestCheck_RejectsEmptyRepoInput` — assert error when
    `inputs["repo"] == ""`
  - `TestCheck_AcceptsValidInput`
  - `TestRun_Format_{SVG,JSON,PNG,JPEG}_HappyPath` — 4 cases, mocked deps
- `internal/templates/repository/partials/partials_test.go`:
  - `TestBaseHeader_RepoVariant` — outputs owner avatar + repo name
  - `TestBaseHeader_NilRepoSafe` — empty when `data.Repo == nil`
  - `TestIntroduction_DescriptionRendered`
  - `TestBaseCommunity_ContributorsAndStargazers`
  - `TestBaseActivity_RecentCommits`
- `tests/integration/output_test.go`:
  - `TestRepositoryTemplate_OctocatHelloWorld_SVG_Golden` — diff vs
    `tests/golden/repository/octocat_hello-world.svg`
  - `TestRepositoryTemplate_OctocatHelloWorld_JSON_Golden` — diff vs
    `tests/golden/repository/octocat_hello-world.json`
