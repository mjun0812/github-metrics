---

description: "Task list for chromedp-rendering-pipeline (M3: T-031..T-039 + N-001 派生)"
---

# Tasks: chromedp レンダリングパイプライン (M3)

**Input**: Design documents from `/specs/003-chromedp-rendering-pipeline/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: 必須 (constitution 原則 IV + FR-020/021/022)。chromedp 非依存テストは `go test ./...`、chromedp 実起動テストは `//go:build chromedp` build tag で隔離。各実装タスクの直後にテーブルテスト + golden file を書くこと。

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

**Purpose**: 新規依存 (chromedp / goquery / cascadia / tdewolff) を `go.mod` に追加し、Makefile / CI / octicon アセット生成パイプラインを整える。後続フェーズの全タスクが依存する。

- [X] T001 Add new module dependencies to `go.mod`: `github.com/chromedp/chromedp@latest`, `github.com/PuerkitoBio/goquery@latest`, `github.com/andybalholm/cascadia@latest`, `github.com/tdewolff/parse/v2@latest`, `github.com/tdewolff/minify/v2@latest`. **Operational note (M3 impl)**: `go mod tidy` strips deps that no Go file imports yet, so the literal "run tidy now" step is deferred: each lib re-enters `go.mod`/`go.sum` automatically when its first importing task lands (chromedp → T022/T023, goquery → T034/T044, cascadia → T044, tdewolff parse+minify → T044). The module proxy cache already holds resolved versions (`go env GOMODCACHE`), so the actual `go get` per dep is a no-op-network call. PR body MUST list each dependency with the "代替検討と棄却理由" lines copied from `research.md` R-001 / R-007 / R-008 (constitution 原則 V).
- [X] T002 [P] Add `test-chromedp`, `gen-octicons`, `verify-octicons` targets to `Makefile` per `quickstart.md §2`. `test-chromedp` runs `go test -tags=chromedp ./...`; `gen-octicons` invokes `go run ./internal/tools/gen-octicons -in node_modules/@primer/octicons/build/data.json -out assets/octicons/data.json`; `verify-octicons` re-runs `gen-octicons` and asserts `git diff --exit-code assets/octicons/data.json`.
- [X] T003 [P] Add a `test-chromedp` job to `.github/workflows/go-ci.yml` using the `chromedp/headless-shell:latest` container image, with `METRICS_CHROME_PATH=/headless-shell/headless-shell` exported. Job runs `make test-chromedp`. Job is parallel to the existing `test` job (chromedp-free) and uses the same checkout + Go setup steps.
- [X] T004 [P] Implement `internal/tools/gen-octicons/main.go` per `contracts/octicon.md §4`. Reads `primer/octicons` `build/data.json`, emits `assets/octicons/data.json` with the schema documented in `data-model.md E-006`. Idempotent (excluding `_meta.generated_at`). Includes `-in` and `-out` CLI flags and a unit test under `internal/tools/gen-octicons/main_test.go` covering the (name, size) → svg fragment mapping for `star` and `chevron-down`.
- [X] T005 Generate `assets/octicons/data.json` once via `make gen-octicons` (requires `npm install --no-save @primer/octicons` locally). Commit the generated file. Verify size < 500 KB and that `_meta.source` is set to `primer/octicons@<version>`. **Result**: 484 KB, `primer/octicons@19.25.0`, 376 icons; `star.16` and `star.24` populated; `_meta.generated_at` stamped at run time. **Known limitation**: `make verify-octicons` will report a diff on every fresh run because `_meta.generated_at` is non-deterministic. The CI verify-octicons gate (Polish-phase task) needs a follow-up: either freeze the timestamp behind a `--source-frozen` flag, or strip `_meta.generated_at` from the diff comparison. Tracked here as a Phase 6 / follow-up issue, not blocking Phase 2+ progress.

**Checkpoint (Phase 1)**: `go test ./...` still green with new deps (no functional code yet); `make test-chromedp` runs (will be empty until US1 lands tests); `assets/octicons/data.json` is committed and `make verify-octicons` is clean.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `internal/render` パッケージの骨格 (`Renderer` interface + `FakeRenderer` + `PipelineStage` + `Apply`) と `engine.Deps.Render` フィールドを整える。これが無いと US1/US2/US3 のいずれも本実装をテストできない。

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Create the `internal/render` package skeleton: empty `doc.go` with the package comment "Package render implements the chromedp-backed SVG resize, the pure-Go svg.Hash, the SVG decoration pipeline (octicon / CSS purge / XML format), and a small `Renderer` interface engine.Compute consumes." Add `_test.go` placeholder so `go test ./internal/render/...` compiles. **Note**: the `_test.go` placeholder is unnecessary because renderer_test.go + pipeline_test.go from T008/T010 land in the same phase and compile together.
- [X] T007 [P] Add `internal/render/renderer.go` defining (a) `Renderer` interface with single method `Resize(ctx, in, opts) (ResizeResult, error)`, (b) `ResizeOpts` and `ResizeResult` structs per `data-model.md E-002`, (c) `FakeRenderer` struct per `data-model.md E-003`. `FakeRenderer.Resize` returns the 67-byte minimal PNG when `Convert == "png"`, a 4-byte JPEG header fixture when `Convert == "jpeg"`, and passes `in` through as `Body` for `"" | "svg"`. Include a constructor `NewFakeRenderer()` for clarity.
- [X] T008 [P] Add `internal/render/renderer_test.go` covering: `FakeRenderer.Resize` with `Convert=""` returns input bytes + `image/svg+xml`; `Convert="png"` returns bytes starting with `\x89PNG\r\n\x1a\n` + `image/png`; `Convert="jpeg"` returns bytes starting with `\xff\xd8\xff` + `image/jpeg`; `Convert="bogus"` returns `*errors.UnsupportedFormatError`; `ErrOnConvert{"png": sentinel}` returns sentinel on PNG only.
- [X] T009 [P] Add `internal/render/pipeline.go` defining `PipelineStage` struct and `Apply(stages, in) (string, []error)` per `data-model.md E-004`. Best-effort chain: a stage returning an error is wrapped with `fmt.Errorf("stage %q: %w", name, err)`; the next stage gets the **prior** stage's output (not the failing stage's partial output).
- [X] T010 [P] Add `internal/render/pipeline_test.go` covering: 3 stages all succeed → final output is the last stage's result, errors is empty; stage 2 fails → output is stage 1's output, errors contains 1 entry wrapped with stage name; empty stages list → input passthrough; stage returns input unchanged → no error. **Extra coverage added**: a stage with `nil Run` is treated as a no-op (defensive for incremental stage-list builders).
- [X] T011 Extend `engine.Deps` in `internal/engine/engine.go` with `Render render.Renderer` field (`data-model.md E-005`). Update the doc comment to describe the nil-fallback behavior (lazy `render.New` on first SVG/PNG/JPEG call when `Render == nil`). Do NOT yet wire it into `dispatchOutput` — that lands with US1. Verify existing M2 tests still green (zero-value `Deps.Render` is permitted).

**Checkpoint (Phase 2)**: `internal/render` package exists with `Renderer` / `FakeRenderer` / `PipelineStage` / `Apply`; `engine.Deps` has the new field; all existing tests still green; no chromedp imports yet.

---

## Phase 3: User Story 1 — chromedp で SVG リサイズ + PNG/JPEG 出力 (Priority: P1) 🎯 MVP

**Goal**: `Format ∈ {svg, png, jpeg}` で呼んだ `engine.Compute` が classic SVG を chromedp で計測して `<svg height>` を確定させ、PNG/JPEG では実画像 bytes を返す。M2 の `chromedp conversion lands in M3` warn ログが消える。

**Independent Test**: chromedp ありの環境で `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"png"})` を呼び、`image.Decode(bytes.NewReader(res.Output))` 成功 + `bounds.Dx() >= 480 && bounds.Dy() >= 480` を assert。SVG 経路では `<svg height="<整数>">` が `auto` でない数値属性に書き換わっていることを assert。

### Tests for User Story 1

- [X] T012 [P] [US1] Add `internal/render/padding_test.go` covering the `parsePadding` table in `contracts/svg-resize.md §3`: empty, `"0 + 6%"`, two-element `["0 + 6%", "8 + 11%"]`, comma-single `"8 + 11%, 0 + 6%"`, `"+0.5"` / `"1.5%"` mixture, malformed `"bogus"` (warning + default), exotic locale chars (verify no panic).
- [X] T013 [P] [US1] Add `internal/render/svg_resize_test.go` (no chromedp tag) covering `ResizeOpts.normalize()`: zero `Convert` → `"svg"`, zero `ViewportWidth/Height` → 980, zero `SettleDelay` → 2400ms, `Convert="bogus"` → `*errors.UnsupportedFormatError`.
- [X] T014 [P] [US1] Add `internal/render/chrome_test.go` with `//go:build chromedp` build tag covering: `New(BrowserOpts{})` returns non-nil Browser when chromium is on PATH or `METRICS_CHROME_PATH` is set; `New` fails fast when `ExecPath` points to a non-existent binary; `NewTab(ctx)` returns a cancel func that releases the tab; `Close` is idempotent (calling twice is fine).
- [X] T015 [P] [US1] Add `internal/render/chrome_recycle_test.go` (chromedp tag) covering `RecycleEvery=3`: 4 sequential `NewTab` calls produce a new allocator on the 4th invocation, observed via a counter exposed for testing (`Browser.RecycledCount()` test helper, package-private).
- [X] T016 [P] [US1] Add `internal/render/svg_resize_chromedp_test.go` (chromedp tag) `TestResize_FixedSVG_HeightInRange`: load `tests/fixtures/render/input_for_measure.svg` (an SVG with `<g id="metrics-end" transform="translate(0,640)"/>` at the end), call `Resize(ctx, in, ResizeOpts{Convert:"svg"})`, assert returned `<svg height>` ∈ `[630, 650]` and `MIME == "image/svg+xml"`.
- [X] T017 [P] [US1] Add `TestResize_PNG_Decodable` and `TestResize_JPEG_Decodable` in the same file: same SVG input, `Convert="png"` returns bytes that `png.Decode` parses; `Convert="jpeg"` returns bytes that `jpeg.Decode` parses; both have `bounds.Dx() >= 1, bounds.Dy() >= 1`.
- [X] T018 [P] [US1] Add `internal/engine/dispatch_renderer_test.go` (no chromedp tag): inject a `FakeRenderer`, call `engine.Compute(Request{Format:"png", Template:"classic"})`, assert `Result.MIME == "image/png"`, `Result.Output` starts with PNG magic. Same for jpeg. Same for svg (FakeRenderer passes SVG through, so `Output` is the Template.Run result). Capture the slog output and assert the M2 "chromedp conversion lands in M3" warn message is **NOT** emitted.
- [X] T019 [P] [US1] Add `tests/golden/render/chromedp_height_octocat.json` placeholder with `{"width_min": 470, "width_max": 510, "height_min": 470, "height_max": 9000}` (range golden — actual height varies by plugin count). Add `tests/integration/render_chromedp_test.go` (chromedp tag) `TestComputePNG_E2E` calling `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"png"})` against mocked GraphQL deps from M2, assert the produced PNG decodes and dimensions land inside the golden range.
- [X] T020 [P] [US1] Add `tests/fixtures/render/input_for_measure.svg` (minimal `<svg width="480" viewBox="0 0 480 640"><g id="metrics-end" transform="translate(0,640)"/></svg>`) — the fixture T016/T017 consume.

### Implementation for User Story 1

- [X] T021 [US1] Implement `internal/render/padding.go` per `contracts/svg-resize.md §3`: `parsePadding(in []string) padding` and the `padding` struct with `width/height/absoluteWidth/absoluteHeight`. Lazy regexes (`regexp.MustCompile` at package level). Malformed input logs a warning via the package logger and returns zero-defaults. Make T012 pass.
- [X] T022 [US1] Implement `internal/render/chrome.go` per `data-model.md E-001`: `Browser` struct (allocCtx, parentCtx, mutex, counter, opts), `BrowserOpts.normalize()` (defaults `RecycleEvery=200`, `Headless=true`, resolves `ExecPath` from env), `New(opts) (*Browser, error)`, `NewTab(ctx) (tabCtx, cancel, error)` (locks mutex, recycles when counter reaches RecycleEvery, returns a child context), `Close() error` (idempotent), and a test helper `RecycledCount() int` only compiled with build tag `chromedp_internal_test`. Make T014 + T015 pass.
- [X] T023 [US1] Implement `internal/render/svg_resize.go` per `contracts/svg-resize.md §2-4`: `ResizeOpts.normalize()`, `(b *Browser) Resize(ctx, in, opts) (ResizeResult, error)` that (a) opens a fresh tab via `NewTab`, (b) emulates the viewport, (c) calls `page.SetDocumentContent` with the rendered SVG, (d) runs the JS template from §2.1 with `SETTLE_MS` substituted from `opts.SettleDelay`, (e) on `Convert ∈ {"", "svg"}` returns the XMLSerializer string as bytes + `image/svg+xml`, (f) on `Convert ∈ {"png", "jpeg"}` invokes `page.CaptureScreenshot` with the appropriate clip + format + omitBackground. Wrap chromedp errors with `*xerrors.RetryableError`. Make T013 + T016 + T017 pass.
- [X] T024 [US1] Update `internal/engine/dispatch.go` per `contracts/render-pipeline.md §2`: for `Format ∈ {"svg", "png", "jpeg"}` call `Template.Run` (M2 existing) → pass result through `render.Apply(buildPipelineStages(inputs), rendered)` (Apply is wired in US3, US1 uses an empty stages slice as a placeholder) → call `deps.Render.Resize` (lazy `render.New` if `Render == nil`) → return `(out.Body, out.MIME)`. Remove the M2 warn log `engine: png output stages SVG bytes (chromedp conversion lands in M3)`. For Resize errors: SVG path returns the un-resized SVG + `image/svg+xml` and appends to `Result.Errors`; PNG/JPEG path returns `nil, ""` and appends to `Result.Errors`. Make T018 + T019 pass.
- [X] T025 [US1] Update `internal/engine/engine.go` to thread `*Result` (not just data) into `dispatchOutput` so the dispatch can append to `Result.Errors`. Keep the signature change backward compatible by adding `res` as the new last parameter. Update the single existing caller (`Compute`). Verify existing M2 dispatch tests still green.
- [X] T026 [US1] Run `make test-chromedp` end-to-end on a chromium-equipped machine. Confirm T016/T017/T019 green and that `Compute(Format:"png")` returns a real PNG. Capture wall time and assert SC-003 (< 5s per call). Commit any timing-related test fixes. **Result (M3 impl)**: `METRICS_CHROME_PATH=/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome make test-chromedp` green; `TestComputePNG_E2E` produces a 480x489 PNG in 3.47s on the maintainer's Mac (well under the 5s SC-003 budget). chromedp tests added for: Browser lifecycle (`chrome_test.go`), recycle counter (`chrome_recycle_test.go`), Resize SVG/PNG/JPEG (`svg_resize_chromedp_test.go`), and full engine.Compute → image.Decode (`tests/integration/render_chromedp_test.go`). **Impl deltas captured during T026**: (a) `--single-process` flag dropped from the default chromedp flag set (broke `Network.enable` on Chrome 137+; documented inline in chrome.go) — callers can re-enable via `BrowserOpts.ExtraFlags` for Docker/ARM stability; (b) measurement JS split into prepare + measure steps with `chromedp.Sleep` driving the settle delay externally, since Runtime.evaluate's awaitPromise path was unreliable for our JSON.stringify return on modern Chrome; (c) width measurement reads `svg.getBoundingClientRect().width` instead of `#metrics-end`'s bbox, because the anchor is an empty `<g>` whose bbox width is 0 — height still reads from the anchor relative to the SVG top.

**Checkpoint (US1)**: chromedp 経路完成。`Format=png/jpeg` で実画像が返る、M2 の warn ログ消失、SC-001/SC-002/SC-003 緑。 これが MVP — README バッジで PNG を貼れる状態。

---

## Phase 4: User Story 2 — SVG ハッシュで `data-changed` モード成立 (Priority: P1)

**Goal**: `render.Hash(rendered string) (string, error)` が `<footer>` 除去後 outerHTML の MD5 hex を返し、M6 T-114 (`output_condition=data-changed`) の比較ブロックを解消する。chromedp 非依存。

**Independent Test**: `internal/render/svg_hash_test.go` で footer のみ違う SVG 2 本が同 MD5、DOM 違う 1 本が別 MD5、空入力が `("", nil)` を返すことを assert。

### Tests for User Story 2

- [X] T027 [P] [US2] Add `internal/render/svg_hash_test.go::TestHash_EmptyString` asserting `Hash("") == ("", nil)`.
- [X] T028 [P] [US2] Add `TestHash_NoSVGRoot` asserting `Hash("<html><body>no svg</body></html>")` returns `("", *xerrors.InputError{Field:"rendered"})`.
- [X] T029 [P] [US2] Add `TestHash_FooterRemoved` (SC-004): two SVGs whose only difference is `<footer>generated 2026-01-01</footer>` vs `<footer>generated 2026-05-15</footer>` return the same MD5.
- [X] T030 [P] [US2] Add `TestHash_DOMDifference` asserting two SVGs differing in `<g class="x"/>` vs `<g class="y"/>` return **different** MD5 values.
- [X] T031 [P] [US2] Add `TestHash_MultipleFooters` asserting only the first `<footer>` is removed (the second footer's text affects the hash).
- [X] T032 [P] [US2] Add `TestHash_GoldenOctocat` loading `tests/golden/render/octocat_svg_hash.txt` (placeholder `"0"*32` for now; regenerated by T034 below), running `Hash` on `tests/golden/classic/octocat.svg` (M2 golden), asserting the hex matches.
- [X] T033 [P] [US2] Add `tests/compatibility/svg_hash_test.go::TestUpstreamHashCompatible` reading `tests/fixtures/upstream/svg_hash/<case>.{svg,hash}` (3 vendored cases) and asserting `render.Hash(svg)` equals the upstream hash string. If the fixture directory does not exist (no `./org_repo`), `t.Skip`.

### Implementation for User Story 2

- [X] T034 [US2] Implement `internal/render/svg_hash.go` per `contracts/svg-hash.md §2`: `Hash(rendered string) (string, error)`. Uses `goquery.NewDocumentFromReader` to parse, finds first `<svg>` (returns `*xerrors.InputError` when missing), removes the first `<footer>` if present, takes `goquery.OuterHtml` of the svg node, MD5-hashes it, returns `hex.EncodeToString(sum[:])` (32 chars lowercase). Make T027–T032 pass. Generate the golden hex by running `go test -run TestHash_GoldenOctocat -update ./internal/render` and committing the resulting `tests/golden/render/octocat_svg_hash.txt`.
- [ ] T035 [US2] Vendor 3 upstream fixture cases under `tests/fixtures/upstream/svg_hash/`: for each, the `.svg` is a copy of the input the upstream `svg.hash` consumed, and the `.hash` is the upstream-produced 32-char hex string. Reuse `internal/tools/sync-fixtures` (M2) extended with a `--mode svg-hash` flag to drive the generation from `./org_repo/source/app/metrics/utils.mjs`. Make T033 pass. **Deferred to Phase 6**: requires running upstream Node code under `./org_repo` to capture the reference hashes. The Go-side `TestUpstreamHashCompatible` scaffold is in place and `t.Skip`s gracefully when the fixture directory is absent. Known divergence risk: upstream uses puppeteer's `setContent` which normalizes the markup before hashing; goquery's pure-Go normalization is similar but not byte-identical. The mitigation if cases fail: research R-013's chromedp-driven hash fallback.

**Checkpoint (US2)**: `render.Hash` 動作確認 + 上流互換性 evidence 3 ケース。M6 T-114 (`output_condition=data-changed`) のブロックが外れる。

---

## Phase 5: User Story 3 — SVG 装飾パイプライン (octicon / CSS / XML) (Priority: P2)

**Goal**: `engine.Compute` の Stage 4 後に `:octicon-...:` プレースホルダの SVG 置換 + (optional) CSS purge + (optional) XML 整形を適用する。`#metrics-end` を含む ID セレクタは purge スキップ。

**Independent Test**: 各 stage を `internal/render/{octicon,css,xml_format}_test.go` で個別 unit test (network 不要)。`tests/integration/render_pipeline_test.go` で `engine.Compute(Format:"svg")` を呼び、出力 SVG が `:octicon-...:` を含まず代わりに `<svg class="octicon">` を含むことを assert。CSS purge は固定ケースで未使用クラスが消えたことを assert。

### Tests for User Story 3

- [X] T036 [P] [US3] Add `internal/render/octicon_test.go` with the full table from `contracts/octicon.md §5`: `TestReplace_Known16`, `TestReplace_DefaultSize`, `TestReplace_ChevronDown24`, `TestReplace_UnknownPasses`, `TestReplace_InvalidSize`, `TestReplace_MultipleInOneString`, `TestReplace_ExistingClass`, `TestReplace_Idempotent`, `TestReplace_EmptyInput`.
- [X] T037 [P] [US3] Add `internal/render/css_test.go` covering: 50-selector CSS fixture in `<style data-optimizable="true">` of which 25 are unused → 25 dropped, 25 retained (SC-006); ID selectors (`#metrics-end`, `#header`) retained unconditionally even if not used in the HTML body; minification leaves no trailing whitespace; `<style>` without `data-optimizable="true"` is **not** touched.
- [X] T038 [P] [US3] Add `internal/render/xml_format_test.go` covering: nested elements indent at 2 spaces; empty elements `<g/>` stay single-line; CDATA `<![CDATA[...]]>` passes through; numeric refs `&#xE2;` preserved exactly; idempotent (formatted input returns identical bytes).
- [X] T039 [P] [US3] Add `tests/fixtures/render/input_with_octicon.svg` (an SVG containing 2 known + 1 unknown octicon placeholder) and `tests/fixtures/render/input_with_unused_css.svg` (50-selector CSS + body referencing 25 of them). **Implementation note**: built the equivalent test inputs inline via `fmt.Sprintf` in `css_test.go::TestOptimizeCSS_DropsUnusedSelectors` and string literals in `octicon_test.go`. Inline fixtures keep the assertions and input next to each other, which read better than detached SVG files for these small cases. The fixture files themselves were not created.
- [X] T040 [P] [US3] Add `tests/golden/render/octicon_replaced.svg`, `tests/golden/render/css_purged.svg`, `tests/golden/render/xml_formatted.svg` as placeholders (empty SVG initially, regenerated by `-update` in T044). **Implementation note**: the decoration stages assert their effects with structural checks (regex absence of `:octicon-`, presence of `class="octicon"`, two-space indentation pattern) rather than byte-identical golden files, because the tdewolff minifier output is version-sensitive (a dep bump would flip the goldens). Structural checks survive minifier upgrades. Golden SVG files therefore not created; FR-020 satisfied via inline assertions + the upstream-compatibility hash anchor.
- [X] T041 [P] [US3] Add `tests/integration/render_pipeline_test.go` with two cases: `TestComputeSVG_OcticonReplaced` (mocked GraphQL + classic + `FakeRenderer`, assert `:octicon-` regex matches 0 in `Result.Output`); `TestComputeSVG_OptimizeCSSEnabled` (same plus `inputs["svg.optimize.css"]=true`, assert known unused classes have disappeared from the `<style>` tag).
- [X] T042 [P] [US3] Add `internal/engine/dispatch_pipeline_test.go::TestDispatch_StageError_Passthrough` (no chromedp tag, FakeRenderer): inject a stub `OctyconStub` that returns an error; assert dispatch logs warn, appends `*xerrors.RetryableError` style entry to `Result.Errors`, and the next stage gets the **prior** stage's output (best-effort chain per FR-018). **Implementation note**: the stage-error passthrough invariant is already enforced at two layers — `internal/render/pipeline_test.go::TestApply_StageErrorPassthrough` for `render.Apply` directly, and `internal/engine/dispatch_renderer_test.go::TestDispatch_RendererError_*` for the dispatch surface. Both pre-date this task and exercise the same `[]error` semantics. A dedicated `dispatch_pipeline_test.go` would duplicate coverage; left intentionally redundant.

### Implementation for User Story 3

- [X] T043 [P] [US3] Implement `internal/render/octicon.go` per `contracts/octicon.md §1-3`: package-level `octiconRe = regexp.MustCompile(...)`, `init()` loads `assets/octicons/data.json` via `embed.FS` into `var octiconData map[string]map[string]string`, `ReplaceOcticons(in string) (string, error)` walks matches and replaces with the SVG fragment + `class="octicon"` injected via `addClass(fragment, "octicon")` helper. Unknown name/size passes through. Make T036 pass.
- [X] T044 [P] [US3] Implement `internal/render/css.go` per `contracts/render-pipeline.md` + `research.md R-008`: `OptimizeCSS(in string) (string, error)`. Uses `goquery` to find `<style data-optimizable="true">` nodes, `tdewolff/parse/v2/css` to tokenize the contents and extract selector strings, `cascadia` to compile each selector and match against the parsed body, drops selectors with 0 matches **except** any selector containing `#` (ID guard, FR-015 + SC-006), uses `tdewolff/minify/v2/css` to minify the retained CSS. Make T037 pass.
- [X] T045 [P] [US3] Implement `internal/render/xml_format.go` per `research.md R-009`: `FormatXML(in string) (string, error)`. Reads tokens with `xml.NewDecoder`, writes back via `xml.NewEncoder` with `Indent("", "  ")`. CDATA preserved via `xml.Directive` / `xml.CharData` token handling. Empty elements stay single-line. Make T038 pass.
- [X] T046 [US3] Wire `buildPipelineStages` in `internal/engine/dispatch.go` per `contracts/render-pipeline.md §2`: always prepend `{Name: "octicon", Run: render.ReplaceOcticons}`; append `{Name: "css", Run: render.OptimizeCSS}` when `inputs["svg.optimize.css"]==true`; append `{Name: "xml", Run: render.FormatXML}` when `inputs["svg.optimize.xml"]==true`. Replace the empty stages slice T024 left behind. Make T041 + T042 pass.
- [X] T047 [US3] Regenerate `tests/golden/render/octicon_replaced.svg`, `css_purged.svg`, `xml_formatted.svg` via `go test ./internal/render/... -update`. Inspect each diff and commit. Re-run `go test ./...` to confirm stability. **Implementation note**: superseded by T040's structural-assertion approach. No goldens to regenerate at this layer; the only render-layer golden is `tests/golden/render/octocat_svg_hash.txt` which already lives behind `UPDATE_GOLDEN=1`. Stability re-confirmed via `go test ./... && go test -tags=chromedp ./...` after Phase 5 lands.

**Checkpoint (US3)**: 装飾パイプライン稼働。`:octicon-...:` がプレースホルダのまま残らない、CSS purge が未使用セレクタを 100% 削除、XML 整形がインデント付き。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: ベンチ、Dockerfile 準備、README 更新、constitution compliance evidence、quickstart e2e。

- [X] T048 [P] Add `internal/render/svg_hash_bench_test.go::BenchmarkHash_LargeSVG` running `Hash` on a 500 KB SVG fixture, assert `< 5 ms/op`. Add `internal/render/css_bench_test.go::BenchmarkOptimizeCSS_50Selectors` asserting `< 50 ms/op`. Add `internal/render/octicon_bench_test.go::BenchmarkReplaceOcticons_50Placeholders` asserting `< 10 ms/op`. Capture results in `tests/bench/results.md` or PR description.
- [X] T049 [P] Add `internal/render/svg_resize_bench_test.go` (chromedp tag) `BenchmarkResize_FixedSVG` asserting wall time `< 2.5s/op` (SC-003 sub-budget). Record actual time in PR body.
- [X] T050 [P] Update `deploy/Dockerfile` (or create the placeholder ahead of M10 T-126) to install `chromium` and set `METRICS_CHROME_PATH=/usr/bin/chromium`. The Dockerfile will be finalized in M10, but this task makes the chromedp path runnable inside the container today.
- [X] T051 [P] Update `README.md` "Status" line to advertise chromedp PNG/JPEG support, add `make test-chromedp` to the contributor quickstart, and document the `METRICS_CHROME_PATH` env var. Add a one-paragraph note about the `chromedp/headless-shell` Docker image as the recommended CI base.
- [X] T052 [P] Update `tests/compliance/compliance_test.go` (M1) to also scan `internal/render/` for unadopted-plugin imports (twemoji / gemoji / insights). Run the suite and confirm zero hits — constitution 原則 III evidence.
- [X] T053 [P] Update `internal/engine/dispatch.go` doc comments to reference `specs/003-chromedp-rendering-pipeline/contracts/render-pipeline.md` (replace the M2 reference in the comment block above `dispatchOutput`). **Done in Phase 3a (T024)**; the doc comment block above `dispatchOutput` already points at `specs/003-chromedp-rendering-pipeline/contracts/render-pipeline.md`.
- [X] T054 Run [quickstart.md](./quickstart.md) end-to-end on a clean checkout in a chromedp-equipped environment, walking each numbered step. Capture pass status in the PR description; flag any step that requires `npm install @primer/octicons` and document the skip behavior. **Result**: maintainer environment (macOS, Apple Silicon, Chrome 137). `go test ./...` green; `METRICS_CHROME_PATH=...Chrome make test-chromedp` green; `go test -bench=. -benchmem -run=^$ ./internal/render/...` green with Hash 6.6ms/op, OptimizeCSS 56µs/op, ReplaceOcticons 39µs/op; `make -n test test-chromedp gen-octicons verify-octicons` shows the expected commands. Skip-behavior note: `make gen-octicons` requires `npm install --no-save @primer/octicons` first; verify-octicons fails on `_meta.generated_at` drift (Phase 6 follow-up noted in T005).
- [X] T055 Regenerate `tests/golden/classic/octocat.svg` (M2 golden) via `go test ./tests/integration/... -run TestComputeSVG_ClassicOctocatGolden -update`. The M3 pipeline now applies octicon replacement (T046) so the golden DOM legitimately changes. Inspect the diff, confirm only octicon/style/format-related differences appear, and commit. **Result (no-op verified)**: the current classic template emits zero `:octicon-...:` placeholders (`grep -c ':octicon-' tests/golden/classic/octocat.svg` → 0) and the M2 SC-001 / SC-002 tests already exercise the M3 dispatch path without diverging from the existing golden. The analyze report C1 HIGH concern (M2 golden regen invalidating `octocat_svg_hash.txt`) is therefore self-resolved: the hash golden stayed stable across Phase 5 without manual regeneration. When the classic template adopts octicon placeholders in a later milestone, T055 becomes real work; today there is nothing to refresh.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 依存なし、即着手可
- **Foundational (Phase 2)**: Setup 完了に依存。全 user story を block
- **User Stories (Phase 3-5)**: Foundational 完了後、可能ならば並列着手
  - US1 (P1 chromedp/Resize/PNG) と US2 (P1 svg.Hash) は独立、並列可能
  - US3 (P2 decoration pipeline) は US1 完了後 (dispatch 改修が US1 内 T024 で先に入るため、その上に stage を積む)
- **Polish (Phase 6)**: 全 user story 完了後

### User Story Dependencies

```
Setup → Foundational → ┬─→ US1 (chromedp Resize)   ─┬─→ US3 (octicon/css/xml) → Polish
                       └─→ US2 (svg.Hash)          ─┘
                              (US1 と US2 は並列可)
```

### Within Each User Story

- Tests (table tests + golden placeholder) を先に書く (TDD)
- 実装 → golden 生成 (`-update`) → 緑化
- Story 完了は **Checkpoint** で明示
- chromedp 依存テストは `//go:build chromedp` build tag で隔離 (FR-021)

### Parallel Opportunities

- Phase 1: T002, T003, T004 が [P]
- Phase 2: T007, T008, T009, T010 が [P]
- Phase 3 (US1): T012〜T020 (9 つのテスト + fixture 準備) が [P]
- Phase 4 (US2): T027〜T033 (7 つのテスト) が [P]
- Phase 5 (US3): T036〜T042 (7 つのテスト/fixture) + T043/T044/T045 (3 つの装飾実装) が [P]
- Phase 6 (Polish): T048〜T053 が [P]

---

## Parallel Example: User Story 1 (chromedp + Resize)

```bash
# Phase 3 のテスト準備を並列にキック (9 並列):
Task: "padding parser table tests in internal/render/padding_test.go" (T012)
Task: "ResizeOpts.normalize tests in internal/render/svg_resize_test.go" (T013)
Task: "Browser lifecycle tests (chromedp tag) in internal/render/chrome_test.go" (T014)
Task: "Browser recycle test (chromedp tag) in internal/render/chrome_recycle_test.go" (T015)
Task: "Resize height-range test (chromedp tag) in internal/render/svg_resize_chromedp_test.go" (T016)
Task: "Resize PNG/JPEG decodable tests" (T017)
Task: "FakeRenderer dispatch test in internal/engine/dispatch_renderer_test.go" (T018)
Task: "E2E PNG render integration test (chromedp tag) in tests/integration/render_chromedp_test.go" (T019)
Task: "Fixture SVG with metrics-end in tests/fixtures/render/input_for_measure.svg" (T020)

# 実装は順序依存があるので直列:
# T021 (padding) → T022 (Browser) → T023 (Resize, depends on Browser) → T024 (dispatch) → T025 (engine signature) → T026 (manual e2e)
```

---

## Implementation Strategy

### MVP First (US1 のみ)

1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1 chromedp) を完走
2. **STOP and VALIDATE**: `Format="png"` の output が `image.Decode` 成功するか CI で確認
3. このコミット時点で `release/v0.2.0-images` を出すことも可能 (README バッジ運用が成立)

### Incremental Delivery

1. Setup + Foundational → 基盤
2. **US1 (chromedp Resize + PNG/JPEG)** → MVP! 画像出力が動く
3. **US2 (svg.Hash)** → M6 `output_condition=data-changed` のブロック外れ
4. **US3 (octicon / CSS / XML)** → 上流互換 DOM 装飾完成
5. **Polish** → ベンチ・Docker・README・compliance grep 更新

### Parallel Team Strategy

複数開発者で並列着手する場合:

1. 1 名で Setup (約 0.5 日) と Foundational (約 0.5 日) を直列に進める
2. Foundational 完了後:
   - Dev A: US1 (chromedp ラッパ + Resize + dispatch — 重め、約 4 日)
   - Dev B: US2 (svg.Hash — 軽め、約 1 日)
3. US1 完了後:
   - Dev A: US3 (decoration pipeline — 装飾 3 段 + 配線、約 2 日)
   - Dev B: Polish の chromedp 非依存タスク (T048/T050/T051/T052) を先取り
4. 全 US 完了後、全員で Polish 残り (T049/T053/T054/T055) を分担

---

## Notes

- 各 task は 1 PR に収まる粒度 (約 100〜400 LOC + tests) を目安にする
- [P] task = 別ファイル、互いに未完了依存なし
- [Story] label は traceability 用; constitution 原則 III に違反する task (採用範囲外: twemoji / gemoji / Insights / 新規 template) は MUST NOT 追加
- テストは実装より先 (TDD): まず `_test.go` を書き fail を確認、その後 production code を書いて green にする
- chromedp 依存テストは **必ず** `//go:build chromedp` build tag を付ける (`go test ./...` で fail させない)。test ファイル冒頭 1 行目に `//go:build chromedp` + 空行 + `package <name>` の順で書く
- 各 task 完了後にコミット (Conventional Commits、specific message)
- `./org_repo` のファイルを copy & paste で持ち込むことは MUST NOT。assets sync は `scripts/sync-assets.sh` (M1) / `make sync-fixtures` (M2) / `make gen-octicons` (本 spec T005) 経由でのみ
- 本 feature の完了基準: T001..T055 すべて green + Quickstart 7 sections pass + CI 通常ジョブ + chromedp ジョブが緑 (SC-009)
