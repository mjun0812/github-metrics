---

description: "Task list for output-classic-json (M2: T-023..T-029)"
---

# Tasks: classic テンプレート + JSON 出力 (M2)

**Input**: Design documents from `/specs/002-output-classic-json/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: 必須 (constitution 原則 IV + FR-016/017/018)。各実装タスクの直後にテーブルテスト + golden file を書くこと。

**Organization**: 6 フェーズ — Setup / Foundational / US1 / US2 / US3 / Polish。各 user story は独立にテスト可能。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 並列実行可能 (別ファイル、未完了依存なし)
- **[Story]**: US1〜US3 のうちどのストーリーに属するか
- 各タスク行に対象ファイルの相対パスを明記

## Path Conventions

- 単一 Go プロジェクトレイアウト: `internal/<pkg>/`, `assets/`, `tests/`
- 詳細は [plan.md §Project Structure](./plan.md) を参照

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 上流 fixture 取得ツールと、テストヘルパの足場を整える。後続フェーズの全タスクが依存する。

- [X] T001 Implement `internal/tools/sync-fixtures/main.go` — reads `./org_repo/tests/cases/octocat.yml`, runs the upstream Node implementation under `./org_repo` to materialize the output, and copies the resulting JSON to `tests/fixtures/upstream/<login>.json` (research R-004). Idempotent and network-free against the local upstream checkout.
- [X] T002 [P] Wire `make sync-fixtures` target in `Makefile` invoking `go run ./internal/tools/sync-fixtures --user octocat` and a `make check-output-compat` target that runs the new `tests/compatibility/...` suite.
- [X] T003 [P] Add `tests/integration/svg_normalize.go` providing `NormalizeSVG(raw []byte) ([]byte, error)`: parse via `encoding/xml`, lex-sort attributes, collapse text-node whitespace, drop comments, regex-mask the footer's `Last updated …` / `lowlighter|mjun0812/github-metrics@…` segments to `__MASKED__` (research R-009). The helper is shared by every classic golden test.
- [X] T004 [P] Add `internal/engine/version.go` exposing `Version() string` (default returns the value of `version` linker variable, falls back to `"dev"`) and `SetVersionForTest(t TB, v string)` that swaps it through `t.Cleanup` (research R-010). Update `cmd/metrics-action/main.go` and `cmd/metrics-cli/main.go` to read this single source.

**Checkpoint (Phase 1)**: `make sync-fixtures` produces `tests/fixtures/upstream/octocat.json`; `NormalizeSVG` is callable from tests; `engine.Version()` is the single source of truth for the version string.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `engine.Result` 拡張 + format ディスパッチ shell。これが無いと US1/US2/US3 のいずれも `Result.Output` を埋められない。

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Extend `engine.Result` in `internal/engine/engine.go` with `Output []byte` and `MIME string` fields (data-model E-001). Update doc comments to describe the IANA values defined in contracts/result-dispatch.md §2.
- [X] T006 Add `dispatchOutput(ctx, req, deps, tmpl, data, pcPartial)` helper in `internal/engine/engine.go` that follows the pseudocode in contracts/result-dispatch.md §3. Wire it as the new Stage 4 of `engine.Compute`, populating `Result.Output` / `Result.MIME`. PNG/JPEG branches log a warn via `deps.Logger` (research R-008). Add table tests in `internal/engine/engine_test.go` covering format default resolution (FR-014), `Format=""` + Template=nil falling back to `"json"`, and `Format="bogus"` returning `*errors.UnsupportedFormatError`.

**Checkpoint (Phase 2)**: `engine.Compute` returns a `Result` with `Output` and `MIME` set per `Request.Format`; existing M1 tests still green; new dispatch tests green.

---

## Phase 3: User Story 1 — JSON 出力で full data structure を取得 (Priority: P1) 🎯 MVP

**Goal**: `Format: "json"` で呼んだ `engine.Compute` が、上流互換キー集合の JSON を `Result.Output` に格納し、循環参照を `"[Circular]"` で安全に潰す。

**Independent Test**: mocked GraphQL backend で `engine.Compute(Request{Login:"octocat", Format:"json"})` を呼び、`Result.Output` を `tests/golden/json/octocat.json` と `assert.JSONEq` 比較。`tests/compatibility/json_test.go` で上流 fixture とのキー集合差分ゼロを assert。SVG パスは一切経由しない。

### Tests for User Story 1

- [X] T007 [P] [US1] Add `internal/engine/json_test.go` with table tests covering: simple Data marshalling, `time.Time` → RFC 3339 string, `map[int]string` → `[{key,value}]` sorted by key, `map[T]struct{}` set → sorted `[T]`, `config.Token` masked to `"(provided)"` (data-model E-002 normalization table).
- [X] T008 [P] [US1] Add `internal/engine/json_cycle_test.go`: build a `Data` with `Plugins["self-ref"]` pointing back into itself, plus `Plugins["a-b"]` / `Plugins["b-a"]` where two values reference each other. Assert `Marshal` returns a `[]byte` containing `"[Circular]"` and **does not panic** (FR-002, contracts/json-output.md §5).
- [X] T009 [P] [US1] Add `tests/golden/json/octocat.json` placeholder (empty `{}` for now) and `tests/integration/output_json_test.go::TestComputeJSON_OctocatGolden` that calls `engine.Compute` against the mocked GraphQL fixture and `assert.JSONEq` against the golden. The golden gets populated by T013 below via `-update`.
- [ ] T010 [P] [US1] Add `tests/compatibility/json_test.go::TestUpstreamKeysetCompatibility` reading `tests/fixtures/upstream/octocat.json` and the same test's locally-produced JSON; assert the upstream key set is a subset of the local key set (FR-018, SC-001). Use a recursive walker that collects every JSON-pointer-style path so nested keys (`computed.repositories.count`) are compared too. If `tests/fixtures/upstream/octocat.json` does not exist, the test calls `t.Skip` so contributors without `./org_repo` can still pass CI.

### Implementation for User Story 1

- [X] T011 [US1] Implement `internal/engine/json.go` (data-model E-002): `Marshal(*plugins.Data) ([]byte, error)`, `cycleDetector` with `seen map[uintptr]struct{}`, `normalize(any) any` that handles the value-table from contracts/json-output.md §3. Treat `reflect.Value.Pointer()` as the cycle key for `reflect.Ptr` / `reflect.Map` / `reflect.Slice`; unaddressable values short-circuit to `"[Circular]"`. Time, Token, error special cases routed through dedicated branches. Make T007 + T008 pass.
- [X] T012 [US1] Wire `dispatchOutput` (T006) to call `Marshal` when `format == "json"`, set `MIME = "application/json"`. Verify `engine.Compute(Request{Format: "json"})` returns `len(Result.Output) > 0`, `json.Valid(Result.Output)` is true, `Result.MIME == "application/json"`.
- [X] T013 [US1] Regenerate `tests/golden/json/octocat.json` via `go test ./tests/integration/... -run TestComputeJSON_OctocatGolden -update`. Inspect the diff and commit. Run the full `go test ./...` once more to confirm stability.

**Checkpoint (US1)**: JSON output complete with cycle protection and upstream compatibility verified. This is the MVP — could ship to a downstream-tool consumer today.

---

## Phase 4: User Story 2 — classic テンプレートで SVG ベース DOM を生成 (Priority: P1)

**Goal**: `Format: "svg"`, `Template: "classic"` で呼んだ `engine.Compute` が、4 partial と optional metadata footer を含む SVG を生成し、`Result.Output` / `Result.MIME` を populate する。XML 正規化後 MD5 で golden 一致。

**Independent Test**: mocked GraphQL backend + classic registered で `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼び、`tests/integration/svg_normalize.NormalizeSVG` を通してから `tests/golden/classic/octocat.svg` と MD5 比較。

### Tests for User Story 2

- [X] T014 [P] [US2] Add `internal/templates/classic/escape_test.go` covering `EscapeXML`: angle brackets / ampersand / quotes / apostrophe map to `&lt;`/`&gt;`/`&amp;`/`&#34;`/`&#39;` (or numeric refs per contracts/classic-template.md §3); idempotent on already-escaped strings; empty string passes through.
- [X] T015 [P] [US2] Add `internal/templates/classic/partials/partials_test.go` with table cases per partial (data-model E-004): `BaseHeader` with `Data.User == nil` returns `""`; with login + name populated returns a fragment containing `data-section="header"` and escaped name; `Introduction` always returns `""` when `Data.Plugins["introduction"] == nil`; `BaseActivityCommunity` outputs the outer `<section data-section="activity-community">` even when sub-fields are nil; `BaseRepositories` with `Computed.Repositories.Count == 0` returns `""`; with Count=250 returns fragment containing the `format.Format` short form `"250"`.
- [X] T016 [P] [US2] Add `internal/templates/classic/classic_test.go::TestClassic_Check_*` covering: `account == "user"` + `format == "svg"` passes; `account == "repository"` returns `*errors.InputError{Field:"account"}`; `format == "pdf"` returns `*errors.UnsupportedFormatError`; empty `format` passes (engine handles default resolution).
- [X] T017 [P] [US2] Add `tests/golden/classic/octocat.svg` placeholder (an empty `<svg/>` for now) and `tests/integration/output_svg_test.go::TestComputeSVG_ClassicOctocatGolden`: builds the same mocked-GraphQL deps from US1, calls `engine.Compute(Request{Template:"classic", Format:"svg"})`, normalizes both sides via `svg_normalize.NormalizeSVG`, MD5-compares (contracts/classic-template.md §5).

### Implementation for User Story 2

- [X] T018 [P] [US2] Add `internal/templates/classic/escape.go` with `EscapeXML(s string) string` and `FormatCount(n int64) string` (delegates to `internal/format.Format`). Make T014 pass.
- [X] T019 [P] [US2] Add `internal/templates/classic/partials/base_header.go` implementing `BaseHeader` per contracts/classic-template.md §2.2. Emits `<section data-section="header">` with `<h1>` containing avatar `<img>` (when `Data.User.AvatarURL != ""`) and login/name span. Empty when `Data.User == nil`. Make T015's BaseHeader cases pass.
- [X] T020 [P] [US2] Add `internal/templates/classic/partials/introduction.go` implementing `Introduction` per contracts/classic-template.md §2.3 — stub that returns `""` until M4 lands an introduction plugin. Make T015's Introduction cases pass.
- [X] T021 [P] [US2] Add `internal/templates/classic/partials/base_activity_community.go` implementing `BaseActivityCommunity` per contracts/classic-template.md §2.4 — emits the outer two-section row scaffolding so M4 plugins can hydrate it later. Make T015's BaseActivityCommunity cases pass.
- [X] T022 [P] [US2] Add `internal/templates/classic/partials/base_repositories.go` implementing `BaseRepositories` per contracts/classic-template.md §2.5 — `<section data-section="repositories">` with three `<div class="field">` lines for count / stargazers / forks using `FormatCount`. Make T015's BaseRepositories cases pass.
- [X] T023 [US2] Overwrite `assets/templates/classic/partials/_.json` to the MVP 4-element list (`["base.header","introduction","base.activity+community","base.repositories"]`) and preserve the original as `assets/templates/classic/partials/_.json.upstream` (research R-007). Document the swap in a one-line comment if the JSON format allows; otherwise a sibling `_.json.README.md`.
- [X] T024 [US2] Add `internal/templates/classic/classic.go` (data-model E-003): `classicTemplate` struct embeds `assets/templates/classic` via `//go:embed`, parses `metadata.yml` once during `init()`, reads `partials/_.json` to determine partial order, registers the four named partials via a local lookup table, and exposes `Name`, `Metadata`, `FS`, `Check`, `Run`. `Run` follows the 8-step pipeline in data-model E-003 (load skeleton → defs → styles → foreignObject → partials → optional metadata footer → close). Make T016 + T017 pass after T025 regenerates the golden.
- [X] T025 [US2] Regenerate `tests/golden/classic/octocat.svg` via `go test ./tests/integration/... -run TestComputeSVG_ClassicOctocatGolden -update`. Inspect the diff carefully — confirm the DOM matches the upstream classic shape documented in contracts/classic-template.md §1, then commit.

**Checkpoint (US2)**: classic SVG path complete. Both major output formats now work end-to-end through `engine.Compute`.

---

## Phase 5: User Story 3 — Result.Output が呼び出し側に届く (Priority: P2)

**Goal**: `cmd/metrics-action` と `cmd/metrics-cli` の将来 (M6) 呼び出し側が、`engine.Compute` の戻り値 1 つを byte slice + MIME としてそのまま流せる。本 spec 範囲では engine 側の配線 + integration テストで完結する (cmd 側のファイル書き出しは M6)。

**Independent Test**: 同じ mocked GraphQL deps を再利用し、`Format` を `""` / `"json"` / `"svg"` / `"png"` / `"jpeg"` / `"bogus"` で順に呼んで `Result.MIME` と `Result.Output` の先頭 bytes を assert。

### Tests for User Story 3

- [X] T026 [P] [US3] Add `tests/integration/output_test.go::TestComputeJSON_DefaultFromTemplate`: when classic is registered and `Format == ""`, `Result.MIME == "application/json"` (because classic's `metadata.yml.formats[0]` is `"svg"`, but for this case we omit Template to test the noop fallback → "json"). Actually, re-check contracts/result-dispatch.md §4 for the truth table and write tests for all four rows.
- [X] T027 [P] [US3] Add `TestComputeSVG_Classic` asserting `Result.Output` begins with `<svg` and `Result.MIME == "image/svg+xml"`.
- [X] T028 [P] [US3] Add `TestComputePNG_M2WarnsAndReturnsSVG`: capture log output via a `bytes.Buffer`-backed slog handler, call `engine.Compute(Request{Format:"png", Template:"classic"})`, assert `Result.MIME == "image/png"`, `Result.Output` begins with `<svg`, and the captured log contains `format=png` at `warn` level (research R-008).
- [X] T029 [P] [US3] Add `TestComputeUnknownFormat_Error` asserting `Format == "bogus"` returns a non-nil error that satisfies `errors.As(&xerrors.UnsupportedFormatError{})`.
- [X] T030 [P] [US3] Add `TestComputeSVG_NoTemplate_Errors` asserting `Format == "svg"` with no Template registered returns a non-nil `*errors.InputError{Field:"template"}`.

### Implementation for User Story 3

- [X] T031 [US3] Tighten the `dispatchOutput` implementation from T006 to match every row of the contracts/result-dispatch.md §4 truth table; ensure the warn log shape matches T028's expectation. Make T026–T030 pass.
- [X] T032 [US3] Update [`specs/002-output-classic-json/quickstart.md`](./quickstart.md) §4 step block to reflect the final test names (T026–T030) and adjust any inaccurate language. Verify each `go test ./...` command listed there returns green on a clean checkout.

**Checkpoint (US3)**: format dispatch verified for every documented row; engine surface stable enough for cmd-side wiring in M6.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Bench, README, constitution compliance evidence.

- [X] T033 [P] Add `BenchmarkCompute_JSON_Octocat` and `BenchmarkCompute_SVG_Classic` in `tests/integration/bench_test.go`, each running `engine.Compute` once per iteration against the mocked deps. Confirm `< 2s/op` on the contributor laptop (SC-003).
- [X] T034 [P] Update `README.md` "Status" line and Toolchain matrix entry for the new test directories. Add a one-paragraph note about `make sync-fixtures` to the contributor quickstart.
- [X] T035 [P] Update `tests/compliance/compliance_test.go` (M1) to also scan `internal/engine/` and `internal/templates/classic/` for the unadopted-plugin grep (constitution principle III evidence). Run the suite and confirm zero hits.
- [X] T036 [P] Verify `make check-compat` still reports `0 diff across 21 plugins and 2 templates` after the new `classic/_.json` overwrite. The `_.json` swap only changes the partial list (not metadata.yml keys) so the report should stay zero; capture the output in the PR body.
- [X] T037 Run [quickstart.md](./quickstart.md) end-to-end on a clean checkout, walking each numbered step. Capture pass status in the PR description; flag any step that requires `./org_repo` and document the skip behavior.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 依存なし、即着手可
- **Foundational (Phase 2)**: Setup 完了に依存。全 user story を block
- **User Stories (Phase 3-5)**: Foundational 完了後、可能ならば並列着手
  - US1 (P1 JSON) と US2 (P1 classic SVG) は独立、並列可能
  - US3 (P2 dispatch) は US1 + US2 の実装出力に依存 (実 Marshal と Template が無いと dispatch のテストが固められない) → US1/US2 完了後
- **Polish (Phase 6)**: 全 user story 完了後

### User Story Dependencies

```
Setup → Foundational → ┬─→ US1 (JSON)    ─┐
                       └─→ US2 (classic) ─┼─→ US3 (dispatch) → Polish
                                          │
                                          └─→ (US1 / US2 が並列で良い)
```

### Within Each User Story

- Tests (table tests + golden placeholder) を先に書く (TDD)
- 実装 → golden 生成 (`-update`) → 緑化
- Story 完了は **Checkpoint** で明示

### Parallel Opportunities

- Phase 1: T002, T003, T004 が [P]
- Phase 3 (US1): T007〜T010 が [P] (5 つのテスト準備が並列)
- Phase 4 (US2): T014〜T017 (4 つのテスト準備) + T018〜T022 (4 つの partial 実装) が [P]
- Phase 5 (US3): T026〜T030 (5 つのテスト) が [P]
- Phase 6 (Polish): T033〜T036 が [P]

---

## Parallel Example: User Story 2 (classic SVG)

```bash
# Phase 4 のテスト準備を並列にキック (4 並列):
Task: "EscapeXML table tests in internal/templates/classic/escape_test.go" (T014)
Task: "Partial-per-file table tests in internal/templates/classic/partials/partials_test.go" (T015)
Task: "Template.Check coverage in internal/templates/classic/classic_test.go" (T016)
Task: "engine.Compute golden test scaffold in tests/integration/output_svg_test.go" (T017)

# 実装も並列にキック (5 並列):
Task: "EscapeXML + FormatCount in internal/templates/classic/escape.go" (T018)
Task: "BaseHeader partial" (T019)
Task: "Introduction stub" (T020)
Task: "BaseActivityCommunity scaffold" (T021)
Task: "BaseRepositories partial" (T022)
```

T023 (`_.json` 上書き) と T024 (`classic.Template`) は順序依存 (T024 が `_.json` を読むため)、T025 (golden 生成) は最後。

---

## Implementation Strategy

### MVP First (US1 のみ)

1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1 JSON) を完走
2. **STOP and VALIDATE**: `Format="json"` の output が安定するか CI で確認
3. このコミット時点で `release/v0.1.0-json-only` を出すことも可能 (下流の JSON 消費者には十分)

### Incremental Delivery

1. Setup + Foundational → 基盤
2. **US1 (JSON)** → MVP! 下流ツールに JSON を提供できる
3. **US2 (classic SVG)** → README バッジ運用が始められる
4. **US3 (dispatch)** → engine 表面の安定化、M6 への準備
5. **Polish** → 互換性・ベンチ・compliance grep 更新

### Parallel Team Strategy

複数開発者で並列着手する場合:

1. 1 名で Setup + Foundational を直列に進める (約 1 日)
2. Foundational 完了後:
   - Dev A: US1 (engine/json.go + tests)
   - Dev B: US2 (classic/ + partials + tests)
3. US1/US2 完了後、いずれか 1 名が US3 (dispatch tighten + 全パス test) を担当
4. 全 US 完了後、全員で Polish (4 タスク [P]) を分担

---

## Notes

- 各 task は 1 PR に収まる粒度 (約 100〜400 LOC + tests) を目安にする
- [P] task = 別ファイル、互いに未完了依存なし
- [Story] label は traceability 用; constitution 原則 III に違反する task (採用範囲外の partial / template / 出力形式) は MUST NOT 追加
- テストは実装より先 (TDD): まず `_test.go` を書き fail を確認、その後 production code を書いて green にする
- 各 task 完了後にコミット (Conventional Commits、specific message)
- `./org_repo` のファイルを copy & paste で持ち込むことは MUST NOT。assets sync は `scripts/sync-assets.sh` (M1) / `scripts/sync-fixtures.sh` (本 spec T001) 経由でのみ
- 本 feature の完了基準: T001..T037 すべて green + Quickstart 9 sections pass + CI 全 7 job 緑
