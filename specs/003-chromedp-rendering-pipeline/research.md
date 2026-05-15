# Phase 0 Research: chromedp レンダリングパイプライン (M3)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は M3 spec の Technical Context に含まれる決定事項を整理する。各決定は **Decision / Rationale / Alternatives considered** の三項目で記述する。Phase 0 終了時点で `NEEDS CLARIFICATION` は **0 件**。

## R-001: ヘッドレスブラウザ制御ライブラリの選択

- **Decision**: `github.com/chromedp/chromedp` を採用。
- **Rationale**: 上流 Node 版が puppeteer を使っており、chromedp は CDP (Chrome DevTools Protocol) を直接叩く Go 実装で意味論が一致する。Star 数 11k 以上、Active maintenance、`evaluate` / `setContent` / `screenshot` がすべて 1st-class API として揃っている。alternatives は `playwright-go` だが、Node の playwright driver を子プロセスとして起動する hybrid 実装で Docker サイズ + 起動オーバヘッドが増える。
- **Alternatives considered**:
  - `playwright-go`: hybrid 実装で Node driver を同梱する必要があり、Docker イメージサイズが +120MB。却下。
  - `rod` (`github.com/go-rod/rod`): chromedp と同等の機能を持つが、ドキュメント・採用例が薄く、chromedp の方が安定。
  - `chromem-go` / その他軽量代替: HTML レンダリング機能を持たず、SVG の `getBoundingClientRect()` を取得できない。却下。

## R-002: Browser インスタンスのライフサイクル

- **Decision**: `internal/render.Browser` 構造体で `chromedp.NewExecAllocator` + `chromedp.NewContext` の親 context を保持し、`NewTab(ctx) *Tab` で子 context を切る。N 回 (既定 200) 利用後に `Recycle()` で allocCtx を作り直す。
- **Rationale**: 上流 `lowlighter/metrics` は `svg.resize.browser` 設定 (既定 200) で同じ recycle 戦略を採っている (`docs/design/04-rendering.md §3.2`)。chromedp ExecAllocator はプロセス生成コストが 1〜2 秒かかるため、Compute 呼び出し毎に作るのは性能予算 SC-003 (< 5 秒) を圧迫する。共有 + リサイクルが最適。
- **Alternatives considered**:
  - 呼び出し毎に新規 allocator: 起動オーバヘッドで SC-003 超過、テスト時の DNS / ファイル lock 競合リスク。
  - 永久共有 (recycle なし): メモリリーク / DevTools ハンドル枯渇のリスク。長時間プロセス (M6 Action は数秒〜数十秒だが web インスタンスは長期) に備え recycle 必須。
  - sync.Pool: chromedp の context は `Get/Put` ABI に乗らない (cancellation semantics が壊れる)。却下。

## R-003: chromedp 起動 flag セット

- **Decision**: `--no-sandbox`, `--disable-extensions`, `--disable-dev-shm-usage`, `--disable-gpu`, `--single-process` を既定で渡す。`METRICS_CHROME_PATH` 環境変数が設定されていれば `ExecPath` として尊重する。
- **Rationale**: Docker (Action runner) 環境では `/dev/shm` が小さく `--disable-dev-shm-usage` 必須、root 実行で `--no-sandbox` 必須、ARM 環境では `--single-process` で安定。上流 (`docs/design/04-rendering.md §3.1`) と同じセット。
- **Alternatives considered**:
  - `--headless=new` を追加: chromedp が既定で headless mode を立てるため不要。明示すると古い chromium (M120 未満) で起動失敗。
  - `--remote-debugging-port=0` を固定: chromedp が自動割り当てするので不要。
  - WebSocket 経由で外部 chrome 接続: ローカル開発と Docker で構成が分岐する複雑度に見合うメリットなし。

## R-004: SVG リサイズの計測戦略

- **Decision**: 上流の puppeteer 評価スクリプト (`docs/design/13-appendix.md §G`) を **JS 文字列のまま** chromedp の `chromedp.Evaluate` に渡し、`<svg> #metrics-end` の `getBoundingClientRect()` から `(y, width)` を取得する。`y` を高さ、`width` をそのまま幅とする (`#metrics-end` を SVG 末尾に置く前提)。
- **Rationale**: 計測ロジックを Go 側で書き直すと「実ブラウザの layout 結果」を再現できず DOM 同等性が崩れる。`evaluate` で JS を投げ込む方式は puppeteer/chromedp 間で意味論が同じため、上流互換性 (constitution 原則 II) を担保する最短ルート。
- **Alternatives considered**:
  - chromedp の `dom.GetBoxModel` で BoundingClientRect 取得: SVG element に対する CDP の box model はブラウザによって挙動が異なるため安定しない。
  - Go 側で SVG をパースして CSS 計算: SVG 内の CSS は `@font-face` / `width: 100%` 等を含み、純粋な Go 実装で再現するのは現実的でない。

## R-005: padding パース規則

- **Decision**: `docs/design/13-appendix.md §G.1` の正規表現規則をそのまま実装する。string なら `,` で 2 要素に split、各要素について `([+-]?[\d.]+)%$` で relative を取り出し、残部分の先頭から `([+-]?[\d.]+)` で absolute を取り出す。不足要素は最初の要素にフォールバック。
- **Rationale**: 上流が同じ規則で動いており、`config.padding="0 + 6%, 8 + 11%"` のような既存ユーザー設定 (action.yml input) をそのまま受け入れられる。
- **Alternatives considered**:
  - 独自 DSL に置換 (例 `width=N height=M`): 上流互換性を壊す。constitution 原則 I 違反。
  - JSON / YAML で受ける: 既存 input が文字列なので変更不可。

## R-006: PNG/JPEG 変換実装

- **Decision**: chromedp の `page.CaptureScreenshot` (CDP コマンド) を `clip = {x:0,y:0,width,height}` + `omitBackground=true` で呼び出し、戻り値の bytes をそのまま `Result.Output` に乗せる。MIME は `image/png` / `image/jpeg`。
- **Rationale**: puppeteer の `page.screenshot({clip, type, omitBackground})` と CDP 経由で同等の意味。Go 側で再エンコードしないので image quality が劣化しない。chromedp は標準で `chromedp.CaptureScreenshot(&buf)` ヘルパを提供。
- **Alternatives considered**:
  - SVG → PNG を Go の `golang.org/x/image` 系で変換: SVG レンダリングを自前で持つ必要があり (anti-aliasing, text shaping)、上流のフォント仕上がりに到達できない。却下。
  - rasterize 専用 native binary (Inkscape / resvg) を子プロセスで呼ぶ: 配布バイナリに含めるサイズコスト大、CGO 依存も増える。却下。

## R-007: svg.Hash の DOM 正規化

- **Decision**: `goquery` で `<footer>` 要素を 1 件削除 → `<svg>` ノードの `outerHTML` → `crypto/md5` で MD5 hex (32 文字小文字)。空入力は `("", nil)` を返す。
- **Rationale**: 上流の `svg.hash` アルゴリズム (`docs/design/13-appendix.md §H`) と完全互換。空 input ガードは上流 (`Hash(rendered string) -> null`) の動作を Go 慣習 (`("", nil)`) に翻訳したもの。`goquery` 不在では `golang.org/x/net/html` 直接でも実装可能だが、CSS 内 purge でも `goquery` を使うため 1 ライブラリに集約。
- **Alternatives considered**:
  - `crypto/sha256`: 上流が MD5 なので一致しない。`output_condition=data-changed` の比較相手は上流互換動作が必要。却下。
  - 標準 `encoding/xml` で DOM 操作: footer 検索が SVG XML namespace の壁で複雑化、コードが増える。

## R-008: CSS purge アルゴリズム

- **Decision**: `tdewolff/parse/v2/css` で `<style data-optimizable="true">` 内の CSS をトークン化し、各セレクタを抽出。`goquery` で本文 HTML をパース、`cascadia` でセレクタを照合。マッチ 0 件のセレクタを drop。ID セレクタ (`#xxx`) は無条件で保持 (svg.Resize の `#metrics-end` を消さないため)。残った CSS を `tdewolff/minify/v2/css` で minify。
- **Rationale**: PurgeCSS (Node) の Go 等価動作。`tdewolff/parse` は MIT、stable、Go 製プロダクション CSS ツールの定番。`cascadia` は `goquery` 内部依存と同一なので追加コストなし。
- **Alternatives considered**:
  - `mrcrowl/purify-css-go`: 過去 3 年メンテなし、CSS3 セレクタ未対応。却下。
  - 正規表現で雑に削る: 擬似クラス (`:hover` 等) と擬似要素 (`::before`) を含む CSS を壊す。却下。
  - csso (Node) を child_process で呼ぶ: 配布 Docker に Node runtime を含めることになり、Go 移植の意義を毀損。却下。

## R-009: XML 整形アルゴリズム

- **Decision**: 標準 `encoding/xml` の Token reader で順次トークン化 → `xml.Encoder` に `Indent("", "  ")` を設定して書き戻す独自ヘルパを実装。`xml.Header` は出力しない (SVG 内 XML 宣言は上流互換のため必要ない場合が多い)。空要素 (`<g/>`) はそのまま単一行で出力。
- **Rationale**: Node の `xml-formatter` と同等の意味を、追加依存なしで標準ライブラリだけで達成できる。CDATA / 数値参照 (`&#xE2;`) は標準 encoder が透過に扱う。
- **Alternatives considered**:
  - `mattn/go-xml-formatter`: メンテ不活発、CDATA 周りで escape が壊れる既知バグ。却下。
  - `etree` (`beevik/etree`): 機能十分だが、SVG 巨大 DOM の reparse コストが大きい (5 倍遅い)。標準 ライブラリ で十分。

## R-010: octicon SVG 置換とアセット生成

- **Decision**: `internal/tools/gen-octicons/main.go` を実装し、`primer/octicons` npm パッケージ (`./node_modules/@primer/octicons/build/data.json` または GitHub Release tarball) から **(name, size) → SVG fragment** マッピングを抽出して `assets/octicons/data.json` に書き出す。runtime は `embed.FS` でこの JSON をロードし、`:octicon-<name>(-<size>)?:` を正規表現で見つけて置換。size 省略時は 16px を採用。
- **Rationale**: octicon set は 600+ アイコン、計 200KB 程度。これを Go ソースに hardcode するのは管理コストが大きく、JSON で外出しすれば将来 octicon が増えても tool を再実行するだけで済む。primer/octicons は MIT。
- **Alternatives considered**:
  - octicon を 1 つずつ Go の `string` リテラルで定義: 200 ファイル超の重複が出る。
  - octicon を runtime に HTTP fetch: ネットワーク依存が常時必要になり、Action のオフライン runner で失敗する。却下。
  - octicon-go ライブラリ (`github.com/luozhouyang/go-octicons`): 古いバージョン (5.x) しかない、最新の 19.x 系が未対応。却下。

## R-011: パイプラインの段順序

- **Decision**: `engine.Compute` の Stage 4 (template.Run) 直後に `internal/render/pipeline.Apply(stages, in)` を呼ぶ。順序は (1) `ReplaceOcticons` (常時) → (2) `OptimizeCSS` (入力 `svg.optimize.css=true` のとき) → (3) `FormatXML` (入力 `svg.optimize.xml=true` のとき) → (4) `Resize` (`Format ∈ {svg, png, jpeg}` のとき)。各段の失敗は `Result.Errors` に追記し、本文は **直前段の値** をそのまま次段に渡す best-effort 連鎖。
- **Rationale**: 上流のパイプライン (`docs/design/04-rendering.md §1`) と同じ順序: テンプレート → 装飾 → 最適化 → リサイズ → 出力。装飾→最適化→リサイズ の順は重要で、(a) リサイズ後に octicon を入れると DOM 行高が変わって計測がやり直し、(b) 最適化を octicon 前に行うと未使用と判定された CSS class が後で必要になって壊れる。
- **Alternatives considered**:
  - 並列実行: 各段は string → string の純粋関数チェーンなので並列化メリットなし、依存違反が出る。
  - octicon を template.Run の中で行う: classic / repository / 将来 template で重複実装になる。pipeline 段に集約する方が DRY。

## R-012: テスト戦略 (chromedp 依存隔離)

- **Decision**: `//go:build chromedp` build tag で chromedp 実起動が必要なテスト (`svg_resize_test.go`, `chrome_test.go`, `tests/integration/render_chromedp_test.go`) を隔離。`make test` は通常テストのみ、`make test-chromedp` で chromium 同梱環境のみのテストを実行。chromedp 実起動なしで `engine.Compute(Format:"png")` を試したいケースは `internal/render.FakeRenderer` (決定論的な 1x1 PNG/JPEG を返す) を `Deps.Render` に注入する。
- **Rationale**: 開発者ローカル (chromium 未インストール) でも `go test ./...` が緑になることが、constitution 原則 IV「テスト基盤の維持容易性」を担保する最低条件。CI の chromedp ジョブで 1 ケース以上の SVG → PNG e2e を回せば DOM 同等性は担保される。
- **Alternatives considered**:
  - `httptest` + Selenium IDE 録画再生: 録画が壊れやすい、メンテコスト高。
  - すべてのテストで chromedp 起動を必須化: CI 全 job で chromium dependency が必要になり、PR レビュー速度が落ちる。
  - testcontainers-go で chromium コンテナを ad-hoc 起動: 各 PR で docker pull 必要、現実的でない。

## R-013: chromedp 起動失敗時のフォールバック挙動

- **Decision**: chromedp `Run` のエラーは `*errors.RetryableError` でラップして `Result.Errors` に追加。`Result.Output` には **装飾後だが Resize 前の SVG bytes** をセット (best-effort)。`Request.Die == true` の場合のみ Compute から error を返して短絡する。
- **Rationale**: chromium 不在環境 (ローカル開発者の `go run cmd/metrics-cli`) で「PNG は無理だが SVG なら見られる」状態を返す方が UX が良い。Action 環境では Dockerfile で chromium 同梱を保証するため、本来 fallback には到達しない。
- **Alternatives considered**:
  - 常にエラーを返す: ローカル開発で `cmd/metrics-cli` が動かなくなり、CLI dryrun に支障。却下。
  - SVG bytes を `MIME: image/svg+xml` のまま返す (Format が PNG/JPEG でも): MIME と Output の整合性が壊れる。`MIME: image/png` のまま SVG bytes を返すのも M2 の warn ログと同じ罠なので、PNG/JPEG 経路で chromedp 失敗のときは **InputError として上位に伝播** する方が呼び出し側にとって明確 (M2 dispatch.go との対称性)。 → **再考**: spec の Edge Case と FR-009/FR-018 の整合に従い、PNG/JPEG 経路では chromedp 失敗を `*RetryableError` で `Result.Errors` に追記しつつ、`Result.Output` は空 + `MIME: ""` で返す。呼び出し側は `len(Output)==0` と `Errors` で判断する。SVG 経路では装飾後 SVG を返す。
  - リトライをこの層で 3 回: 上位の `internal/action/retry.go` (T-109) が retry を持つ責務なので、本層では 1 回だけ試す。

## R-014: 性能予算と Sleep(2.4s) の妥当性

- **Decision**: `Sleep(2.4s)` を上流既定値のまま採用。`Compute` 総合 5 秒予算 (SC-003) のうち 50% を chromedp に充てる前提。プロファイリング後に短縮可能なら別 spec で詰める。
- **Rationale**: 上流 puppeteer 実装と同じ wait 値。SVG 内 CSS アニメーション (`<animate>`, `@keyframes`) が安定する時間を実測で詰めた値で、これより短いと layout が確定する前に計測される。
- **Alternatives considered**:
  - `WaitVisible('#metrics-end')` で polling: SVG が initial render される時点では既に visible なので意味なし。
  - `chromedp.RequestAuthRequired` / network idle waiting: SVG は外部リソースを参照しないので idle に到達しない。
  - 短縮 (1.0s): isocalendar 等の長尺アニメーションで計測が早すぎるリスク。後続最適化 spec で計測ベースで詰める。

## R-015: Deps.Render の型 (Renderer interface)

- **Decision**: `engine.Deps` に `Render render.Renderer` フィールドを追加。`Renderer` は 1 メソッド `Resize(ctx, in string, opts ResizeOpts) (ResizeResult, error)` を持つ interface。本番は `*render.Browser` (がメソッド `Resize` を実装) を渡し、テストは `render.FakeRenderer` (固定値返却) を渡す。
- **Rationale**: chromedp 直接依存を engine から消し、テスト容易性を確保する。同じ Browser インスタンスを M4 で `topics` plugin scrape からも使えるよう、Browser 自体には汎用 `NewTab(ctx)` を残しておく (Renderer interface はあくまで engine 用の薄いラッパ)。
- **Alternatives considered**:
  - `Deps.Browser *render.Browser` を直接渡す: テスト時に `Browser` の構造体フィールドをすべてモックする必要があり、ヘルパが膨らむ。interface で 1 メソッド化が最短。
  - `Deps.Render func(ctx, in, opts) (ResizeResult, error)` (関数型): メソッドセット拡張時に互換性が壊れる。interface の方が将来拡張に強い。
