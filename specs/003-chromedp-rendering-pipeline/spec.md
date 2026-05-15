# Feature Specification: chromedp レンダリングパイプライン (M3)

**Feature Branch**: `003-chromedp-rendering-pipeline`

**Created**: 2026-05-15

**Status**: Draft

**Input**: User description: "M3: chromedp rendering pipeline"

## User Scenarios & Testing *(mandatory)*

本機能の「ユーザー」は 2 種類存在する。

- **GitHub Action 利用者 (一次)**: `output: png` / `output: jpeg` を指定して README に貼る画像を取得する、または `output: svg` で `output_condition: data-changed` を有効にしたい利用者。M2 までは PNG/JPEG 経路でも生 SVG バイトが返るだけだった (M2 `internal/engine/dispatch.go` の warn ログを参照)。
- **データ消費 downstream (二次)**: 出力 SVG の DOM 装飾 (octicon/gemoji/twemoji 置換、CSS purge、XML 整形) を期待する README 読者・スクリーンリーダーなど。バイト一致は不要だが、上流 `lowlighter/metrics` と DOM 単位で同等の「見た目」を出すことが目標 (constitution 原則 II)。

### User Story 1 - SVG を chromedp でリサイズし最終 SVG / PNG / JPEG を得る (Priority: P1)

GitHub Action 利用者が `output: svg|png|jpeg` を指定したとき、`engine.Compute` が classic テンプレート出力 SVG を実ブラウザで描画して **`#metrics-end` 要素のバウンディング** を計測し、`<svg height="...">` 属性を確定させる。さらに `output ∈ {png, jpeg}` のときは `page.Screenshot` で確定サイズの画像バイトを返す。`Result.MIME` が `image/svg+xml` / `image/png` / `image/jpeg` のいずれかになる。M2 で残っていた `engine: png output stages SVG bytes (chromedp conversion lands in M3)` warn ログは出なくなる。

**Why this priority**: README バッジ運用の主要 path。M2 で `Result.Output` には SVG が入っていたが、PNG/JPEG はバイト的に偽物だった。本ストーリーで初めて「画像が出る」 MVP 到達点となる。MVP 完走最短経路 (`docs/design/16-tasks-mvp.md §3` クリティカルパス) を満たす。

**Independent Test**: mocked GraphQL backend + classic + `chromedp` 起動可能環境 (Docker 内 chromium または `METRICS_CHROME_PATH` で指す chromium) で `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"png"})` を呼び、`image.Decode(bytes.NewReader(res.Output))` が成功し `bounds.Dx() > 0 && bounds.Dy() > 0` であることを assert。SVG 経路では戻り値の `<svg height="...">` が `auto` でない数値属性に書き換わっていることを assert。

**Acceptance Scenarios**:

1. **Given** classic テンプレートが registry に登録され、chromedp が利用可能 (`METRICS_CHROME_PATH` で chromium が見つかる) な状態で **When** `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼ぶ **Then** `Result.MIME == "image/svg+xml"`、`Result.Output` の `<svg>` ルート要素は `height` 属性を持ち、その値は整数文字列 (例 `"640"`) で `auto` ではない。
2. **Given** 同条件で `Format: "png"` を指定する **When** Compute を呼ぶ **Then** `Result.MIME == "image/png"`、`Result.Output` は `image/png` の magic header (`\x89PNG\r\n\x1a\n`) で始まり、`image.Decode` が `image.Image` を返す。
3. **Given** 同条件で `Format: "jpeg"` を指定する **When** Compute を呼ぶ **Then** `Result.MIME == "image/jpeg"`、`Result.Output` は `\xff\xd8\xff` で始まり、`jpeg.Decode` が成功する。
4. **Given** classic SVG が `<g id="metrics-end" transform="translate(0,640)"/>` を末尾に持つ状態で **When** chromedp が要素のバウンディング `y=640` を返した **Then** padding 既定 (`1.0 + 0`) では `<svg height>` が `"640"` に確定する。
5. **Given** `config.padding = "0 + 6%, 8 + 11%"` のような padding 指定がある状態で **When** Compute する **Then** width/height の最終値が `ceil(measured_dim * (1 + relative/100) + absolute)` で算定される (詳細は `docs/design/13-appendix.md §G.1`)。
6. **Given** chromedp の起動に失敗する環境 (chromium 不在 / sandbox 不可) で **When** Compute を呼ぶ **Then** `*RetryableError` 系または `*InputError` 系の typed error が `Result.Errors` に追加され、panic しない。`die=true` 経路で呼ばれていない限り Compute は完走する。

---

### User Story 2 - SVG ハッシュで `data-changed` モードを成立させる (Priority: P1)

GitHub Action 利用者が `output_condition: data-changed` を指定したい場合 (M6 T-114)、本 spec で追加する `svg.Hash` 関数を使って **直前リビジョン SVG と新規 SVG の `<footer>` 除去後 outerHTML の MD5** を比較し、一致時は commit をスキップできるようにする。chromedp は不要 (pure Go)、`goquery` で DOM 解析する。

**Why this priority**: M6 の T-114 (`output_condition=data-changed`) が直接ブロックされている。`svg.Hash` 自体は chromedp 非依存だが、本 spec の DOM 関連ユーティリティ群と一緒に M3 で完結させると後工程依存がない。

**Independent Test**: `internal/render/svg_hash_test.go` でテーブルテスト。入力 SVG 2 本 (同じ DOM だが `<footer>` の generated 時刻だけ違う) で `Hash(a) == Hash(b)`、DOM 要素を 1 つ削った 3 本目で `Hash(a) != Hash(c)` を assert。chromedp は経由しない。

**Acceptance Scenarios**:

1. **Given** classic SVG 2 本 (footer 内の `data-section="metadata"` の generated タイムスタンプのみが異なる、それ以外の DOM は同一) **When** `svg.Hash(svgA)` と `svg.Hash(svgB)` を呼ぶ **Then** 戻り値 MD5 hex は一致する。
2. **Given** classic SVG 2 本 (footer 含めて DOM が異なる: 例 user.followers の値が違う) **When** ハッシュを取る **Then** MD5 hex は不一致。
3. **Given** 空文字列 `""` または `nil` 相当 **When** Hash を呼ぶ **Then** 空文字 `""` を返し、エラーは返さない (`docs/design/13-appendix.md §H` の仕様)。

---

### User Story 3 - SVG 装飾: octicon / CSS purge / XML 整形 (Priority: P2)

GitHub Action 利用者が classic テンプレート出力 SVG に対し以下の処理を有効化できる:

1. `:octicon-<name>-<size>:` プレースホルダ → 埋め込み `assets/octicons/data.json` から `<svg>` 置換 (常時有効、ネットワーク不要)。
2. `svg.optimize.css` 入力で `<style data-optimizable="true">` 配下の未使用セレクタを削除 + minify。
3. `svg.optimize.xml` 入力で 2 スペースインデント + 末尾整形。

これらは `engine.Compute` の出力ステージで chromedp 前後に挟まれる (詳細はパイプライン順序を `contracts/pipeline.md` に固定する。Phase 1 成果物)。

**Why this priority**: classic テンプレートが embed する CSS と octicon プレースホルダを実際の見た目に変換する。M2 までは partial が `:octicon-...:` を出力できても、最終 SVG に SVG ノードが入っていないため `<img src=>` 等で読み込んだ際にプレースホルダ文字列が露出する欠陥があった。

**Independent Test**: 各機能を `internal/render/{octicon,css,xml_format}_test.go` で個別 unit test (network 不要)、加えて `tests/integration/render_pipeline_test.go` で `engine.Compute(Format:"svg")` を呼び、出力 SVG が `:octicon-star-16:` を含まず代わりに `<svg ... class="octicon">` を含むことを assert。CSS purge は固定ケースで未使用クラスが消えたことを assert。

**Acceptance Scenarios**:

1. **Given** classic partial 出力に `:octicon-star-16:` を含む **When** Compute が render 段階で octicon 置換を行う **Then** 出力 SVG に `<svg class="octicon"...><path d="..."/></svg>` (16px svg) が埋まり、プレースホルダ文字列は残らない。
2. **Given** `:octicon-repo:` (size 省略) **When** 置換する **Then** 16px の SVG が使われる (`docs/design/04-rendering.md §8.3` の既定)。
3. **Given** 未知 octicon `:octicon-doesnotexist-24:` **When** 置換する **Then** プレースホルダはそのまま素通り、panic / エラーは発生しない。
4. **Given** `svg.optimize.css = true` かつ `<style data-optimizable="true">.foo{}.bar{}</style>` を持ち、本文 HTML が `class="foo"` のみ参照する **When** CSS optimize を実行する **Then** `.bar` は削除され、最終 `<style>` には `.foo` のみ残る。
5. **Given** `svg.optimize.xml = true` **When** XML format を実行する **Then** 2 スペースインデント + 改行が `<svg>` 子要素ごとに入り、空要素は単一行で出力される。

---

### Edge Cases

- **`METRICS_CHROME_PATH` 未設定 + chromium が PATH に無い**: chromedp の `Allocator.Run` が失敗。`*RetryableError` でラップして `Result.Errors` に追加。`die=true` なら Compute は短絡。
- **chromedp 起動成功後にタイムアウト (3 秒以内に `#metrics-end` が DOM に現れない)**: タイムアウト後 `*RetryableError`、SVG は装飾前 (Template.Run 直後) の値をそのまま返す (壊れた SVG よりは見た目が成立する)。実装方針は `contracts/svg-resize.md` で確定。
- **Browser instance 再利用上限 (既定 200 回) 到達**: `Browser.Recycle()` で allocCtx を作り直す。クライアントから見るとブロッキングが入るが panic しない。
- **`paddings: ""` (空文字)**: 既定値 (絶対 0, 相対 0%) として扱う。
- **`paddings` がリスト/カンマ区切り両方で渡る**: `docs/design/13-appendix.md §G.1` のとおり、まず string なら `,` で split、次に最初の要素 → 2 要素目の順で width/height に割り当て、不足は最初の要素にフォールバック。
- **`svg.optimize.css=true` だが `<style data-optimizable="true">` ノードが無い**: no-op、エラー無し。
- **CSS optimizer が ID セレクタを誤削除**: SVG 内 `id="metrics-end"` は **必ず保持** (svg.Resize の計測対象、消すと壊滅)。ID セレクタは purge スキップ。
- **巨大 SVG (例 isocalendar 53 週 × 7 日のセル数千個)**: `page.SetContent` 〜 measure までの total wall time が 5 秒を超えない (constitution 性能目標)。
- **Compute から渡された `Format: "png"` だが Template が PNG 非サポート**: classic は `[svg, png, jpeg, json]` を `metadata.yml` で declare している前提。`Template.Check` で先に typed error を出す (M2 既存挙動)。
- **gemoji / twemoji 置換中にネットワーク失敗**: 本 spec では装飾 hook は実装しないが、空 stage を予約 (PipelineHook chain で no-op)。後続 spec で実装するときの設計と整合させる。

## Requirements *(mandatory)*

### Functional Requirements

#### chromedp ラッパ / svg.Resize / PNG·JPEG (US1, T-031 + T-032 + T-039)

- **FR-001**: System MUST `internal/render/chrome.go` に `Browser` 構造体を実装する: `New(opts BrowserOpts) (*Browser, error)`、`NewTab(ctx context.Context) (context.Context, context.CancelFunc, error)`、`Close() error`、内部で `chromedp.NewExecAllocator` を保持。`NewTab` は呼び出し側に tab context と cancel func を返し、cancel は defer 想定。
- **FR-002**: System MUST 起動 flag のデフォルトセットに `--no-sandbox`, `--disable-extensions`, `--disable-dev-shm-usage`, `--disable-gpu`, `--no-first-run`, `--no-default-browser-check` を含め、`METRICS_CHROME_PATH` 環境変数が空でなければ ExecPath として使用する。`--single-process` は **デフォルト除外** とし、Docker / ARM 安定性が必要な caller が `BrowserOpts.ExtraFlags` で opt-in できるようにする (Chrome 137+ desktop で `Network.enable` を破壊するため。経緯は実装の chrome.go inline doc 参照)。
- **FR-003**: System MUST `Browser.NewTab` を N 回呼び出すごとに (N は既定 200、上流 `svg.resize.browser` 互換) allocCtx を再生成する recycle 機構を持つ。再生成中の `NewTab` 呼び出しはブロックしてよい。
- **FR-004**: System MUST `internal/render/svg_resize.go` に `Resize(ctx context.Context, rendered string, opts ResizeOpts) (ResizeResult, error)` を実装する。`ResizeResult` は `{Body []byte, Width int, Height int, MIME string}`。
- **FR-005**: System MUST chromedp に `EmulateViewport(980, 980)` 相当を渡し、`page.SetDocumentContent` 相当で SVG を注入したのち、ユーザー任意 scripts (extras flag で有効化された配列) を `new Function("document", script)(document)` 形式で順次同期実行する。SettleDelay は `chromedp.Sleep` で JS の外側から駆動する (Runtime.evaluate の awaitPromise が JSON.stringify 返却で modern Chrome 上で不安定なため、prepare → Sleep → measure の 3 ステップに分割した)。
- **FR-006**: System MUST 計測前に `svg` クラスへ `no-animations` を一時付与、`SettleDelay` (既定 2.4 秒) 待機、`#metrics-end` 要素の `getBoundingClientRect().y` を SVG ルート bbox の `y` で減算して `height` を、`width` は **SVG ルートの `getBoundingClientRect().width`** から取得する (`#metrics-end` が空 `<g>` の場合 bbox `width` が 0 になる Blink 現実挙動への対応)。JS 評価スクリプトの実体は実装 (`internal/render/svg_resize.go`) の `jsPrepareTemplate` / `jsMeasureTemplate` 参照。
- **FR-007**: System MUST padding を `docs/design/13-appendix.md §G.1` のアルゴリズムで `(width, height) → ceil(measured_dim * (1 + relative/100) + absolute)` に変換し、`<svg height="...">` を書き換える。`<svg height="auto">` ノードは書き換えない。
- **FR-008**: System MUST `opts.Convert ∈ {"", "svg", "png", "jpeg"}` を扱う。空文字または `"svg"` のときは XMLSerializer 相当の `outerHTML` を bytes として返し `MIME = "image/svg+xml"`。`"png"` / `"jpeg"` のときは `page.CaptureScreenshot(clip={0,0,width,height}, format=convert, omitBackground=true)` で画像 byte slice を返し `MIME = "image/png" | "image/jpeg"`。
- **FR-009**: System MUST `engine.dispatchOutput` を改修し、`Format ∈ {svg, png, jpeg}` のとき (a) `Template.Run` の結果文字列を取得、(b) `render.Resize` を呼んで `(Body, MIME)` を取得、(c) M2 の `image/svg+xml` 直返しを置き換える。M2 で発火していた `chromedp conversion lands in M3` warn ログは除去する。
- **FR-010**: System MUST `engine.Deps` (依存注入) に `Render render.Renderer` フィールドを追加する。`Render == nil` の場合 Compute は初回利用時に `render.Browser.New` を遅延起動して `defer Close()` する (テスト / SDK 利用者の便宜)。テスト時は `*render.FakeRenderer` を `Deps.Render` に注入することで chromedp 実起動を回避できる (Renderer は 1 メソッド interface)。

#### svg.Hash (US2, T-033)

- **FR-011**: System MUST `internal/render/svg_hash.go` に `Hash(rendered string) (string, error)` を実装し、空文字を渡したときは `("", nil)` を返す。
- **FR-012**: System MUST `goquery` で `<footer>` 要素を 1 件削除した上で `<svg>` の `outerHTML` を取得し、その MD5 hex (32 文字小文字) を返す。
- **FR-013**: System MUST 同 DOM (footer のみ違う) に対し決定論的に同一ハッシュを返すテーブルテストを置く。

#### SVG 装飾パイプライン (US3, T-034 / T-035 / T-038)

- **FR-014**: System MUST `internal/render/octicon.go` に `ReplaceOcticons(rendered string) (string, error)` を実装。`:octicon-<name>(-<size>)?:` 正規表現で抽出し、`assets/octicons/data.json` (T-012 で生成済み) を `embed.FS` で読み、SVG 文字列 (`<svg class="octicon" ...>`) に置換する。未知 octicon は素通し。
- **FR-015**: System MUST `internal/render/css.go` に `OptimizeCSS(rendered string) (string, error)` を実装。`<style data-optimizable="true">` を抽出し、`tdewolff/parse/v2/css` でセレクタ列挙 → `cascadia` で本文 HTML 内のマッチ判定 → 未使用セレクタを削除 → `tdewolff/minify/v2/css` で minify。`#metrics-end` を含む ID セレクタは purge スキップ MUST。
- **FR-016**: System MUST `internal/render/xml_format.go` に `FormatXML(rendered string) (string, error)` を実装し、`lineSeparator="\n"`, インデント=2 スペース, `collapseContent=true` 相当で整形する。
- **FR-017**: System MUST `engine.Compute` の出力ステージで以下の順に変換を適用する MUST (詳細は `contracts/pipeline.md`):
  1. `Template.Run` (M2 で実装済み)
  2. `ReplaceOcticons` (常時 enable、ネットワーク不要)
  3. `OptimizeCSS` (input `svg.optimize.css=true` のとき)
  4. `FormatXML` (input `svg.optimize.xml=true` のとき)
  5. `Resize` (`Format ∈ {svg, png, jpeg}` のとき必須)
- **FR-018**: System MUST 装飾ステージ各段でエラーが発生した場合、`Result.Errors` に typed error を追加し SVG 本体は **直前段の値** をそのまま次段に渡す (best-effort パイプライン)。`die=true` のときのみ短絡。
- **FR-019**: System MUST settings の `svg.optimize.css` / `svg.optimize.xml` を `Request.Inputs` から bool として読み取り、既定値は両方 `false` とする (`metadata.yml` の `core` プラグイン定義に合わせる)。

#### テスト基盤

- **FR-020**: System MUST `tests/golden/render/` 配下に以下を置く: (a) `chromedp_height_octocat.json` (Resize の `Width/Height` 期待値レンジ), (b) `octicon_replaced.svg` (octicon 置換後の SVG 抜粋), (c) `css_purged.svg` (CSS 最適化後の抜粋), (d) `xml_formatted.svg` (XML 整形後の抜粋)。
- **FR-021**: System MUST chromedp を要するテストは `//go:build chromedp` build tag で隔離し、`make test` (既定) では skip、`make test-chromedp` (新規) で chromium ありの環境のみ実行する。CI では Docker chromium ありジョブで `test-chromedp` を回す。
- **FR-022**: System MUST `internal/render/chromedp_fake.go` のようなフェイク `RenderFunc` を test helper として提供し、上位 `engine_test.go` が chromedp 実起動なしで PNG/JPEG 経路を試せるようにする。フェイクは決定論的な小さな PNG/JPEG (1x1 ピクセル) を返してよい。

### Key Entities *(include if feature involves data)*

- **render.Browser**: chromedp `ExecAllocator` をラップした struct。Recycle counter、parent ctx を持つ。`New(opts)` / `NewTab(ctx)` / `Close()` を公開。
- **render.ResizeOpts**: `{Padding []string, Convert string, Scripts []string, BrowserPath string}` 等の入力構造体。
- **render.ResizeResult**: `{Body []byte, Width int, Height int, MIME string}`。
- **render.PipelineHook**: `func(in string) (string, error)` 形式の関数を `engine.Compute` が順番に適用する slice。テスト時は差し替え可能。
- **assets/octicons/data.json**: T-012 で生成済の primer/octicons 抽出 JSON。本 spec の `ReplaceOcticons` がデータ source として読む。
- **engine.Deps (拡張)**: M1 で `{Logger, RESTClient, GraphQLClient}` 等を持っていた構造体に `Render render.RenderFunc` (または `Browser *render.Browser`) を追加。テスト時に DI 経由でフェイク差し替え。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: chromedp あり環境で `engine.Compute(Format:"png")` を呼んだとき、戻り値 `Result.Output` が `image.Decode` で成功する PNG (octocat fixture で `width >= 480, height >= 480`)。constitution 原則 II「DOM 同等 + 動く画像」の最低ラインを満たす。
- **SC-002**: `engine.Compute(Format:"svg")` の戻り値 SVG の `<svg>` ルートが `height="<整数>"` で確定し、`auto` でないテストが緑。
- **SC-003**: `Compute` (chromedp 込み、classic + 主要 10 plugin) の wall time が **5 秒以内** で完走する (constitution 性能目標、`docs/design/10-testing-deployment.md §3`)。`svg.Resize` 単体は **2.5 秒以内** (sleep 2.4 秒 + α)。
- **SC-004**: `svg.Hash` が同 DOM・異 footer の SVG 2 本に対して同一 MD5 を返すゴールデンテストが緑 (output_condition=data-changed 用)。
- **SC-005**: `ReplaceOcticons` が `:octicon-star-16:` を 16px の `<svg class="octicon">` に置換し、最終出力 SVG に octicon プレースホルダ文字列が **0 件残存** することを正規表現で assert。
- **SC-006**: `OptimizeCSS` が固定テストケース (50 セレクタ中 25 が未使用) で **未使用セレクタの 100% 削除** を達成し、`#metrics-end` を含む ID セレクタは purge されない (誤削除 0 件)。
- **SC-007**: chromedp 非依存テスト (`go test ./...`) が chromium 不在環境でも全て緑 (FR-021 の build tag 隔離が機能している)。
- **SC-008**: `internal/render/` パッケージの unit test coverage が **80% 以上**。
- **SC-009**: `lefthook run pre-commit --all-files` 緑、CI 全 job 緑。chromedp ジョブは Docker chromium で 1 ケース以上の SVG → PNG が成功する。

## Assumptions

- chromedp は MIT ライセンスで vendor 可。`go.mod` に `github.com/chromedp/chromedp` (latest stable) を追加する。新規依存追加の根拠は本 spec FR-001/004 で明示済 (constitution 原則 V「依存追加根拠を PR に MUST」)。
- ローカル開発環境では `brew install chromium` または `METRICS_CHROME_PATH=/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` で chromium 確保済みとする。CI は Docker `chromedp/headless-shell` または equivalent イメージを使う。
- twemoji (T-036) と gemoji (T-037) はネットワーク fetch を伴うため、本 spec の P1〜P2 範囲には **含めない**。別ストーリー化または M3 内別 spec に分離し、本 spec では skip 可能 hook として PipelineHook chain に空 stage を予約しておく。
- M4 で実装する plugin (`topics`, `starlists` 等) が chromedp scrape を必要とする (`docs/design/16-tasks-mvp.md N-002`)。本 spec で実装する `render.Browser` インスタンスをそのまま M4 が再利用する設計とし、`Browser` API は plugin scrape 用途にも汎用に保つ MUST。
- `tdewolff/parse/v2/css` および `cascadia` (`andybalholm/cascadia`) は M1 では未導入。本 spec で `go.mod` に追加する。`goquery` (`PuerkitoBio/goquery`) も svg.Hash + CSS purge 両方が使うため本 spec で導入する。
- `assets/octicons/data.json` は M1 の T-012 で生成済みである前提。未生成なら本 spec の前提条件として T-012 を先行実行する。
- M2 で実装した `dispatchOutput` を本 spec で改修する。後方互換性は問題にならない (M2 dispatch.go は明示的に「M3 で置き換える」warn ログを出している)。
- 性能目標 SC-003 を満たすため、chromedp Browser は **複数 Compute 呼び出しで使い回す**。CLI モードでは Compute 完了時に `Browser.Close()` を defer、Action モードでは process lifetime で 1 インスタンス保持する。
- 完了基準は「上記 22 FR + 9 SC が緑」とする。本 spec の範囲外 (twemoji / gemoji / N-002 オンライン scrape) は別途 issue 化する。
