# Implementation Plan: chromedp レンダリングパイプライン (M3)

**Branch**: `003-chromedp-rendering-pipeline` | **Date**: 2026-05-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-chromedp-rendering-pipeline/spec.md`

## Summary

M2 で `engine.Compute` は classic SVG / JSON の bytes を返すところまで動いた。本 feature は M2 の `dispatchOutput` が PNG/JPEG 経路で残していた `chromedp conversion lands in M3` warn ログを解消し、**実ブラウザで SVG の最終高さを計測 → `<svg height>` を書き換え → 必要なら PNG/JPEG にスクリーンショット** までを実装する。同時に、上流 `lowlighter/metrics` の出力に近づけるための **SVG 装飾パイプライン** (`:octicon-...:` の SVG 置換、`<style data-optimizable="true">` の未使用セレクタ purge、XML 整形) を engine の Stage 4 直後に挟む。さらに M6 で必要な `svg.Hash` (footer 除去後 outerHTML の MD5) を pure Go で先取り実装する。chromedp 起動失敗や測定タイムアウト時は装飾前 SVG を best-effort で返し、`Result.Errors` に typed error を追記する (panic しない)。

## Technical Context

**Language/Version**: Go 1.26 (M1/M2 から継続)。

**Primary Dependencies**:

- 標準: `encoding/xml`, `regexp`, `crypto/md5`, `image/png`, `image/jpeg`, `embed`, `bytes`, `strings`, `sync`
- M1/M2 で導入済み: `log/slog`, `gopkg.in/yaml.v3`, `Khan/genqlient`, `golang.org/x/sync/errgroup`, `hashicorp/go-retryablehttp`
- **新規追加 (本 spec)**:
  - `github.com/chromedp/chromedp` (latest stable) — ヘッドレス Chromium 制御。`svg.Resize` と将来の M4 plugin scrape (`topics`, `starlists`) で使う
  - `github.com/chromedp/cdproto` — chromedp が依存する DevTools protocol 型定義 (transitive だが go.mod に出る)
  - `github.com/PuerkitoBio/goquery` — DOM 操作。`svg.Hash` の footer 除去 + CSS purge 段の本文セレクタマッチで使う
  - `github.com/andybalholm/cascadia` — CSS セレクタパーサ (goquery 互換、CSS purge のセレクタ照合用)
  - `github.com/tdewolff/parse/v2/css` — CSS トークナイザ
  - `github.com/tdewolff/minify/v2` (+ `/v2/css`) — CSS minify

**Storage**:

- `assets/octicons/data.json` を `embed.FS` 経由でロード (octicon 置換テーブル)。本 spec 内に generation tool `internal/tools/gen-octicons/main.go` を実装し、`make gen-octicons` で primer/octicons から生成する。
- chromedp の ExecAllocator は一時 user-data-dir (`os.TempDir()/metrics-chrome-*`) を作成し、`Browser.Close()` で `RemoveAll` する。永続キャッシュは持たない。
- `tests/golden/render/` に各装飾段の goldenfile を置く (M2 で確立した `tests/golden/` 階層を踏襲)。

**Testing**:

- chromedp 非依存テスト: 標準 `go test ./...` で常時実行。`internal/render/{svg_hash,octicon,css,xml_format}_test.go` と engine fake renderer 経由の `engine_test.go`。
- chromedp 依存テスト: `//go:build chromedp` build tag で隔離、`make test-chromedp` ターゲットのみ実行。CI は `chromedp/headless-shell` イメージを使うジョブで動かす。
- golden file: PNG/JPEG はバイト同一不能のため、`width >= N && height >= N` のレンジ検証 + `image.Decode` 成功で代替 (`docs/design/10-testing-deployment.md §1.1`)。

**Target Platform**: M1/M2 と同じ (GitHub Actions runner / 配布バイナリ)。chromedp は本 spec で初めて runtime 依存になるため、Dockerfile (M10 T-126) で `chromium` パッケージを同梱する前提を確立する。

**Project Type**: 単一 Go プロジェクト (cmd/+internal/)。

**Performance Goals**:

- `Compute` (chromedp 込み、classic + 主要 10 plugin) 1 ユーザー分 < 5 秒 (constitution §3 / SC-003)
- `Resize` 単体 < 2.5 秒 (`Sleep(2.4s)` + 計測 + Screenshot)
- `Hash` < 5 ms / SVG 1 本 (pure Go、ベンチで確認)
- `OptimizeCSS` < 50 ms / 50 セレクタ
- `FormatXML` < 30 ms / 200 要素

**Constraints**:

- **原則 I (input compat)**: 新規 input なし。`svg.optimize.css` / `svg.optimize.xml` / `config.padding` はすべて M1 で `core` plugin が metadata.yml に既に持っている input。新規 key 追加は **MUST NOT**。
- **原則 II (output contract)**: SVG は DOM 同等、JSON は本 spec で変更しない。PNG/JPEG はピクセル単位同一を目指さず、`image.Decode` 成功 + 期待 width/height レンジでのみ assert (SC-001)。
- **原則 III (scope)**: 本 spec の範囲は T-031〜T-035 + T-038 + T-039 + T-033 のみ。twemoji (T-036) / gemoji (T-037) は **MUST NOT 含めない** (空 hook stage のみ予約)。
- **原則 IV (tests + golden)**: 全装飾段に goldenfile + chromedp ジョブに 1 ケース以上の SVG → PNG e2e。
- **原則 V (Go conventions)**: 新規依存追加根拠は本 plan の Technical Context に明示済。code は英語、docs は日本語、chromedp 起動 flag (`--no-sandbox` 等) は文字列定数で集中管理する。
- **下位互換**: M2 で `Result.Output` を消費する側 (現状はテストのみ) は `MIME` の値しか見ていない。本 spec で SVG bytes の内容が「Template.Run 直後」から「装飾 + Resize 後」に変わるが、`MIME` 契約は維持されるため上位互換。

**Scale/Scope**:

- 本 spec: T-031, T-032, T-033, T-034, T-035, T-038, T-039 + N-001 派生 (gen-octicons tool) = ~8 タスク。Phase 2 (`/speckit-tasks`) で細分化。
- 想定追加パッケージ: `internal/render/` (新規ディレクトリ + 6 ファイル)、`internal/tools/gen-octicons/` (新規 tool)、`tests/golden/render/` (新規階層)、`tests/integration/render_pipeline_test.go` (新規)。
- 想定 LOC: 本体 +1000 (chromedp ラッパが重い)、テスト +700。

## Constitution Check

*GATE: Phase 0 前と Phase 1 後の両方で再評価する。*

### Initial Constitution Check (pre Phase 0)

| 原則 | 内容 | 適合性 | 根拠 |
|---|---|---|---|
| I. 入力互換性 (NON-NEGOTIABLE) | `action.yml` / `settings.json` キー完全互換 | ✅ PASS | 新規 input ゼロ。`svg.optimize.*` / `config.padding` は上流互換 input を再利用 (FR-019)。 |
| II. 出力契約 (DOM/JSON 単位) | JSON 完全互換、SVG/Markdown は DOM 同等 | ✅ PASS | JSON 経路は本 spec で変更なし。SVG は M2 出力に装飾を上乗せするのみで、`<svg>` ルート + `<foreignObject>` + `<div class="items-wrapper">` の階層を維持 (FR-017)。PNG/JPEG はバイト一致を目指さず、SC-001 で `image.Decode` 成功 + 最低 width/height レンジで担保。 |
| III. スコープ規律 | 採用機能のみ実装 | ✅ PASS | 本 spec は T-031〜T-035 + T-038 + T-039 + T-033 のみ。twemoji / gemoji / Insights は **明示的に除外** (Assumptions 参照)。 |
| IV. テーブルテスト + Golden File | mocked + golden 強制 | ✅ PASS | `tests/golden/render/` 配下に octicon / css / xml の golden、chromedp 依存テストは fake renderer + chromedp build tag の 2 系統。SC-007/SC-009。 |
| V. Go 規約と言語ポリシー | Go latest, cmd/+internal/, 日本語 docs / 英語 comments | ✅ PASS | 新規依存 (chromedp, goquery, cascadia, tdewolff) は本 plan で採用根拠を明示。新規ファイルはすべて `internal/render/`, `internal/tools/gen-octicons/` 配下。 |

**Gate result**: PASS — 違反なし、Complexity Tracking 不要。

### Post-Design Constitution Check (after Phase 1)

| 原則 | 設計後の再評価 |
|---|---|
| I | `contracts/render-pipeline.md` に「input は M1 `core` plugin で読み込み済の `svg.optimize.css/xml`, `config.padding` のみ参照」と明記。`Request` / `Inputs` map のスキーマ拡張なし。差分ゼロ。 |
| II | `contracts/svg-resize.md` で `<svg>` ルート属性の変更箇所 (height のみ) を明示、その他の DOM は touched しない。`contracts/octicon.md` で octicon 置換が `<svg class="octicon">` 1 要素単位のローカル置換であることを定義。SC-001/002 で測定可能。 |
| III | `contracts/render-pipeline.md` の段一覧に twemoji / gemoji / Insights が **登場しない** ことで scope 規律を担保。 |
| IV | `contracts/svg-hash.md` / `contracts/octicon.md` / `contracts/render-pipeline.md` に goldenfile pathと test 命名規則を明示。chromedp build tag の隔離は `quickstart.md` で再確認。 |
| V | data-model 内の Go 型 (`render.Browser`, `render.ResizeOpts`, `render.ResizeResult`) は英語、コメントは英語。新規依存追加根拠は plan Technical Context に列挙済。 |

**Gate result**: PASS — 設計でも違反なし。

## Project Structure

### Documentation (this feature)

```text
specs/003-chromedp-rendering-pipeline/
├── plan.md                # 本ファイル
├── research.md            # Phase 0 成果物 (chromedp / CSS purge / octicon 戦略の決定記録)
├── data-model.md          # Phase 1 成果物 (render.Browser / ResizeOpts / ResizeResult / PipelineStage の Go 型)
├── quickstart.md          # Phase 1 成果物 (chromium 同梱手順 + build tag + make ターゲット)
├── contracts/             # Phase 1 成果物
│   ├── svg-resize.md          # chromedp 評価スクリプト, padding パース, Screenshot 規則
│   ├── render-pipeline.md     # engine.Compute Stage 4 後の装飾段ディスパッチ
│   ├── svg-hash.md            # footer 除去 + MD5 アルゴリズム
│   └── octicon.md             # :octicon-<name>-<size>: 置換規則 + 未知 octicon 素通し
├── checklists/
│   └── requirements.md    # /speckit-specify で生成済
└── tasks.md               # /speckit-tasks 出力 (未作成)
```

### Source Code (repository root)

M2 の構成から **新規** に追加されるファイルのみ列挙する (既存ファイルへの編集は本 plan 内で明記する)。

```text
github-metrics/
├── internal/render/
│   ├── chrome.go              # 新規: Browser ラッパ (allocCtx 管理 + recycle)。
│   ├── chrome_test.go         # 新規: build tag = chromedp、起動 / NewTab / Close。
│   ├── svg_resize.go          # 新規: Resize(ctx, in, opts) — chromedp 経由で高さ計測 + Screenshot。
│   ├── svg_resize_test.go     # 新規: build tag = chromedp、固定 SVG → 期待 height range。
│   ├── svg_hash.go            # 新規: Hash(in) — goquery で footer 除去 + MD5。
│   ├── svg_hash_test.go       # 新規: pure Go テスト。
│   ├── octicon.go             # 新規: ReplaceOcticons(in) + assets/octicons/data.json embed。
│   ├── octicon_test.go        # 新規: 既知 / 未知 / size 省略のテーブルテスト。
│   ├── css.go                 # 新規: OptimizeCSS(in) — purge + minify。
│   ├── css_test.go            # 新規: 50 セレクタ中 25 未使用ケース。
│   ├── xml_format.go          # 新規: FormatXML(in) — 2sp インデント + 改行。
│   ├── xml_format_test.go     # 新規: 入れ子要素のインデント検証。
│   ├── pipeline.go            # 新規: PipelineStage 型 + Apply(stages, in) — best-effort 連鎖。
│   ├── pipeline_test.go       # 新規: stage 失敗時の直前値素通し検証。
│   ├── renderer.go            # 新規: Renderer interface + FakeRenderer (テスト用)。
│   └── fake_test.go           # 新規: FakeRenderer の決定論的 1x1 PNG 動作。
├── internal/engine/
│   ├── engine.go              # 既存。Deps に Render Renderer を追加、Stage 4 後に装飾段を呼ぶ。
│   ├── dispatch.go            # 既存。svg/png/jpeg 経路を render.Renderer 経由に差し替え (M2 warn ログ削除)。
│   └── engine_test.go         # 既存。FakeRenderer 注入の PNG/JPEG 経路テストを追加。
├── internal/tools/gen-octicons/
│   └── main.go                # 新規: primer/octicons から assets/octicons/data.json を生成する CLI。
├── assets/octicons/
│   └── data.json              # 新規 (生成物): 16/24px の全 octicon を {name: {size: svg-fragment}} で持つ。
├── tests/
│   ├── golden/render/
│   │   ├── chromedp_height_octocat.json  # 新規: width/height レンジ期待値。
│   │   ├── octicon_replaced.svg          # 新規: octicon 置換後 SVG fragment。
│   │   ├── css_purged.svg                # 新規: CSS 最適化後 SVG fragment。
│   │   └── xml_formatted.svg             # 新規: XML 整形後 SVG fragment。
│   ├── integration/
│   │   ├── render_pipeline_test.go       # 新規: engine.Compute(Format:"svg") に対する pipeline e2e。
│   │   └── render_chromedp_test.go       # 新規 (build tag=chromedp): SVG → PNG/JPEG e2e。
│   └── fixtures/
│       └── render/
│           ├── input_with_octicon.svg    # 新規: octicon プレースホルダ入り SVG。
│           ├── input_with_unused_css.svg # 新規: 50 セレクタ中 25 未使用の CSS 入り SVG。
│           └── input_for_measure.svg     # 新規: #metrics-end 入りの最小 SVG。
└── Makefile                              # 既存。test-chromedp / gen-octicons ターゲット追加。
```

**Structure Decision**: M2 で確立した `internal/engine` / `internal/templates/classic` を **拡張** する形で実装。本 spec のレンダリング系コードはすべて新規 `internal/render/` パッケージに集約し、エンジン側は `Renderer` interface 1 つを `Deps` に追加するだけで結合する。これにより、chromedp 依存は `internal/render` の中だけに閉じ込められ、他パッケージは `RendererFake` でユニットテストできる (FR-022)。`gen-octicons` は既存 `internal/tools/` 階層 (`check-compat` / `sync-fixtures` / `gen-graphql`) に倣う。

## Complexity Tracking

> Constitution Check は PASS につき記入不要。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (なし) | — | — |
