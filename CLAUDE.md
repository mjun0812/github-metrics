# github-metrics

upstream `lowlighter/metrics` (Node.js/EJS) の **subset** を Go に移植した GitHub プロフィールカード生成ツール。GitHub Action と CLI の2形態で動く (Web インスタンスは不採用)。v4.0.0 (2026-07-09) 以降、レンダリングは完全に browser-free。

## Adopted MVP scope (DO NOT DEVIATE)

新しい機能に着手する前に、必ず以下の source of truth で採用対象を確認すること。

- **採用機能定義・MVP スコープ**: [docs/scope.md](docs/scope.md)

### Skipped (実装禁止)

- **M5**: Web インスタンス (chi server / OAuth / insights 等の HTTP 公開機能) — 「運用コストが高く不要」と決定済
- **M8**: ソーシャル / 外部 API plugin (anilist / leetcode / chess / steam / music / pagespeed / tweets / stackoverflow / wakatime 等) — 全数不採用

### Enforcement (compliance tests)

`tests/compliance/compliance_test.go` が以下を gating する。落ちた場合は即対処:

- `TestCompliance_M4_AdoptedPlugins`: 採用 19 plugin dir の厳密一致
- `TestNoUnadoptedPluginReference`: 不採用 plugin slug の production code への混入検出
- `TestNoRemovedSentinelComments`: `// removed:` 系の削除履歴コメントの禁止 — **コード内コメントには現在の性質だけを書き、「何を消したか」を書かない**

## Rendering architecture (#409 以降)

v4.0.0 で chromedp/Chromium を排除し、native SVG + resvg のパイプラインに全面移行した。視覚まわりを触るときは必ずこの前提で:

- **partial は native SVG を出力し、消費高さを自己申告する**: シグネチャは `(markup string, height int, err error)`。テンプレートが `chrome.StackSections` で縦積みし、root `<svg height>` を Go 側で確定する (`height="99999"` プレースホルダや計測パスは存在しない)
- **SVG プリミティブは `internal/templates/chrome/svg.go`** (`WrapSection` / `SVGText` / `SVGField` / `SVGColumn` / `SVGAvatarGrid` 等)。新しい partial 実装はここを再利用する
- **テキスト幅は `internal/render/fontmetrics`** (Liberation Sans 埋め込み) で計測する。閲覧ブラウザの fallback フォントは最大 ~13.5% 幅広なので、**幅を固定する要素には ~14% のヘッドルーム** (`chipLabelSafety` / `achValueSafety` が前例) を取り、はみ出しやすいものは `text-anchor="middle"` で中央寄せする
- **PNG/JPEG は `internal/render/resvg.go`** が resvg バイナリ (subprocess、`METRICS_RESVG_PATH`) で描画する。resvg は `var()` / `:root` / `:not` 等を解釈しないため、**色は literal fill/stroke 属性で出力**する (`@keyframes` は無視され inline 最終値で描かれるので、gauge 等のアニメ CSS は残してよい)
- **ネスト `<svg>` の viewport は card 幅 (480px) と申告高さでクリップされる**: HTML と違い overflow が visible にならない。要素が境界を越えないよう clamp / 高さ算入を忘れない (PR #722 が前例)
- `chrome_*` input と `internal/templates/chrome` の「chrome」は **UI 用語 (カードの枠) で、ブラウザとは無関係**

詳細は [docs/rendering.md](docs/rendering.md)。

## Development workflow

- **テスト**: `make test` (unit)。golden 更新は `go test ./tests/integration/... -update` + 対象 package の `-update` + `UPDATE_GOLDEN=1 go test ./internal/render -run TestHash_GoldenOctocat`。**footer timestamp だけが変わった golden はコミットに含めず revert する**
- **resvg 依存テスト**: `make test-resvg` (要 resvg バイナリ + `METRICS_RESVG_PATH`)。未設定なら自動 skip
- **lint**: CI の golangci-lint は prealloc / unparam / revive / gosec が有効で、`make test` では検出できない。**push 前に必ずローカルで `golangci-lint run ./internal/... ./tests/...` を 0 issues にする**
- **入力バリデーション**: metadata.yml の min/max を実行時に適用する層は存在しない。**新しい整数入力は読み取り箇所で必ず clamp する** (非正値はデフォルトへフォールバック、GraphQL connection の `first` は 100 上限。`habits.go` / `reactions.go` #472 が前例)
- **視覚変更の検証**: golden SVG を resvg で PNG 化して目視すること。resvg は font-family が解決できないと `<text>` を全 skip するので、`--sans-serif-family "Liberation Sans"` 等の generic mapping を渡す
- **doc サンプル**: `docs/examples/` は regen-doc-samples workflow (`gh workflow run regen-doc-samples.yml -f branch=main`) が draft PR で更新する。手元で編集しない
- **リリース**: semver タグ (`vX.Y.Z`) の push だけで release.yml が全て行う (multi-arch イメージ + バイナリ + cosign + vMAJOR floating tag)

## Historical context

- M1–M10 (移植 MVP): v1.0.0 として 2026-05-18 リリース
- #409 (chromedp 排除、native SVG + resvg 化): v4.0.0 として 2026-07-09 リリース。経緯・意思決定ログは issue #409 と sub issue #682–#695 に記録
