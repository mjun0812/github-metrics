# Contract: `repo` input (action.yml + CLI flag + YAML config)

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-006, E-007

This contract defines the new top-level `repo` input M7 introduces.
Per spec clarify Q3 Option B (R-004), the input is **top-level**, not
namespaced under `plugin_*`.

## 1. action.yml entry

```yaml
inputs:
  # ... existing entries (user, template, token, committer_*, etc.) ...
  repo:
    description: |
      Repository name when template is 'repository'. Combined with `user`
      to form the GitHub identifier `<user>/<repo>`. Required when
      `template: repository`; ignored (with debug-level log) for all
      other templates.
    required: false
    default: ''
```

The entry is **additive**: no existing input is renamed, removed, or
re-typed. Constitution I (入力互換性 NON-NEGOTIABLE) is preserved.

## 2. CLI flag

| Form             | Behavior                                                  |
|------------------|-----------------------------------------------------------|
| `--repo <name>`  | Set repo to `<name>` (top-level CLIFlags.Repo)            |
| `--repo=<name>`  | Same (Go `flag` package alternative syntax)               |
| (omitted)        | repo defaults to `""` — validator only fails if template is `repository` |

The CLI flag does NOT accept the `<owner>/<name>` form. When a `/` is
present, the action emits a one-shot `slog.Warn("--repo: drop the
'owner/' prefix; the canonical form is --user <owner> --repo <name>'")`
and takes the suffix after the last `/` as the repo name (spec edge
case).

## 3. YAML config (`--config <path>.yaml`)

```yaml
# inputs.yaml — top-level `repo` alongside `user` / `template`
user: octocat
template: repository
repo: hello-world  # ← new in M7
output: svg
filename: github-metrics.svg
```

`LoadYAMLConfig` (M6 `internal/action/cli.go`) treats `repo` as a
flat top-level key — no flattening rules apply (those are reserved for
`plugins:`, `config:`, `committer:` blocks).

## 4. Env (Action mode)

`INPUT_REPO=<name>` — the GitHub Actions runner injects this when the
workflow YAML sets `with.repo: <name>`. The M6 `ParseInputs` already
walks `INPUT_<UPPER>` env vars, so no new code is needed to recognize
the variable beyond the input-list addition in E-007.

## 5. Priority (`ToInvocation`)

The merged input map honors the M6 precedence (highest first):

1. `--repo <name>` (CLI flag)
2. YAML config `repo: <name>`
3. preset YAML's `q.repo` (if `--preset` references one — unchanged)
4. `INPUT_REPO` env
5. metadata default (`""`)

## 6. Validation timing

Per R-006:

- `ParseFlags` does NOT enforce non-empty `--repo` (template-conditional)
- `runWith` / `runCLIWith` validate **after** input merge + **before**
  building engine deps:

  ```go
  if inv.Template == "repository" && stringInput(inv.Inputs, "repo", "") == "" {
      return xerrors.NewInputError("repo",
          errors.New("template 'repository' requires --repo / INPUT_REPO"))
  }
  ```

- The validator runs before `defaultBuildDeps` so SC-003 (no API call,
  5-second exit) is honored

## 7. Test plan

- `internal/action/cli_test.go`:
  - `TestParseFlags_RepoFlag` — `--repo hello-world` lands in CLIFlags.Repo
  - `TestParseFlags_RepoFlagWithSlash` — `--repo owner/name` → warn + name = `"name"`
  - `TestToInvocation_RepoFlagBeatsConfig`
  - `TestToInvocation_RepoFromYAML`
  - `TestToInvocation_RepoFromEnv`
- `internal/action/action_test.go`:
  - `TestRun_RepositoryTemplate_MissingRepoFailsFast` — exit 1, no API call (SC-003)
  - `TestRun_ClassicTemplate_WithRepoIgnored` — debug log, classic output unchanged (FR-007)
- `tests/integration/cli_test.go`:
  - extend the `TestCLI_ConfigYAML_Equivalence` matrix with a
    `--repo hello-world` ↔ `repo: hello-world` pair
- `internal/tools/gen-action-yml/main_test.go`:
  - assert `\n  repo:\n` is present in the generated action.yml
