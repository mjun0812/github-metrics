# Contract: per-plugin repo-mode switching

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-005

This contract defines how the 7 M4 adopted plugins that overlap with
the upstream `repository` template's partial list gain a "repo-mode"
without breaking M4 user-mode behavior.

## 1. Affected plugins (7)

| Plugin slug    | Source package                       | repo-mode source for aggregation        |
|----------------|--------------------------------------|----------------------------------------|
| `activity`     | `internal/plugins/activity`          | `data.Repo.Activity` (RepoActivity)    |
| `contributors` | `internal/plugins/contributors`      | `data.Repo` + REST listContributors    |
| `languages`    | `internal/plugins/languages`         | `data.Repo.PrimaryLanguage`            |
| `people`       | `internal/plugins/people`            | `data.Repo` collaborators (REST)        |
| `projects`     | `internal/plugins/projects`          | `data.Repo` pinned items                |
| `sponsors`     | `internal/plugins/sponsors`          | `data.Repo.SponsorshipsAsMaintainer`   |
| `stargazers`   | `internal/plugins/stargazers`        | `data.Repo` time-series of stars        |

The other 14 adopted plugins (`achievements`, `calendar`, `habits`,
`isocalendar`, `notable`, `reactions`, `repositories`, `sponsorships`,
`starlists`, `stars`, `topics`, `traffic`, `languages.recent`,
`languages.indepth`) are **NOT** in `assets/templates/repository/partials/_.json`
— they are unaffected and continue to render only when classic
template is active.

## 2. Plugin Compute signature (unchanged)

The exported `Compute` signature on each plugin stays identical to the
M4 surface:

```go
type Plugin interface {
    Name() string
    Compute(ctx context.Context, pc *plugins.PluginContext) error
}
```

No new method, no new interface, no registry change. Constitution III
(採用 21 plugin invariant) holds — `TestCompliance_M4_AdoptedPlugins`
keeps passing.

## 3. Per-plugin internal refactor

Each affected plugin's existing `Compute` body becomes
`computeUserMode`; a new `computeRepoMode` is added; the exported
`Compute` dispatches:

```go
// internal/plugins/<name>/<name>.go

func (p *Plugin) Compute(ctx context.Context, pc *plugins.PluginContext) error {
    if pc.Data.Repo != nil {
        return p.computeRepoMode(ctx, pc)
    }
    return p.computeUserMode(ctx, pc)
}

// computeUserMode: existing M4 body, unchanged
func (p *Plugin) computeUserMode(...) error { /* ... */ }

// computeRepoMode: new — reads pc.Data.Repo
func (p *Plugin) computeRepoMode(...) error { /* ... */ }
```

The two helpers may share lower-level utilities (formatters, octicon
lookups, partial result struct types). The result type stored under
`data.Plugins["<name>"]` gains a `Mode string` field:

```go
type Result struct {
    Mode string `json:"mode"` // "user" | "repo"
    // ... existing fields
}
```

## 4. Partial unchanged

The per-plugin partial registered in
`internal/templates/classic/partials/plugins.go` continues to consume
the same result struct. Partials check `result.Mode` only if the
visual rendering needs to differ between modes (most won't; e.g.,
`languages` partial just renders whatever languages are in the
result). The `repository` template registers its own partial set
under `internal/templates/repository/partials/plugins.go` that mirrors
the classic template's plugin partials by name — both call the same
plugin Result struct.

## 5. Test plan (per plugin)

For each of the 7 plugins, the existing user-mode tests stay green
**unchanged** + a paired repo-mode test lands:

- `TestPlugin_UserMode_HappyPath` (existing M4 test) — asserts
  `result.Mode == "user"`
- `TestPlugin_RepoMode_HappyPath` (new) — set up a `*plugins.Data`
  with `Repo: &plugins.Repo{...}`, assert `result.Mode == "repo"`
  and the aggregation reflects the single-repo data
- `TestPlugin_RepoMode_NilRepoFallback` (new) — when `Data.Repo ==
  nil`, assert `Compute` takes the user-mode path (regression guard
  against accidental nil-deref in repo-mode)

Each plugin's tests are independent — they can land in 7 parallel
PRs if desired. The M7 PR bundles them into one branch.

## 6. Non-affected plugins (guard test)

A single new test in `internal/plugins/plugin_test.go` (or
`tests/compliance/`) iterates the 14 non-affected plugin slugs and
asserts that their `Compute` does NOT branch on `pc.Data.Repo`. The
simplest check: compute results with `Data.Repo == nil` and
`Data.Repo != nil` and assert byte-identical output for these
plugins. This locks the contract that only the 7 listed plugins
implement repo-mode switching.

## 7. Failure mode

If a plugin's `computeRepoMode` returns an error:

- Per the M2 plugin contract, the error is captured in
  `result.Errors[]` (when `Die == false`, the default)
- The partial for that plugin renders empty (nil-safe)
- Other plugins continue to run

Repo-mode failures do NOT escalate to fail-fast — only the missing
`repo` input or 404 on `base.Repository` cause fail-fast (per R-006
and contract `base-repository-query.md` §5).
