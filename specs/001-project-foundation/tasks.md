---

description: "Task list for project-foundation (M1 19 baseline tasks)"
---

# Tasks: プロジェクト土台 (M1 19 タスク一括)

**Input**: Design documents from `/specs/001-project-foundation/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: 必須 (constitution 原則 IV + FR-030 により MUST)。各実装タスクの直後にテーブルテスト + 必要に応じて golden file テストを書くこと。

**Organization**: 8 フェーズ — Setup / Foundational / US1〜US5 / Polish。各 user story は独立にテスト可能。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 並列実行可能 (別ファイル、未完了依存なし)
- **[Story]**: US1〜US5 のうちどのストーリーに属するか
- 各タスク行に対象ファイルの相対パスを明記

## Path Conventions

- 単一 Go プロジェクトレイアウト: `cmd/<binary>/`, `internal/<pkg>/`, `assets/`, `scripts/`, `tests/`, `.github/workflows/`
- 詳細は [plan.md §Project Structure](./plan.md) を参照

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: リポジトリ骨格 + ビルド / CI 配管の確立。後続フェーズの全タスクが依存する。

- [x] T001 Initialize `go.mod` at repo root with `module github.com/mjun0812/github-metrics` and `go 1.23` directive (覆われる FR: FR-001)
- [x] T002 [P] Create directory skeleton: `cmd/metrics-action/`, `cmd/metrics-cli/`, `internal/`, `assets/`, `scripts/`, `tests/fixtures/`, `tests/golden/`, `tests/integration/`, `.github/workflows/` (空ディレクトリは `.gitkeep` で git 追跡)
- [x] T003 [P] Add `LICENSE` file at repo root with MIT license text (upstream `lowlighter/metrics` の MIT 表記を踏襲、Copyright 行は本リポジトリの所有者で更新) — constitution Development Workflow ライセンス遵守項目
- [x] T004 Write `Makefile` at repo root with targets: `build` (両バイナリ to `bin/`), `test`, `test-race` (`-race`), `lint` (`golangci-lint` + `govulncheck`), `bench`, `gen` (`go generate ./...`), `docker` (placeholder), `e2e` (placeholder), `tools` (golangci-lint + govulncheck install), `check-compat` (placeholder, T048 で本実装) — FR-002
- [x] T005 [P] Write `.golangci.yml` per research R-008 enabling: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `revive`, `gofumpt`, `gosec`, `nilerr`, `prealloc`, `unparam`; disable `exhaustive`, `exhaustruct`, `wrapcheck`; set `run.timeout: 10m`
- [x] T006 [P] Write `.github/workflows/go-ci.yml` with PR-triggered jobs: `test` (`go test ./...`), `vet` (`go vet ./...`), `lint` (`golangci-lint run --timeout=10m`), `vuln` (`govulncheck ./...`), `test-race` (`go test -race ./...`), `smoke-macos` (build + `go test ./internal/config/...` on `macos-latest`); use `ubuntu-latest` primary — FR-003
- [x] T007 [P] Add `scripts/sync-assets.sh` skeleton (実体は T024 [US2] で実装) which prints "Not yet implemented" and exits 1 — placeholder to wire into `Makefile`'s `make sync-assets` invocation
- [x] T008 Add minimal `cmd/metrics-action/main.go` and `cmd/metrics-cli/main.go` each with `--help` flag that prints usage and exits 0; both must compile with no warnings — FR-001, US1 AS1

**Checkpoint (Phase 1)**: `make build` produces `bin/metrics-action` and `bin/metrics-cli`; `--help` returns exit 0; CI workflow file exists.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 全 user story の共通基盤 (logger / errors / context / format)。これが完了するまで US2〜US5 を着手しない。

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T009 [P] Implement `internal/logger/logger.go` with `slog` JSON default handler, `--log-format text` switch, `--debug` level toggle; add `internal/logger/logger_test.go` with table tests for level switching and handler swap (research R-006) — FR-004
- [x] T010 [P] Implement `internal/errors/errors.go` defining 5 struct error types (`InputError`, `NotFoundError`, `ForbiddenError`, `UnsupportedFormatError`, `RetryableError`) with `Error()` and `Unwrap()` methods; add `internal/errors/errors_test.go` with table tests covering `errors.Is` and `errors.As` (research R-011) — FR-005
- [x] T011 [P] Implement `internal/ctxutil/ctxutil.go` with `WithLogin(ctx, login)` and `LoginFromContext(ctx) (string, bool)`; add `internal/ctxutil/ctxutil_test.go` verifying round-trip + slog handler picks up the attribute — FR-006
- [x] T012 [P] Implement `internal/format/format.go` with `Format(n, opts)`, `FormatBytes(n)`, `FormatPercentage(n, opts)`, `FormatDate(t, opts)`, `Ellipsis(s, n)`, `S(n, suffix)`, `FormatError(err, opts)`; add `internal/format/format_test.go` with table tests covering boundary values (0, 999, 1000, 999999, 1000000) and timezone switching — FR-007
- [x] T013 Run `go test ./internal/logger/... ./internal/errors/... ./internal/ctxutil/... ./internal/format/...` and confirm 100% green; verify package coverage ≥ 80% per SC-005 using `go test -cover` and update report in PR description

**Checkpoint (Phase 2)**: Foundational packages compile, all tests pass, coverage ≥ 80% on 4 packages.

---

## Phase 3: User Story 1 - ビルド可能で CI 緑のプロジェクト骨格 (Priority: P1) 🎯 MVP

**Goal**: 貢献者がリポジトリをクローンして `make build && make test` を実行すると両バイナリがビルド・テストパスし、PR 上で CI 全 jobs が緑になる状態を達成する。

**Independent Test**: クリーン環境で clone → `make build && make test` → PR を開いて CI 4 jobs (`test` / `vet` / `lint` / `vuln`) が緑になることを確認。Quickstart §1 + §7 で記載された手順そのまま。

### Tests for User Story 1

- [x] T014 [P] [US1] Add `internal/logger/logger_context_test.go` proving `slog.Default()` emits `login` attribute when context is enriched via `ctxutil.WithLogin` — US1 AS4
- [x] T015 [P] [US1] Add `tests/integration/binaries_test.go` invoking compiled `bin/metrics-action --help` and `bin/metrics-cli --help` via `os/exec` and asserting exit code 0 and presence of "Usage:" in stdout — US1 AS1 / AS2

### Implementation for User Story 1

- [x] T016 [US1] Wire foundational packages into `cmd/metrics-action/main.go` and `cmd/metrics-cli/main.go`: initialize logger, set up signal-aware context with `signal.NotifyContext`, ensure `--help` flag goes through `flag.NewFlagSet` (Cobra deferred to T-117); keep both mains stubbed to print version banner and exit 0 if no other flags given — US1 AS1
- [x] T017 [US1] Open PR against `main` to validate `.github/workflows/go-ci.yml` runs all jobs green within 7 minutes (SC-001); attach CI run URL in PR body — US1 AS3

**Checkpoint (US1)**: バイナリビルド + CI 緑が確認できる「Buildable shell」段階。MVP として deploy 可能 (機能はまだなし、ただし配管は整備済)。

---

## Phase 4: User Story 2 - 上流互換の設定入力レイヤ (Priority: P1)

**Goal**: 上流 `lowlighter/metrics` と同じ `action.yml` インプット + `settings.json` キー集合を、本実装が **キー差分 0** で読み込み、内部正規化済み入力マップに変換できる状態。

**Independent Test**: 採用 21 plugin + base/core + classic/repository の `metadata.yml` を `embed.FS` 経由でロード、`tests/fixtures/inputs/*.yml` のケースを `Inputs.ForAction` に通して `tests/golden/inputs/*.json` と比較。GitHub API 通信は不要。

### Tests for User Story 2

- [x] T018 [P] [US2] Write fixture files `tests/fixtures/settings/{empty.json, with_comments.json, sandbox.json}` and golden expectations `tests/golden/settings/*.json`; add table-test cases in `internal/config/settings_test.go` covering load-absent (returns `Settings{Port: 3000}`), `//` key strip, and Sandbox=true override path — US2 AS1 / AS2, FR-008
- [x] T019 [P] [US2] Write fixture files `tests/fixtures/inputs/{user_basic.yml, sandbox.yml, action.yml}` and golden expectations `tests/golden/inputs/*.json`; add table tests in `internal/config/inputs_test.go` covering each `type` (string / number / boolean / array (3 formats) / token / json), `.user.login` placeholder resolution, and `Token.String()` returns `(provided)` — US2 AS3 / AS4 / AS5, FR-010 / FR-011 / FR-012

### Implementation for User Story 2

- [x] T020 [P] [US2] Implement `internal/config/inputs_token.go` with `Token` struct (raw string), `NewToken`, `String() string` returning `(provided)` or empty, `Reveal() string`, `MarshalJSON()` returning `"(provided)"`, `GoString()` returning `"(provided)"` (data-model E-007) — FR-012
- [x] T021 [US2] Implement `internal/config/settings.go` with `Settings` struct (data-model E-001), `Load(path string) (*Settings, error)`, `stripCommentKeys(raw []byte) ([]byte, error)` for `//`-prefixed key removal (research R-005), and Sandbox enforcement (`Optimize=true / Cached=0 / PluginsDefault=true / Extras.Default=true / Mocked=true`); make tests from T018 pass — FR-008
- [x] T022 [US2] Implement `internal/config/metadata.go` with `PluginMetadata` / `TemplateMetadata` / `ActionMetadata` / `PackageMetadata` / `InputDef` structs (data-model E-002), `MetadataLoader` aggregate (data-model E-003), and `Load(fsys fs.FS) (*MetadataLoader, error)`; add `to.Action(key)` / `to.Web(key)` / `to.Query(key)` / `Extras(name, settings)` helpers — FR-009, US2 AS5
- [x] T023 [US2] Implement `internal/config/inputs.go` with `Inputs` struct (data-model E-004), `NormalizeInput(def InputDef, raw any) (any, error)`, `Inputs.ForAction(env, preset)` / `Inputs.ForWeb(query)` / `Inputs.ForData(data, q, account)`, and placeholder resolver (`.user.login` etc.); make tests from T019 pass — FR-010, FR-011
- [x] T024 [US2] Implement `scripts/sync-assets.sh` (full body) to copy `./org_repo/source/plugins/{base,core,languages,activity,achievements,repositories,isocalendar,calendar,habits,stars,topics,starlists,people,notable,contributors,reactions,projects,sponsors,sponsorships,stargazers,traffic}/*` (採用 21 + base/core) and `./org_repo/source/app/web/statics/.../templates/{classic,repository}/*` into `assets/plugins/<name>/` and `assets/templates/<name>/`; compute SHA-256 checksums and write `assets/.upstream.lock` with `commit:<sha>` and per-file checksums; **MUST** skip community / unadopted plugin/template directories (constitution 原則 III); script must be idempotent and pure (no network) — T-012, research R-009
- [x] T025 [US2] Add `//go:embed assets/*` directive in `internal/config/embed.go` and ensure `MetadataLoader.Load(embedFS)` completes within 200 ms (benchmark in `internal/config/metadata_bench_test.go`) — FR-018, SC-003

**Checkpoint (US2)**: `settings.json` + `metadata.yml` + `INPUTS` / `INPUT_*` → 内部正規化マップへの変換が完結。`check-compat` (T048) を待たずに、unit + golden tests でキー網羅を保証。

---

## Phase 5: User Story 3 - GitHub API クライアントとレート可視化 (Priority: P2)

**Goal**: mocked GitHub に対し REST / GraphQL / レート照会を発行できる薄いラッパを提供。`MOCKED_TOKEN` 経路では実 GitHub URL への通信を MUST NOT 発生させる。

**Independent Test**: `httptest.NewServer` で REST `/rate_limit` / GraphQL endpoint を mock、リトライ・status code 別挙動・User-Agent ヘッダ・token 種別判定を assert。`MOCKED_TOKEN` で実 URL を叩こうとすると panic することを `panics-recover` パターンで確認。

### Tests for User Story 3

- [x] T026 [P] [US3] Add `internal/httpx/client_test.go` with table tests using `httptest.NewServer`: 503→503→200 retries succeed (3 attempts), 404 does not retry, `Retry-After: 1` header honored, User-Agent matches `metrics/<version> (+https://github.com/mjun0812/github-metrics)` (FR-013) — US3 AS1
- [x] T027 [P] [US3] Add `internal/githubapi/auth_test.go` with table tests for `ClassifyToken`: `ghp_xxx` → TokenClassic, `gho_xxx` → TokenClassic, `ghu_xxx` → TokenClassic, `ghs_xxx` → TokenClassic, `ghr_xxx` → TokenClassic, `github_pat_xxx` → TokenFineGrained (early reject), `NOT_NEEDED` → TokenNone, `MOCKED_TOKEN` → TokenMocked, garbage → TokenUnknown — US3 AS4
- [x] T028 [P] [US3] Add `internal/githubapi/rest_test.go` with mocked transport invoking `RateLimit(ctx)` and asserting `Resources.REST.Remaining` reflects fixture; add `internal/githubapi/rate_test.go` with `go test -race` confirming `Refresh` and `Snapshot` are concurrent-safe — US3 AS2
- [x] T029 [P] [US3] Add `internal/githubapi/graphql_test.go` with mocked transport executing the `User(login)` generated client function and asserting decoded struct fields match fixture — US3 AS3
- [x] T030 [P] [US3] Add `internal/githubapi/mocked_guard_test.go` constructing `mockedGuardRoundTripper` and invoking it with `https://api.github.com/foo`; assert `defer recover() != nil` captures the expected panic message — US3 AS5, FR-017

### Implementation for User Story 3

- [x] T031 [P] [US3] Implement `internal/httpx/client.go` wrapping `hashicorp/go-retryablehttp` (research R-002) with `Get` / `PostJSON` / `PostForm` / `Binary` / `ImgB64` methods; configure exponential backoff for 5xx/429/network, no retry for 4xx, respect `Retry-After` header, set User-Agent `metrics/<version> (+https://github.com/mjun0812/github-metrics)`; make T026 pass — FR-013, contracts/github-api §1
- [x] T032 [P] [US3] Implement `internal/githubapi/auth.go` with `TokenKind` enum and `ClassifyToken(raw string) TokenKind`; reject `github_pat_*` early with `*InputError{Field: "token"}`; make T027 pass — FR-014, contracts/github-api §2.1
- [x] T033 [US3] Implement `internal/githubapi/rest.go` with `REST` struct, `NewREST(token, customBaseURL, opts) (*REST, error)`, `RateLimit(ctx)` calling `/rate_limit`, `HeadRoot(ctx)`; depends on T031 + T032 — FR-014
- [x] T034 [US3] Implement `internal/githubapi/rate.go` with `Resources` / `Quota` structs (data-model E-008), `NewResources()`, `Refresh(ctx, c *REST) error`, `Snapshot() Resources` with `sync.RWMutex`; make T028 pass; depends on T033 — FR-016
- [x] T035 [US3] Create `assets/plugins/base/schema.graphql` by running `gh api graphql -F query=@introspection.graphql > assets/plugins/base/schema.graphql` (固定 commit hash record in PR), then add `genqlient.yaml` at repo root pointing to `assets/plugins/base/schema.graphql`, `assets/plugins/base/queries/*.graphql`, and generated output `internal/githubapi/graphql_gen.go`; run `make gen` to populate generated file — FR-015, research R-003
- [x] T036 [US3] Implement `internal/githubapi/graphql.go` with `GraphQL` struct wrapping `genqlient.Client`, `NewGraphQL(token, customBaseURL, opts) (*GraphQL, error)`, and thin wrapper methods (`User(ctx, login)`, `UserX(ctx, login, fields)`); make T029 pass; depends on T035 — FR-015
- [x] T037 [US3] Implement `internal/githubapi/mocked_guard.go` with `mockedGuardRoundTripper` that panics on real GitHub hosts (`api.github.com`, `github.com`, `*.githubusercontent.com`); wire it into `NewREST` and `NewGraphQL` when `TokenKind == TokenMocked`; make T030 pass — FR-017, contracts/github-api §4
- [x] T038 [US3] Implement `internal/githubapi/testhelper.go` with `MockTransportStub` (minimal `http.RoundTripper` returning fixture-based responses) and `MockGraphQLDoer`; use these in subsequent US5 integration tests until M9 replaces with full `internal/testutil/mocks` — contracts/github-api §6

**Checkpoint (US3)**: GitHub API ラッパが mocked backend で動作。`MOCKED_TOKEN` panic guard が race-clean。レート可視化が `Refresh` / `Snapshot` で機能。

---

## Phase 6: User Story 4 - 並列実行可能な plugin / template レジストリ (Priority: P2)

**Goal**: plugin と template を `init()` で `Register()` するだけで登録でき、`core` プラグインが errgroup で並列実行、panic は recover、エラーは `data.Plugins[name]` に集約される状態。

**Independent Test**: 3 つの fake plugin (success / error / panic) を `RegisterForTest` で登録し、`core.RunPlugins(ctx, parallel=3)` の結果が `data.Plugins` に正しく集約されることを assert。実 GitHub API も実 template も不要。

### Tests for User Story 4

- [x] T039 [P] [US4] Add `internal/plugins/plugin_test.go` covering: `Register` succeeds, duplicate `Register` panics (US4 AS1), `Get(unknown)` returns `(nil, false)`, `Each` iterates in sorted order, `RegisterForTest` returns cleanup that restores prior registration — contracts/plugin-interface §3
- [x] T040 [P] [US4] Add `internal/templates/template_test.go` covering: `Register` succeeds, duplicate `Register` panics, `Get("classic")` returns `(nil, false)` in M1 (classic body deferred to M2), `MustGet("classic")` returns `*errors.NotFoundError`, US4 AS4 — contracts/template-interface §3
- [x] T041 [P] [US4] Add `internal/plugins/core/core_test.go` covering: `Asia/Tokyo` timezone resolution succeeds (US4 AS3), invalid IANA name falls back to UTC and records `data.Config.Timezone.Error`, zero-value initialization of `data.Computed` — FR-024
- [x] T042 [P] [US4] Add `internal/plugins/core/run_plugins_test.go` with 3 fake plugins (success / error / panic) registered via `RegisterForTest`; assert `data.Plugins["success"]` is the result value, `data.Plugins["error"]` is an `error`, `data.Plugins["panic"]` is a `*errors.RetryableError`; also assert `parallel=1` produces equivalent aggregation (US4 AS2 / AS5) — FR-025

### Implementation for User Story 4

- [x] T043 [US4] Implement `internal/plugins/plugin.go` with `Plugin` interface (data-model + contracts/plugin-interface §1), `PluginContext` struct (E-006 / §2), and `internal/plugins/registry.go` with `Register` / `Get` / `Each` / `RegisterForTest` (§3); make T039 pass — FR-020
- [x] T044 [P] [US4] Implement `internal/templates/template.go` with `Template` interface, `PartialFunc` / `PartialContext` (contracts/template-interface §1 / §2), and registry with `Register` / `Get` / `MustGet`; add `Check(q, account, format)` validation rules (§4); make T040 pass — FR-021
- [x] T045 [US4] Implement `internal/plugins/core/core.go` with `core` plugin satisfying `Plugin` interface; populate `data.Config.Timezone` via `time.LoadLocation`, `Animations` / `Display` / `Base64` / `debug.flags` parsing; zero-value-initialize `data.Computed`; make T041 pass; depends on T043 — FR-024
- [x] T046 [US4] Implement `internal/plugins/core/run_plugins.go` with `RunPlugins(ctx, pc, parallel int) error` using `errgroup.SetLimit`; recover panics into `*errors.RetryableError`; treat `parallel <= 0` as `runtime.GOMAXPROCS(0)`; record per-plugin results/errors into `pc.Data.Plugins[name]`; make T042 pass; depends on T043, T045 — FR-025, contracts/plugin-interface §4

**Checkpoint (US4)**: Plugin / Template の登録 + 取得が機能。`core.RunPlugins` が 3 種類 (success / error / panic) を集約しつつ並列実行可能。

---

## Phase 7: User Story 5 - エンドツーエンドでのデータ取得結線 (Priority: P3)

**Goal**: mocked GitHub から `data.User` / `data.User.ContributionsCollection` / `data.Computed.Repositories.*` を populate し、`engine.Compute` が internal `data` 構造体を返す。M1 段階では SVG/JSON 出力は対象外。

**Independent Test**: mocked GraphQL/REST + base+core + 1 つの no-op test plugin で `engine.Compute(ctx, Request{Login:"octocat", Template:"noop"}, deps)` を実行し、戻り値 `data` を `tests/golden/foundation_data.json` と比較。

### Tests for User Story 5

- [x] T047 [P] [US5] Add `tests/fixtures/github/graphql/user.json` (octocat user payload), `tests/fixtures/github/graphql/user_x.json` (bulk fields), `tests/fixtures/github/graphql/repositories_page{1,2,3}.json` (250 repos across 3 pages), `tests/fixtures/github/rest/rate_limit.json` (sample rate response) — supports US5 AS1 / AS2
- [x] T048 [P] [US5] Add `tests/fixtures/github/graphql/organization.json` (org account payload) and field-level fallback fixtures (`packages.json`, `sponsorships_as_sponsor.json`, `sponsorships_as_maintainer.json`, `members_with_role.json`) — supports US5 AS3
- [ ] T049 [P] [US5] Add `tests/fixtures/github/graphql/user_x_partial_failure.json` (bulk query with 502 errors on `packages` field) — supports US5 AS4
- [ ] T050 [P] [US5] Add `internal/plugins/base/base_test.go` covering user account dispatch + bulk-then-fallback + organization fallback — US5 AS3 / AS4
- [ ] T051 [P] [US5] Add `internal/plugins/base/repositories_test.go` covering 3-page pagination + batch-halving retry on timeout — US5 AS2
- [x] T052 [P] [US5] Add `tests/integration/foundation_test.go` invoking `engine.Compute` with mocked backend; assert `Result.Data.User.Login == "octocat"`, `Result.Data.Computed.Repositories.Count == 250`, `Result.Errors == nil`; compare full `Data` against `tests/golden/foundation_data.json` (XML normalize unused for JSON, just `assert.JSONEq`); add `BenchmarkCompute_Foundation_Mocked` ensuring < 2s per run — US5 AS1, SC-006
- [x] T053 [P] [US5] Add `tests/integration/foundation_die_test.go` invoking `engine.Compute` with `Request{Die: true}` where one plugin returns error; assert immediate return with the error and no subsequent plugin execution — US5 AS5

### Implementation for User Story 5

- [x] T054 [US5] Implement `internal/plugins/base/base.go` with `base` plugin satisfying `Plugin` interface; dispatch by `data.Account`: user → `RunUser`, organization → `RunOrganization`; bulk query `UserX` then field-level fallback on 502/"Something went wrong" errors (contracts/plugin-interface §5); make T050 pass; depends on T036 (`graphql.User` / `graphql.UserX`), T043, T038 (testhelper) — FR-022
- [x] T055 [US5] Implement `internal/plugins/base/organization.go` with field-level fallback for `packages`, `sponsorshipsAsSponsor`, `sponsorshipsAsMaintainer`, `membersWithRole`; merge into `data.User`; depends on T054 — FR-022
- [x] T056 [US5] Implement `internal/plugins/base/repositories.go` with `pageInfo.hasNextPage` loop bounded by `settings.repositories`; on timeout, halve batch size and retry; apply `repositories_forks` / `repositories_affiliations` / `repositories_skipped` filters; aggregate `data.Computed.Repositories.{Count, Stargazers, Forks, Releases, Watchers, Issues, PullRequests, Languages}`; make T051 pass; depends on T054 — FR-023
- [x] T057 [US5] Implement `internal/engine/engine.go` with `Compute(ctx, req, deps) (Result, error)`; orchestration: validate template (via `templates.MustGet`), determine `Convert`, build `Imports`, run `base` plugin, dispatch `core.RunPlugins`, wait for goroutines, aggregate errors; honor `die=true` for immediate return vs `die=false` for collected `Result.Errors`; M1 segment of `template.Run` is no-op (returns empty string); make T052 + T053 pass; depends on T044 (templates), T046 (core.RunPlugins), T054〜T056 (base) — FR-026

**Checkpoint (US5)**: `engine.Compute` が mocked backend に対し internal `data` 構造体を 2 秒以内に組み立てる。出力 (SVG/JSON) は未実装だが、後続 M2 タスクが消費する入力契約は確定。

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Foundation 全体の品質ゲート再確認、互換性差分検査、運用ドキュメント整備。

- [ ] T058 [P] Implement `make check-compat` target as a Go program `internal/tools/check-compat/main.go` comparing keys (top-level + nested `inputs.*.{type,default,format,values,global,preset,extras}`) between `./org_repo/source/plugins/<adopted>/metadata.yml` and `assets/plugins/<adopted>/metadata.yml`; exit 0 on zero diff, 1 otherwise; wire into CI as a separate job — SC-002, constitution 原則 I
- [ ] T059 [P] Run `make test-race` in CI workflow as a required job and ensure `internal/githubapi/rate.go` (FR-016) and `internal/plugins/core/run_plugins.go` (FR-025) report no data races over 100 iterations — research R-012, SC-007
- [ ] T060 [P] Verify `golangci-lint run --timeout=10m` and `govulncheck ./...` produce 0 findings; if any findings exist, fix them (no `//nolint` directives without `// reason:` comment); update `.golangci.yml` to reflect any rule disables added — SC-007
- [ ] T061 [P] Add `README.md` at repo root with: project status badge ("M1 in progress"), short description (Go port of `lowlighter/metrics` for adopted feature subset), link to `specs/001-project-foundation/`, link to `docs/design/` design corpus, MIT license badge — no constitution `// removed comments` and no marketing text
- [ ] T062 Run [quickstart.md](./quickstart.md) end-to-end on a clean checkout: walk all 9 sections, confirm every step succeeds and no troubleshooting notes are needed; record pass status in the PR that closes this feature
- [ ] T063 Verify constitution compliance with grep checks: `git log -- org_repo/` empty, no occurrences of unadopted plugin names (`wakatime`, `pagespeed`, `posts`, `rss`, `stackoverflow`, `leetcode`, `anilist`, `music`, `steam`, `tweets`, `crypto`, `nightscout`, `stock`, `chess`, `splatoon`, `fortune`, `poopmap`, `screenshot`, `16personalities`, `lines`, `gists`, `followup`, `discussions`, `code`, `introduction`, `skyline`, `support`) in `internal/plugins/`, no `// removed:` comments in code — SC-008, constitution 原則 III + V

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 依存なし、即着手可
- **Foundational (Phase 2)**: Setup 完了に依存、全 user story を block
- **User Stories (Phase 3+)**: Foundational 完了後、可能ならば並列着手
  - US1 (P1) は Foundational の完了確認も兼ねるため最優先
  - US2 (P1) は US1 と並列可
  - US3 (P2) は US1 / US2 とほぼ独立 (httpx 経由のみ Foundational に依存)
  - US4 (P2) は US3 とは独立、ただし US3 と並列着手で OK
  - US5 (P3) は US2 (metadata), US3 (GitHub API), US4 (registry) すべてに依存 → 最後に着手
- **Polish (Phase 8)**: 全 user story 完了後

### User Story Dependencies

```
Setup → Foundational → ┬─→ US1 (validation of foundation)
                       ├─→ US2 (config layer)
                       ├─→ US3 (GitHub API) ─┐
                       └─→ US4 (registry) ───┤
                                              ▼
                                             US5 (engine wiring, depends on US2/US3/US4)
                                              │
                                              ▼
                                            Polish
```

### Within Each User Story

- Tests (T014/T015, T018/T019, T026〜T030, T039〜T042, T047〜T053) はテーブルテスト書き出し → 実装の順 (TDD)
- Models / packages 単独実装 → service-like wiring → integration
- US 完了は **Checkpoint** で明示

### Parallel Opportunities

- Phase 1: T002, T003, T005, T006, T007 が [P]
- Phase 2: T009, T010, T011, T012 すべてが [P] (異なる internal package)
- Phase 3 (US1): T014, T015 が [P]
- Phase 4 (US2): T018, T019, T020 が [P]
- Phase 5 (US3): T026, T027, T028, T029, T030 すべてが [P]; T031, T032 が [P]
- Phase 6 (US4): T039, T040, T041, T042 が [P]; T044 が [P]
- Phase 7 (US5): T047, T048, T049, T050, T051, T052, T053 が [P]
- Phase 8: T058, T059, T060, T061 すべてが [P]

---

## Parallel Example: User Story 3 (GitHub API + Rate)

```bash
# Phase 5 のテストを並列にキック (5 並列):
Task: "Table tests for httpx.Client in internal/httpx/client_test.go" (T026)
Task: "Table tests for ClassifyToken in internal/githubapi/auth_test.go" (T027)
Task: "Rate tracker concurrency tests in internal/githubapi/rate_test.go" (T028)
Task: "GraphQL decode tests in internal/githubapi/graphql_test.go" (T029)
Task: "MOCKED_TOKEN panic guard test in internal/githubapi/mocked_guard_test.go" (T030)

# 実装は T031 / T032 を並列、その後 T033 → T034 と T035 → T036 を直列、T037 / T038 を最後に並列。
```

---

## Implementation Strategy

### MVP First (US1 のみ)

1. Phase 1 (Setup) を完走
2. Phase 2 (Foundational) を完走 (CRITICAL - 全ストーリー block 解除)
3. Phase 3 (US1) を完走
4. **STOP and VALIDATE**: US1 単体で「Buildable shell」が機能することを確認 (Quickstart §1)
5. このコミット時点で `release/v0.0.1-foundation-shell` を作成可能

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → Buildable shell (MVP!)
3. US2 → 互換 config layer 完成 → `make check-compat` 緑
4. US3 → GitHub API レイヤ完成 → mocked engine round-trip 可能
5. US4 → Plugin/Template registry 完成 → 後続 M4 (21 plugin) が着手可能
6. US5 → End-to-end wiring 完成 → M2 (template render) への入力契約確定
7. Polish → constitution compliance 確認 → `release/v0.1.0`

### Parallel Team Strategy

複数開発者で並列着手する場合:

1. 1 名で Setup + Foundational + US1 を直列に進める (約 1 週間)
2. Foundational 完了後:
   - Dev A: US2 (config)
   - Dev B: US3 (GitHub API)
   - Dev C: US4 (registry)
3. US2/US3/US4 完了後、いずれか 1 名が US5 (engine wiring) を担当
4. 全 US 完了後、全員で Polish を分担

---

## Notes

- 各 task は 1 PR に収まる粒度 (約 100〜400 LOC + tests) を目安とする。
- [P] task = 別ファイル、互いに未完了依存なし。
- [Story] label は traceability 用; constitution 原則 III に違反する task は MUST NOT 追加。
- テストは実装より先 (TDD): まず `_test.go` を書いて fail を確認、その後 production code を書いて green にする。
- 各 task 完了後にコミット (Conventional Commits、specific message)。
- 各 Checkpoint で story を独立に validate してから次の story に進む。
- 採用範囲外プラグイン (`docs/design/15-selection-answer.md` §7) の登録 init を追加することは MUST NOT。違反は constitution 原則 III の重大違反として PR を reject。
- `./org_repo` のファイルを copy & paste で持ち込むことは MUST NOT。`scripts/sync-assets.sh` 経由でのみ取得する。
- 本 feature の完了基準: T001..T063 すべて green + Quickstart 9 sections pass + constitution principle I-V すべて compliant。
