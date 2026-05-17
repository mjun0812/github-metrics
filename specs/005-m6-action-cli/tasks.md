---

description: "Task list for m6-action-cli (Action + CLI 兼用 binary、21 採用 plugin + classic template の HTTP 以外の唯一の実行手段)"
---

# Tasks: M6 — GitHub Action / CLI

**Input**: Design documents from `/specs/005-m6-action-cli/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: 必須 (constitution 原則 IV)。各 `internal/action/<file>.go` に対応する `<file>_test.go` を表テスト + edge case で書く。`tests/integration/` に `action_test.go` + `cli_test.go` を新規追加。各 SC が決定論的 test に対応する (spec SC-001 〜 SC-013)。

**Organization**: 6 フェーズ — Setup / Foundational / US1 (P1 MVP Action mode) / US2 (P1 data-changed + P2 PR) / US3 (P2 CLI) / Polish。各 user story は独立にテスト可能。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 並列実行可能 (別ファイル、未完了依存なし)
- **[Story]**: US1〜US3 のうちどのストーリーに属するか
- 各タスク行に対象ファイルの相対パスを明記

## Path Conventions

- 単一 Go プロジェクトレイアウト: `cmd/metrics-action/`, `internal/action/`, `internal/tools/gen-action-yml/`, `assets/`, `tests/`
- 詳細は [plan.md §Project Structure](./plan.md)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Action / CLI binary を build / test するための infra (cmd entry point、Makefile target、Dockerfile 雛形、action.yml 生成ツール skeleton)。後続フェーズすべてが依存する。

- [ ] T001 Create `cmd/metrics-action/main.go` entry point skeleton with `package main`, `func main()` that calls `action.Run(ctx)` / `action.RunCLI(ctx)` based on `GITHUB_ACTIONS` env detection. Add minimal `cmd/metrics-action/main_test.go` confirming the binary compiles + exits with usage when invoked with no args.
- [ ] T002 [P] Create `internal/action/` package with `doc.go` documenting the package's role (per [plan.md §Project Structure](./plan.md)). Add empty `internal/action/action.go` with `func Run(ctx context.Context) error` + `func RunCLI(ctx context.Context, args []string) error` stubs that return `errors.New("not implemented")` for now.
- [ ] T003 [P] Extend `Makefile` with `build-action`, `gen-action-yml`, `docker-build`, `docker-run-cli` targets per [quickstart.md §3](./quickstart.md#3-makefile-ターゲット). `build-action` MUST produce `bin/metrics-action`. The other targets MAY no-op until later phases land.
- [ ] T004 [P] Create `internal/tools/gen-action-yml/main.go` skeleton + `cmd/gen-action-yml_test.go` with `TestGen_EmptyPluginSet_ProducesMinimalActionYML` (placeholder — actual generation logic lands in T032). The skeleton reads `--output <path>` flag + iterates `assets/plugins/*/metadata.yml` (returns empty subset for now).
- [ ] T005 [P] Create `Dockerfile` skeleton per [research.md R-001](./research.md#r-001-docker-image-の-base-選定): multi-stage build (`golang:1.26-alpine` builder + chromium runtime), final stage copies `bin/metrics-action` to `/metrics-action` + sets `ENTRYPOINT ["/metrics-action"]`. The chromium layer reuses M3's existing Dockerfile pattern. Verify `docker build -t metrics-action:dev .` succeeds locally.
- [ ] T006 [P] Create `.github/workflows/release.yml` skeleton with 2 jobs: (a) `release-docker` triggered on semver tag (`v*.*.*`) pushes GHCR image with 3 tags (`vX.Y.Z`, `latest`, `sha-<short>`) per [clarification Q1](./spec.md#clarifications); (b) `release-binary` cross-compiles 4 platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) per [clarification Q2](./spec.md#clarifications) and uploads to GitHub Releases. Jobs may be NOT triggerable yet — finalization lands in T064.
- [ ] T007 Update `tests/compliance/compliance_test.go::scanRoots` to include `"cmd/metrics-action"` and `"internal/action"` so existing `TestNoUnadoptedPluginReference` walks the new M6 surface (SC-012). Verify `go test ./tests/compliance/...` still green (no M6 code yet, so no false-positives).

**Checkpoint (Phase 1)**: `go build ./cmd/metrics-action` succeeds (empty stub). `make build-action` works. `docker build -t metrics-action:dev .` succeeds (the stub binary inside the image just prints "not implemented" and exits 1, which is OK for the layer). Compliance tests still green.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `internal/action/` package の base helper (banner / inputs parsing / output_action validation / retry policy / token validation / outputs). これらが揃わないと US1 (P1 MVP) でも `engine.Compute` を呼ぶ前段が組めない。

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Banner + outputs

- [ ] T008 [P] Implement `internal/action/banner.go::PrintBanner(w io.Writer, info BannerInfo)` per [data-model.md E-009](./data-model.md#e-009-banner-startup-banner) + [research.md R-007](./research.md#r-007-banner-format-と-slog-handler) + [docs/design/13-appendix.md §E](../docs/design/13-appendix.md). English-fixed ASCII art format with version / template / sorted plugins / mode / masked token. Add `internal/action/banner_test.go::TestPrintBanner_SnapshotMatchesUpstream` (SC-003) using `bytes.Buffer`.
- [ ] T009 [P] Implement `internal/action/outputs.go::SetOutput(key, value string) error` per [research.md R-003](./research.md#r-003-github_output-への-append-仕様). Auto-switches between single-line `KEY=VALUE\n` and heredoc form when value contains newlines. Append to `$GITHUB_OUTPUT` file path. Add `internal/action/outputs_test.go::TestSetOutput_{SingleLine,Multiline,EmptyValue,EnvFileMissing}`.

### Inputs parsing + presets

- [ ] T010 [P] Implement `internal/action/inputs.go::ParseInputs(env map[string]string) map[string]any` that reads `INPUTS` (JSON, preferred) and `INPUT_<UPPER>` env vars per spec FR-001. The priority order (INPUTS > INPUT_<UPPER>) is preserved exactly. Add table test with 6 cases covering both shapes + 1 conflict case.
- [ ] T011 [P] Implement `internal/action/inputs.go::WildcardFilename(template, format string) string` per spec FR-008: `github-metrics.*` → `github-metrics.svg` / `.png` / `.jpeg` / `.json` based on `format`. Add table test with 5 cases (one per format + 1 no-wildcard pass-through).
- [ ] T012 Implement `internal/action/inputs.go::PresetBundle` per [data-model.md E-005](./data-model.md#e-005-presetbundle-presets-統合): `Load(path)` reads YAML, `MergeInto(inputs)` overlays the preset's `q:` map. Add `internal/action/inputs_test.go::TestPresetBundle_LoadAndMerge` with 4 cases.

### Output action validation

- [ ] T013 [P] Implement `internal/action/output_action.go::OutputActionRegistry` + `DefaultRegistry() *Registry` + `Validate(value) error` per [data-model.md E-008](./data-model.md#e-008-outputactionregistry) + [contracts/output-actions.md](./contracts/output-actions.md). The 6 supported values, the 5 unsupported migration messages, and the generic unknown-value message all land here. Add `internal/action/output_action_test.go::TestRegistry_Validate` per [contracts/output-actions.md §5](./contracts/output-actions.md#5-テスト戦略-sc-007-後半-1-test-case-per-unsupported-value).

### Retry policy

- [ ] T014 [P] Implement `internal/action/retry.go::RetryPolicy.Do(ctx, fn)` per [contracts/retry-policy.md](./contracts/retry-policy.md) + [research.md R-002](./research.md#r-002-xerrorsretryableerror-の判定-api). Uses `errors.As(err, &re *xerrors.RetryableError)` for classification — direct type assertion forbidden. Add `internal/action/retry_test.go` with 8 cases per [contracts/retry-policy.md §6](./contracts/retry-policy.md#6-テスト戦略-sc-010-7-retryquotatoken-paths).

### Token validation

- [ ] T015 [P] Implement `internal/action/token.go::TokenValidator.Validate(ctx) (ValidationResult, error)` per [data-model.md E-007](./data-model.md#e-007-tokenvalidator) + [contracts/token-validation.md](./contracts/token-validation.md). All 3 stages: format reject (`github_pat_*`), scope check (`HEAD /` X-OAuth-Scopes), quota check (`GET /rate_limit`). Token-empty + `use_mocked_data=true` is the bypass path. Add `internal/action/token_test.go` with 7 cases per [contracts/token-validation.md §6](./contracts/token-validation.md#6-テスト戦略-sc-010--sc-011).
- [ ] T016 [P] Implement `internal/action/token_mask_test.go` per [contracts/token-validation.md §6](./contracts/token-validation.md#6-テスト戦略-sc-010--sc-011) (the SC-011 token-masking part): 4 cases verifying that `<PAT>` literal never appears in banner output / error wrap / retry log / slog field. `config.Token`'s Stringer mask is the underlying mechanism.

### Notice / release announce

- [ ] T017 [P] Implement `internal/action/notice.go::CheckLatestRelease(ctx, rest, currentVersion) (latest string, noticeMsg string, err error)` per spec FR-021. `noticeMsg` is the user-facing English string ("A newer version v1.2.0 is available: ...") or empty. Add `internal/action/notice_test.go` with 3 cases (newer / same / GitHub API 5xx).

**Checkpoint (Phase 2)**: All foundational helpers exist + their unit tests are green. `internal/action/action.go::Run` / `RunCLI` are still stubs (`not implemented`) but the building blocks are now usable from US1 onward. `go test ./internal/action/...` green.

---

## Phase 3: User Story 1 — P1 MVP Action mode (Priority: P1) 🎯 MVP

**Goal**: `mjun0812/github-metrics@latest` を `uses:` で参照した GitHub workflow が、`engine.Compute` → SVG 出力 → `output_action=commit` でリポジトリに commit するところまで動く。

**Independent Test**: Docker image を build → `docker run -e INPUT_USER=octocat -e INPUT_TOKEN=MOCKED_TOKEN -e INPUT_TEMPLATE=classic -e INPUT_OUTPUT_ACTION=none -e INPUT_DRYRUN=yes -e INPUT_USE_MOCKED_DATA=true ghcr.io/mjun0812/github-metrics:dev` を起動 → exit 0 + `/renders/github-metrics.svg` に valid SVG 生成 → SVG 内に 21 plugin の DOM marker 痕跡が含まれる (M4 で確立済の plugin が走るため)。

### action.yml 生成

- [ ] T018 [US1] Implement `internal/tools/gen-action-yml/main.go` per [contracts/action-yml.md §1](./contracts/action-yml.md#1-生成方法-constitution-原則-iii--v) + [research.md R-004](./research.md#r-004-actionyml-inputs-の生成方法). Walk `assets/plugins/<slug>/metadata.yml` (採用 19 dir) + core inputs (M1-M3 defined) → emit `inputs:` section. Add `outputs:` (`metrics_url`, `metrics_sha`) + `runs.using: docker` + `runs.image: docker://ghcr.io/...`. Add `internal/tools/gen-action-yml/main_test.go::TestGen_AdoptedSetOnly` confirming no unadopted slug appears in output.
- [ ] T019 [US1] Run `make gen-action-yml` to produce the first real `action.yml`. Commit the result. Add a lefthook pre-commit hook entry that runs `make gen-action-yml && git diff --quiet action.yml` so drift between `metadata.yml` and `action.yml` is caught immediately.

### Action mode entry

- [ ] T020 [US1] Implement `internal/action/action.go::Run(ctx context.Context) error` per [data-model.md E-001](./data-model.md#e-001-invocation-per-run-context) + spec FR-001 / FR-002. Reads env (`INPUTS` JSON / `INPUT_<UPPER>` / `GITHUB_*`) → builds `*Invocation` → validates output_action (T013) → validates token (T015) → prints banner (T008) → invokes RetryPolicy.Do (T014) wrapping `engine.Compute` → writes the result to `/renders/<filename>` → invokes Committer (T023) when output_action != none && !dryrun → writes `$GITHUB_OUTPUT` (T009) → exits.
- [ ] T021 [US1] Implement skip detection in `internal/action/action.go::shouldSkip(eventPath string) (bool, string)` per spec FR-002. Reads `GITHUB_EVENT_PATH` file (a JSON payload), checks the commit message for `[Skip GitHub Action]` and `Auto-generated metrics for run #N` patterns. Add `internal/action/action_skip_test.go` with 4 cases (skip-marker present / skip-marker absent / event file missing / event file malformed).

### Committer — commit path

- [ ] T022 [P] [US1] Implement `internal/action/committer.go::Committer` struct per [data-model.md E-002](./data-model.md#e-002-committer-output_action-dispatch). Just the struct definition + `New(rest, inv) *Committer` constructor. No methods yet.
- [ ] T023 [US1] Implement `internal/action/committer.go::(*Committer).runCommit(ctx)` per [contracts/committer.md §2](./contracts/committer.md#2-commit-経路). The 8-step sequence: ensure branch → (optional data-changed check) → GET previous sha → PUT contents. All retry-eligible API calls wrapped via T014 RetryPolicy. Add `internal/action/committer_commit_test.go` with 4 cases (new file / existing file / data-changed skip [T028 dependency] / API 422 conflict → warning + exit 0).

### Integration test

- [ ] T024 [US1] Add `tests/integration/action_test.go::TestAction_MockedFullPath_CommitMode` per spec SC-001. Build the binary, run it via `os/exec` with `INPUT_*` env set, assert exit code 0 + `/renders/github-metrics.svg` is valid SVG + GitHub mock (httptest) received the expected `PUT /contents/...` request.
- [ ] T025 [US1] Add `tests/integration/action_test.go::TestAction_SkipMessage_ExitsZero` per spec SC-004 row (`200 cache hit` analog). Provides a mock `GITHUB_EVENT_PATH` JSON with a `[Skip GitHub Action]` commit message → assert exit 0 + zero engine invocations.
- [ ] T026 [US1] Add `tests/integration/action_test.go::TestAction_TokenValidationMatrix` covering 4 scenarios of SC-010: `token rejected (github_pat_*)`, `token missing`, `quota insufficient → skipped`, `scope warning + continue`. Each scenario uses mocked `HEAD /` / `GET /rate_limit` responses.

**Checkpoint (US1)**: P1 MVP 動作。`docker run` 1 発で `engine.Compute → SVG → commit` の最短経路が成立。SC-001 / SC-004 / SC-010 (partial) を満たす。

---

## Phase 4: User Story 2 — P1 data-changed + P2 PR mode (Priority: P1-P2)

**Goal**: `output_condition=data-changed` で空 commit を抑制 + `output_action=pull-request[-merge|-squash|-rebase]` で PR レビューフローを動かす。

**Independent Test**: 2 連続実行で 1 回目 commit / 2 回目 skip を確認 (data-changed)。`output_action=pull-request-merge` で branch 作成 → SVG commit → PR 作成 → mergeable → auto-merge → head branch 削除の全 sequence を mocked GitHub backend で完走。

### data-changed

- [ ] T027 [P] [US2] Implement `internal/action/data_changed.go::HashComparator` per [data-model.md E-003](./data-model.md#e-003-hashcomparator-data-changed) + [contracts/committer.md §2 step 4](./contracts/committer.md#2-commit-経路) + [research.md R-006](./research.md#r-006-data-changed-用の既存ファイル取得--hash-比較). `Equal(ctx) (bool, error)` reads `GET /contents/<filename>?ref=<branch>`, base64-decodes `content`, calls `render.Hash` (M3 既存) → compares with new body's hash. Add `internal/action/data_changed_test.go` with 4 cases (match / mismatch / 404 / 5xx → RetryableError).
- [ ] T028 [US2] Wire `HashComparator` into `internal/action/committer.go::runCommit` (step 4 of the commit sequence). Update `committer_commit_test.go::TestCommit_DataChangedSkip` (T023 case) to actually exercise the comparator now. Add `tests/integration/action_test.go::TestAction_DataChanged_TwoRuns` per spec SC-005: first run commits, second run with identical Compute result skips (zero new PUT call).

### Committer — pull-request variants

- [ ] T029 [P] [US2] Implement `internal/action/committer.go::(*Committer).runPullRequest(ctx)` per [contracts/committer.md §3](./contracts/committer.md#3-pull-request-経路): branch (`metrics-run-<runId>`) creation → data-changed check → contents commit → `POST /pulls`. Add `internal/action/committer_pr_test.go::TestPR_{NewRun,DataChanged_NoPR,DuplicateBranch,APIFailure_WarningExitZero}`.
- [ ] T030 [US2] Implement `internal/action/committer.go::(*Committer).runPullRequestMerge(ctx, mergeMethod string)` per [contracts/committer.md §4](./contracts/committer.md#4-pull-request-merge--squash--rebase-経路). Re-uses `runPullRequest`, then polls `GET /pulls/{n}` for `mergeable=true` (max 30s @ 5s interval) → `PUT /pulls/{n}/merge?merge_method=<method>` → cleanup head branch via `DELETE /git/refs/heads/<branch>`. The `mergeMethod` is one of `merge` / `squash` / `rebase`. Update committer_pr_test.go with `TestPRMerge_{Success,MergeableTrueImmediately,MergeableFalseTimeout,Squash,Rebase}`.
- [ ] T031 [US2] Wire all 5 PR variants (`pull-request`, `pull-request-merge`, `pull-request-squash`, `pull-request-rebase`) into `Committer.Run(ctx)` dispatcher in `internal/action/committer.go`. Add `tests/integration/action_test.go::TestAction_PullRequestMerge_E2E` per spec SC-006 covering the full sequence end-to-end against a mocked GitHub backend.

### Misc

- [ ] T032 [US2] Hook the `internal/action/notice.go` release check (T017) into `Run(ctx)` startup when `notice_releases=yes`. Print the message as an `slog.Info` line (English fixed). No new test file needed beyond T017's unit test.

**Checkpoint (US2)**: data-changed (SC-005) + PR + auto-merge (SC-006) + 7 controlled-error matrix (SC-007 partial — unsupported `output_action` value test lands here as well, see next bullet) すべて動く。

- [ ] T033 [US2] Add `tests/integration/action_test.go::TestAction_OutputAction_Unsupported_FailFast` per spec SC-007 後半. Sets `INPUT_OUTPUT_ACTION=gist` → assert exit 1 + stderr contains the migration message + assert `engine.Compute` was **never invoked** (mock call counter).

---

## Phase 5: User Story 3 — P2 CLI mode (Priority: P2)

**Goal**: `metrics-action --user octocat --template classic --output svg --dryrun --filename -` のような CLI 起動が、Action mode と同じ engine.Compute 結果を生成する。

**Independent Test**: `bin/metrics-action --user octocat --token-env GITHUB_TOKEN --output svg --dryrun --filename -` を実行 → stdout に valid SVG が流れる (mocked deps なら 30s 以内、SC-008)。

### CLI flag parser + YAML config loader

- [ ] T034 [P] [US3] Implement `internal/action/cli.go::CLIFlags` struct + `ParseFlags(args []string) (*CLIFlags, error)` per [data-model.md E-004](./data-model.md#e-004-cliflags-cli-mode-input) + [contracts/cli-flags.md §1-§3](./contracts/cli-flags.md#1-フラグ一覧). Use standard `flag` package + a thin wrapper for repeatable `--plugin key=value` (per [research.md R-005](./research.md#r-005-cli-flag-パーサ選定)). No subcommand structure — single-shot only.
- [ ] T035 [P] [US3] Implement `internal/action/cli.go::LoadYAMLConfig(path string) (map[string]any, error)` per [contracts/cli-flags.md §6](./contracts/cli-flags.md#6---config-pathyaml-schema). Reads YAML via `gopkg.in/yaml.v3` (M1 既存), flattens `plugins:` / `config:` / `committer:` nested maps into `plugin_*` / `config_*` / `committer_*` flat keys matching action.yml. Add `internal/action/cli_test.go::TestLoadYAMLConfig_{Minimal,Full,InvalidYAML,Conflicting}`.
- [ ] T036 [US3] Implement `internal/action/cli.go::(*CLIFlags).ToInvocation(env map[string]string) (*Invocation, error)`. Merges CLI flags (highest priority) + YAML config (T035) + env (INPUT_<UPPER>) + preset (T012) into the unified `map[string]any` per [contracts/cli-flags.md §3](./contracts/cli-flags.md#3-入力優先度-cli-mode). Add `internal/action/cli_test.go::TestToInvocation_PriorityOrder` with 5 paired test cases (= SC-009).

### `--token-env` + `--filename -`

- [ ] T037 [P] [US3] Implement `internal/action/cli.go::ResolveToken(flags *CLIFlags) (string, error)` per [contracts/cli-flags.md §4](./contracts/cli-flags.md#4-token-入力の取り扱い). Priority: `--token-env` (preferred) > `--token` (warning emitted) > env-only (`INPUT_TOKEN`) > error. Add `internal/action/cli_test.go::TestResolveToken_{Env,Flag,Both,Neither_Mocked,Neither_RealRequired}`.
- [ ] T038 [P] [US3] Implement `internal/action/cli.go::ResolveOutputWriter(filename string, format string) (io.Writer, func() error, error)` per [contracts/cli-flags.md §5](./contracts/cli-flags.md#5---filename-の-semantics). `filename == "-"` returns `os.Stdout` + warning for binary formats. Normal paths get `os.Create` + `mkdir -p` of parent. Returned cleanup func closes the file. Add `internal/action/cli_test.go::TestResolveOutputWriter_{Stdout,FilePath,DirsCreated,BinaryToStdoutWarning}`.

### CLI mode entry

- [ ] T039 [US3] Implement `internal/action/action.go::RunCLI(ctx context.Context, args []string) error`. Parses flags (T034) → loads YAML (T035) → resolves token (T037) → builds `*Invocation` (T036) → validates output_action (T013, even in CLI mode for symmetry) → validates token (T015) → prints banner (T008) → calls `engine.Compute` (with RetryPolicy T014) → writes output via T038 writer → if !dryrun && output_action != none → Committer (US1/US2 path). Update `cmd/metrics-action/main.go` to dispatch: `GITHUB_ACTIONS=true` → `action.Run(ctx)`, otherwise → `action.RunCLI(ctx, os.Args[1:])`.

### Integration tests

- [ ] T040 [US3] Add `tests/integration/cli_test.go::TestCLI_OctocatSVG_Stdout` per spec SC-008. Run `bin/metrics-action --user octocat --plugin use_mocked_data=true --template classic --output svg --dryrun --filename -` via `os/exec` → assert valid SVG on stdout within 30s.
- [ ] T041 [US3] Add `tests/integration/cli_test.go::TestCLI_ConfigYAML_Equivalence` per spec SC-009. 5 paired (`--config x.yaml`, equivalent `INPUT_*` env set) cases × verify resulting `engine.Request` is byte-identical (or `reflect.DeepEqual` true). Mocked engine deps in both cases.

**Checkpoint (US3)**: CLI が action.yml と等価な入力で同じ output を生成。SC-008 / SC-009 を満たす。Action / CLI のどちらでも `--dryrun` で commit/PR をスキップできる。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 残りの SC 担保 + release pipeline finalize + compliance test 拡張 + docs 更新。

- [ ] T042 [P] Extend `tests/compliance/compliance_test.go::TestCompliance_M4_AdoptedPlugins` to assert that `internal/action/` / `cmd/metrics-action/` exist but contain ZERO plugin / template subdirectories (i.e., M6 does not introduce new adopted slugs). The 19 plugin dirs invariant from M4 stays untouched.
- [ ] T043 [P] Finalize `Dockerfile` per [research.md R-001](./research.md#r-001-docker-image-の-base-選定). Multi-stage build with `golang:1.26-alpine` builder → static binary (CGO_ENABLED=0) → chromedp/headless-shell runtime base → COPY binary + chromium runtime + `/renders` working dir → ENTRYPOINT. Verify `docker build -t metrics-action:dev .` succeeds + `docker run --rm metrics-action:dev --help` prints CLI usage.
- [ ] T044 [P] Finalize `.github/workflows/release.yml` per [clarification Q1 + Q2](./spec.md#clarifications). The `release-docker` job builds + pushes 3 tags (`vX.Y.Z`, `latest`, `sha-<short>`) to `ghcr.io/mjun0812/github-metrics`. The `release-binary` job runs `go build -trimpath -ldflags="-s -w -X main.version=<tag>"` for 4 GOOS/GOARCH matrix entries + uploads to GitHub Releases. Trigger on `push: tags: v*.*.*`.
- [ ] T045 [P] Add `tests/integration/action_test.go::TestAction_BannerSnapshot` per spec SC-003. Capture `internal/action/banner.go::PrintBanner` output to a `bytes.Buffer` + diff against the committed snapshot in `tests/golden/action/banner.txt`. Use `-update` flag pattern (existing M4 convention). Semantic comparison (version / plugins / template / mode lines must each appear) rather than byte-exact (timestamps / order tolerance).
- [ ] T046 [P] Add `tests/integration/action_test.go::TestAction_OutputAction_FailureWarning_Matrix` per spec SC-007 前半. 4 deterministic failure-injection cases (`commit_API_403_branch_protection`, `pr_creation_422_conflict`, `merge_409_conflict`, `merge_method_unavailable`) — each MUST log a warning + exit 0 (action does not block workflow).
- [ ] T047 [P] Run the full quickstart end-to-end on a clean checkout per [quickstart.md §4-§5](./quickstart.md). Document any issue found in the PR description. Capture pass/fail per quickstart step. Verify `make test` / `make test-chromedp` / `make test-heavy` all green on the maintainer environment (mac OS + Apple M5 + system Chrome).
- [ ] T048 Update `README.md` "Status" line: M6 (Action / CLI) complete. Add `## Usage as a GitHub Action` section + `## Usage as a CLI` section pointing at quickstart sections. Document `uses: mjun0812/github-metrics@vX.Y.Z` + `docker run ghcr.io/mjun0812/github-metrics:latest` examples.

**Checkpoint (Phase 6)**: 全 13 SC をカバー + 3 並列 CI ジョブ green + Docker image + GitHub Releases binary が release pipeline で publish 可能。M6 spec が完全に閉じる。

---

## Dependencies

### Phase ordering

- **Phase 1 Setup** (T001-T007) → Phase 2 Foundational (T008-T017) → User Story phases (T018-T041) → Phase 6 Polish (T042-T048)
- User Story phases can run partially in parallel after Phase 2 completes, but T024-T026 (US1 integration) requires all US1 tasks complete; T028 / T031 / T033 (US2 integration) requires US1 + the US2 implementation tasks first; T040-T041 (US3 integration) requires the CLI implementation in T036-T039.

### Cross-phase blockers

- **T013 (output_action validator)** blocks T020 (Run) + T039 (RunCLI) + T033 (unsupported value integration test)
- **T014 (RetryPolicy)** blocks T020 + T023 (committer commit) + T030 (PR + merge) + T039 (RunCLI Compute wrapper)
- **T015 (TokenValidator)** blocks T020 + T039 + T026 (token matrix integration test)
- **T018 (gen-action-yml)** blocks T019 (first action.yml) + T024 (action integration test needs the binary to find inputs)
- **T022 (Committer struct)** blocks T023 / T029 / T030 / T031 (all committer methods)
- **T027 (HashComparator)** blocks T028 (commit wire-up) + the data-changed cases in T029 (PR also data-changed-aware)
- **T034 (CLIFlags) + T035 (LoadYAMLConfig)** block T036 (ToInvocation) → T039 (RunCLI)

### Parallel opportunities within phases

- **Phase 1 (Setup)**: T002, T003, T004, T005, T006 are all [P] (different files). T001 must complete first to give `cmd/metrics-action/main.go` something to import. T007 is also [P] but better to run alongside Phase 2 since it's pure test wiring.
- **Phase 2 (Foundational)**: T008, T009 are [P]. T010, T011 are [P] (both in `inputs.go` but different functions and different test cases). T012 sequential after T010 / T011 (touches same file). T013, T014, T015, T017 are all [P] amongst themselves and with the inputs group. T016 [P] runs alongside T015.
- **Phase 3 (US1)**: T018 + T019 must sequence (T019 runs the tool from T018). T020 / T021 / T022 are sequential against same `internal/action/action.go` / `committer.go`. T023 sequential after T022. T024 / T025 / T026 are [P] amongst themselves (different test cases in same test file is OK with `t.Run` subtests but separate task entries here keep ownership clear).
- **Phase 4 (US2)**: T027 [P] with T029. T028 sequential after T027. T030 sequential after T029 (same file). T031 sequential after T030. T032 [P] (independent — `notice.go` was implemented in Phase 2 T017). T033 [P] (independent integration test).
- **Phase 5 (US3)**: T034 / T035 are [P]. T036 sequential after both. T037 / T038 are [P]. T039 sequential after T036/T037/T038. T040 / T041 are [P].
- **Phase 6 (Polish)**: T042 / T043 / T044 / T045 / T046 / T047 are all [P]. T048 sequential after T047 (uses quickstart pass status).

### Suggested execution flow (1 developer)

```text
T001
 └─ T002 ∥ T003 ∥ T004 ∥ T005 ∥ T006 ∥ T007   (Phase 1 parallel batch)
     └─ T008 ∥ T009 ∥ T010 ∥ T011 → T012      (Phase 2 inputs / banner / outputs)
         ∥ T013 ∥ T014 ∥ T015 ∥ T017          (Phase 2 validators / retry, parallel)
         ∥ T016                                (Phase 2 token mask test, parallel with T015)
             └─ Phase 3 (US1):
                T018 → T019                    (gen-action-yml first)
                T022 → T023                    (committer struct + commit path)
                T020 → T021                    (Run + skip detection)
                T024 ∥ T025 ∥ T026              (US1 integration tests, parallel)
                 └─ Phase 4 (US2):
                    T027 → T028                (data-changed)
                    T029 → T030 → T031         (PR + merge variants)
                    T032 ∥ T033                (notice + unsupported integration)
                     └─ Phase 5 (US3):
                        T034 ∥ T035 → T036     (CLI flags + YAML + ToInvocation)
                        T037 ∥ T038            (token + writer)
                         → T039                (RunCLI entrypoint)
                         → T040 ∥ T041         (US3 integration tests)
                             └─ Phase 6 polish T042-T048
```

### Suggested execution flow (2 developers parallel)

```text
After Phase 2 (T008-T017) completes:

Dev A:
  - Phase 3 US1 (T018-T026)
  - then Phase 4 US2 commit-side (T027-T028)
  - then Polish T042 / T045 / T046 / T047 / T048

Dev B:
  - Phase 4 US2 PR-side (T029-T033)
  - then Phase 5 US3 (T034-T041)
  - then Polish T043 / T044
```

---

## Implementation Strategy

- **MVP first**: US1 (P1 Action mode commit path) を完走させて release できる状態を作る。`uses: mjun0812/github-metrics@vX.Y.Z` の最小契約が成立する変化点。
- **Incremental delivery**: US1 → US2 → US3 の順で merge する 3 PR 分割 (Phase 6 polish が 4 PR 目)。M4 で確立した PR 分割パターン (Phase 1-3 / Phase 4 / Phase 5 / Phase 6) を踏襲。
- **Foundational gating**: Phase 2 (T008-T017) の helper はすべて他 phase から呼ばれるので、Phase 2 を完走 + table test 全緑にしてから Phase 3 着手 MUST。途中で API を変更すると下流の作業が無駄になる。
- **Compliance & scope**: 各 phase 完了時に `make test` + `go test ./tests/compliance/...` を実行。19 不採用 plugin slug が production code に漏れないこと + M4 採用 21 plugin dir が変わらないことを継続確認。
- **CLI と Action の対称性**: T036 (`CLIFlags.ToInvocation`) と T020 (`Run` の `Invocation` 構築) が同じ map[string]any 形式に収束することが SC-009 の前提。両者の output を 1 箇所 (`internal/action/inputs.go::ParseInputs`) に通すアーキテクチャを Phase 2 で確立しておく。
- **Golden file 戦略**: M6 では新規 plugin / template の golden は無い (既存 M2-M4 のものを継承)。M6 固有の golden は banner snapshot (`tests/golden/action/banner.txt`) のみ。

## 完了基準

T001-T048 すべて green、Quickstart 7 ステップ pass、CI 通常ジョブ + chromedp ジョブ + heavy ジョブの 3 並列が緑、Action mode の Docker run と CLI mode の binary 実行で同じ engine.Compute 結果が出る、release pipeline (T044) で semver tag push → GHCR push + GitHub Releases binary publish までが自動化される、`tests/compliance/compliance_test.go` の M4 / M6 invariant が全て緑、SC-001 〜 SC-013 の 13 メトリクスが deterministic test case で検証されている。
