# Phase 1 Data Model: M6 — GitHub Action / CLI

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は spec の Key Entities を Go 構造体レベルまで具体化し、`internal/action/*.go` のファイル単位と対応させる。各 Entity は **Source package / Public fields / Validation / Owned by** の四項目で記述する。

---

## E-001: `Invocation` (per-run context)

- **Source package**: `internal/action`
- **Source file**: `action.go`
- **Public fields**:
  - `Mode RunMode` — `action` / `cli` のいずれか (`GITHUB_ACTIONS` env で分岐)
  - `Inputs map[string]any` — INPUTS JSON / INPUT_<UPPER> / CLI flag / preset を merge した最終形 (key normalized: lower case + underscore)
  - `Token config.Token` — masked-on-stringify type (M1 で確立)
  - `Settings *config.Settings` — M1 既存
  - `Metadata *config.MetadataLoader` — M1 既存
  - `GitHubEvent *github.Event` — `GITHUB_EVENT_PATH` を parse した最小構造 (commit messages のみ)
  - `OutputFilename string` — `_filename` (wildcard `*` 解決後)
  - `OutputFormat string` — `svg` / `png` / `jpeg` / `json` (`config.output` から決定)
  - `OutputAction string` — validated; `none` / `commit` / `pull-request[-merge|-squash|-rebase]` のみ
  - `Dryrun bool` — `dryrun=yes` 時 true
- **Validation**:
  - `Mode == cli` で `OutputAction == commit | pull-request*` のとき `GITHUB_REPOSITORY` env 必須 (else exit 1)
  - `OutputAction` がサポート値外 (`gist` / `markdown-*` / 未知文字列) のとき fail-fast (FR-015b)
  - `Token` が `github_pat_*` 始まりのとき fail-fast (FR-004)
  - `OutputFilename` の `*` は 1 つだけ許容、複数あれば error
- **Owned by**: `internal/action/action.go::Run` / `RunCLI`

### `RunMode` enum

```go
type RunMode int

const (
    RunModeAction RunMode = iota  // GITHUB_ACTIONS=true
    RunModeCLI                     // direct binary invocation
)
```

---

## E-002: `Committer` (output_action dispatch)

- **Source package**: `internal/action`
- **Source file**: `committer.go`
- **Public fields**:
  - `RepoOwner string`, `RepoName string` — `GITHUB_REPOSITORY` を split
  - `Branch string` — `committer_branch` 入力。空なら repo default
  - `Filename string` — Invocation.OutputFilename
  - `Message string` — `committer_message` 入力。template substitution (`${user}`, `${template}`, `${date}`) 後の文字列
  - `AuthorName, AuthorEmail string` — `committer_author` / `committer_email`
  - `Sign bool` — `committer_gpg_sign`
  - `Action string` — Invocation.OutputAction
  - `MergeMethod string` — `merge` / `squash` / `rebase` (action が `pull-request-*` 系のとき設定)
  - `Body []byte` — rendered SVG / PNG / JSON / etc.
  - `Commit bool` — `output_condition=data-changed` で false にされる
- **Validation**:
  - `Action == commit | pull-request*` のとき RepoOwner / RepoName / Branch / Filename / Body 必須
  - `Action == none` のとき 何もしない (no-op, exit 0)
- **State transitions**:

  ```text
  Validate inputs
   ├── Action == "none"      → return nil
   ├── Action == "commit"    → ensureBranch → putContents
   └── Action == "pull-request*" → ensureRunBranch → putContents → createPR → (maybe merge)
  ```

- **Owned by**: `internal/action/committer.go::Run`

---

## E-003: `HashComparator` (data-changed)

- **Source package**: `internal/action`
- **Source file**: `data_changed.go`
- **Public fields**:
  - `REST *githubapi.REST` — M1 既存
  - `RepoOwner, RepoName, Branch, Filename string`
  - `NewBody []byte` — freshly rendered SVG bytes
- **Methods**:
  - `Equal(ctx) (bool, error)` — true なら commit skip (Committer.Commit = false)
- **Validation**:
  - 既存ファイル取得が 404 → `(false, nil)` (= "変わった" 扱い、通常 commit)
  - 取得が 200 以外 5xx → `xerrors.RetryableError` で wrap (FR-007 で retry)
  - レスポンス `encoding != "base64"` → error
- **Owned by**: `internal/action/data_changed.go::HashComparator`

---

## E-004: `CLIFlags` (CLI mode input)

- **Source package**: `internal/action`
- **Source file**: `cli.go`
- **Public fields**:
  - `Config string` — `--config <path>.yaml` の YAML ファイルパス
  - `User string` — `--user`
  - `Template string` — `--template`
  - `Token string` — `--token` (env override は TokenEnv 経由)
  - `TokenEnv string` — `--token-env <ENV_NAME>`
  - `Plugins map[string]string` — `--plugin key=value` の repeatable 集約
  - `Output string` — `--output` (svg/png/jpeg/json)
  - `Filename string` — `--filename` (`-` で stdout、それ以外はファイルパス)
  - `Dryrun bool` — `--dryrun`
- **Validation**:
  - `Token != "" && TokenEnv != ""` → exclusive、後者優先 + warning ログ
  - `Filename == "-"` && `Output != svg` → png/jpeg/json を stdout に流すのは binary corruption リスクのため allow (ユーザ責任、warning ログのみ)
- **Transformation**: `CLIFlags.ToInvocation() *Invocation` で `Invocation.Inputs` の形式 (= INPUT_<KEY> 相当) に変換し、action.yml と同じパイプラインに乗せる
- **Owned by**: `internal/action/cli.go::ParseFlags`

---

## E-005: `PresetBundle` (presets 統合)

- **Source package**: `internal/action`
- **Source file**: `inputs.go`
- **Public fields**:
  - `Path string` — `config_presets` 入力で指定された YAML パス (single)
  - `Q map[string]any` — preset の `q:` map (input overrides)
- **Methods**:
  - `Load(path string) (*PresetBundle, error)` — YAML 読み込み (`gopkg.in/yaml.v3`)
  - `MergeInto(inputs map[string]any)` — preset の値で inputs を上書き (CLI flag / INPUT_<KEY> > preset > metadata default の優先順位)
- **Validation**:
  - Path が存在しない → error
  - YAML の root が `q:` を含まない → error
- **Owned by**: `internal/action/inputs.go::LoadPresets`

---

## E-006: `RetryPolicy` (retry classification)

- **Source package**: `internal/action`
- **Source file**: `retry.go`
- **Public fields**:
  - `Retries int` — `retries` 入力 (default 3)
  - `Delay time.Duration` — `retries_delay` 入力 (default 300ms)
- **Methods**:
  - `Do(ctx, fn func() error) error` — fn を最大 (Retries+1) 回呼ぶ
    - error が `*xerrors.RetryableError` (errors.As でチェック) なら Delay 後にリトライ
    - それ以外の error は即 return (fail-fast)
    - 全 retry 消費後の最後の error を return
- **Validation**:
  - Retries < 0 → 0 に clamp
  - Delay < 0 → 0 に clamp
- **Owned by**: `internal/action/retry.go::Do`

---

## E-007: `TokenValidator`

- **Source package**: `internal/action`
- **Source file**: `token.go`
- **Public fields**:
  - `Token config.Token` — masked-on-stringify
  - `REST *githubapi.REST`
  - `RequiredScopes []string` — metadata に基づく per-plugin の必要 scope union
  - `QuotaRest, QuotaGraphQL, QuotaSearch int` — metadata の `quota_required_*` の plugin union
- **Methods**:
  - `Validate(ctx) (ValidationResult, error)`
- **Returns**:

  ```go
  type ValidationResult struct {
      RejectReason string   // "github_pat_* tokens not supported" 等。空なら pass
      MissingScopes []string // 警告ログ用、fatal ではない
      QuotaSufficient bool  // false なら exit 0 skipped
  }
  ```

- **Validation paths**:
  1. Token が `github_pat_` 始まり → RejectReason set, RAISE
  2. `HEAD /` で `X-OAuth-Scopes` 取得 → RequiredScopes と diff → MissingScopes (warning のみ、continue)
  3. `GET /rate_limit` で REST/GraphQL/Search の remaining と QuotaRest/GraphQL/Search を比較 → 不足なら QuotaSufficient=false (action は exit 0 skipped で抜ける)
- **Owned by**: `internal/action/token.go::Validator.Validate`

---

## E-008: `OutputActionRegistry`

- **Source package**: `internal/action`
- **Source file**: `output_action.go`
- **Public fields**:
  - `Supported []string` — 6 値固定: `none`, `commit`, `pull-request`, `pull-request-merge`, `pull-request-squash`, `pull-request-rebase`
  - `UnsupportedMigration map[string]string` — 未対応値 → migration message テンプレート

    ```go
    {
        "gist":                  "output_action='gist' is not supported in this distribution. ...",
        "markdown commit":       "...",
        "markdown pull-request": "...",
        "markdown gist":         "...",
        "gist pull-request":     "...",
    }
    ```

- **Methods**:
  - `Validate(value string) error` — Supported に含まれれば nil、UnsupportedMigration にあればその message を error として return、それ以外は generic "unknown output_action" error
- **Owned by**: `internal/action/output_action.go::Registry`

---

## E-009: `Banner` (startup banner)

- **Source package**: `internal/action`
- **Source file**: `banner.go`
- **Public fields**: なし (pure function)
- **Methods**:
  - `PrintBanner(w io.Writer, info BannerInfo)` — w に English 固定の ASCII art を書く

    ```go
    type BannerInfo struct {
        Version    string
        Template   string
        Plugins    []string // sorted, with " (deprecated)" suffix for deprecated
        Mode       string   // "action" or "cli"
        Token      string   // masked
    }
    ```

- **Format**: `docs/design/13-appendix.md §E` 準拠の固定 ASCII art (上流 Node 版と semantic 互換)
- **Owned by**: `internal/action/banner.go::PrintBanner`

---

## 関連既存 Entity (参照のみ、本 spec では変更なし)

| Entity | Source | Used by M6 |
|--------|--------|-----------|
| `engine.Request` / `engine.Result` / `engine.Deps` | `internal/engine` | M6 はこれを populate して Compute を呼ぶ |
| `plugins.Data` / `plugins.PluginContext` | `internal/plugins` | M6 は engine.Compute 経由で間接利用 |
| `config.Settings` / `config.Token` / `config.MetadataLoader` | `internal/config` | M6 は token mask / settings.json / metadata.yml の loader を再利用 |
| `githubapi.REST` / `githubapi.GraphQL` | `internal/githubapi` | committer / token validator / data_changed comparator から呼ぶ |
| `xerrors.RetryableError` | `internal/errors` | retry policy の type-marker |
| `render.Hash` | `internal/render` | data-changed 比較 |

---

## まとめ

9 entity (E-001 〜 E-009) すべてが `internal/action/*.go` の 1 ファイルに 1 構造体で対応する設計。新規 Go パッケージは `internal/action` の 1 つのみ。`cmd/metrics-action/main.go` は entity を一切持たず、`action.Run(ctx)` / `action.RunCLI(ctx)` を呼ぶだけの shim。
