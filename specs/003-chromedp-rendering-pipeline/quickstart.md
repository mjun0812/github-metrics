# Quickstart: chromedp レンダリングパイプライン (M3) 開発手順

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md)

本 spec の実装に着手する開発者向けの初期セットアップ手順。M2 までの開発手順 (`specs/002-output-classic-json/quickstart.md`) を踏まえている前提で、本 spec 固有の差分のみを記載する。

## 1. 必要なツール

### 1.1 chromium バイナリ

`render.Browser` は chromedp 経由で chromium / chrome を起動する。ローカルで chromedp 依存テスト (`make test-chromedp`) を回したい場合に必要。

| OS | 推奨インストール |
|---|---|
| macOS (M1/M2) | `brew install chromium` → `METRICS_CHROME_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium` |
| macOS (既存 Chrome) | `METRICS_CHROME_PATH=/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome` |
| Linux (Debian/Ubuntu) | `sudo apt install chromium` → `METRICS_CHROME_PATH=/usr/bin/chromium` |
| Linux (Fedora/Arch) | `sudo dnf install chromium` / `sudo pacman -S chromium` |
| Docker dev | `chromedp/headless-shell:latest` イメージを `docker run` で使う |

`METRICS_CHROME_PATH` を設定しない場合、chromedp の auto-detect (`google-chrome` / `chromium` / `chromium-browser` を `$PATH` から探す) に委ねる。

### 1.2 primer/octicons (assets 生成時のみ)

`assets/octicons/data.json` を再生成する場合のみ必要。runtime には不要 (生成済 JSON を embed)。

```bash
npm install --no-save @primer/octicons
go run ./internal/tools/gen-octicons \
  -in node_modules/@primer/octicons/build/data.json \
  -out assets/octicons/data.json
```

### 1.3 Go モジュールの追加

本 spec 開発の最初に以下を実行 (`/speckit-tasks` 後に T-001 相当の準備タスクとして単独 commit する想定):

```bash
go get github.com/chromedp/chromedp@latest
go get github.com/PuerkitoBio/goquery@latest
go get github.com/andybalholm/cascadia@latest
go get github.com/tdewolff/parse/v2@latest
go get github.com/tdewolff/minify/v2@latest
go mod tidy
```

## 2. Makefile ターゲット (追加)

`Makefile` に以下を追加する:

```makefile
# 通常テスト (chromedp 不要、CI 既定)
test:
	go test ./...

# chromedp 起動を含むテスト。chromium 同梱環境のみ実行。
test-chromedp:
	go test -tags=chromedp ./...

# assets/octicons/data.json を primer/octicons から再生成
gen-octicons:
	go run ./internal/tools/gen-octicons \
	  -in node_modules/@primer/octicons/build/data.json \
	  -out assets/octicons/data.json

# octicon 生成物が最新か検証 (CI で実行)
verify-octicons: gen-octicons
	git diff --exit-code assets/octicons/data.json
```

## 3. CI ジョブ構成 (追加)

`.github/workflows/go-ci.yml` に以下のジョブを追加する:

```yaml
jobs:
  test-chromedp:
    runs-on: ubuntu-latest
    container:
      image: chromedp/headless-shell:latest
    env:
      METRICS_CHROME_PATH: /headless-shell/headless-shell
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: make test-chromedp
```

通常の `test` ジョブは chromedp 不要なので chromium を含まない。

## 4. ローカル動作確認手順

### 4.1 SVG 経路 (chromedp 不要)

```bash
go test ./internal/render/...
# 通常テスト群: svg_hash, octicon, css, xml_format, padding が緑
```

### 4.2 SVG 経路 (chromedp 含む)

```bash
export METRICS_CHROME_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium
make test-chromedp
# 約 5〜10 秒で svg_resize + chrome 起動テストが緑
```

### 4.3 CLI からの end-to-end 確認

`cmd/metrics-cli` から実物の SVG/PNG を出力する手順 (M6 T-117 で本格対応するが、本 spec 開発中の動作確認用に手動 invoke できる):

```bash
go run ./cmd/metrics-cli \
  --user octocat --template classic --output png \
  --dryrun > /tmp/octocat.png

file /tmp/octocat.png
# /tmp/octocat.png: PNG image data, 480 x 480, 8-bit/color RGBA, non-interlaced
```

dryrun 中も chromedp が起動するため `METRICS_CHROME_PATH` 必須。

### 4.4 FakeRenderer での dryrun

`engine.Deps.Render = &render.FakeRenderer{}` を渡すと chromedp 起動なしで PNG 経路を検証できる:

```go
// 開発者用 ad-hoc スクリプト (本番コードには含めない)
deps := engine.Deps{ /* ... */ Render: &render.FakeRenderer{} }
res, _ := engine.Compute(ctx, engine.Request{Login:"octocat", Template:"classic", Format:"png"}, deps)
// res.Output は 1x1 PNG、res.MIME == "image/png"
```

## 5. デバッグ手順

### 5.1 chromedp の verbose ログ

```go
opts := render.BrowserOpts{
    Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
    Headless: false,  // 開発時のみ。CI / 本番は true。
}
b, _ := render.New(opts)
```

`Headless: false` で起動すると Chrome ウィンドウが表示され、SVG の DOM を DevTools で観察できる。

### 5.2 SVG が `#metrics-end` を持たない場合

`Resize` 段で JS `TypeError: Cannot read properties of null (reading 'getBoundingClientRect')` が出る。classic テンプレートは M2 で `#metrics-end` を末尾に挿入しているはずなので、出力 SVG を `/tmp/dump.svg` に保存して該当要素を grep:

```bash
go test -run TestComputeSVG_Classic ./tests/integration -v 2>&1 | tee /tmp/test.log
# /tmp/test.log 内に "rendered SVG = ..." のログがあれば、grep '#metrics-end'
```

### 5.3 PNG 出力が壊れる

`omitBackground=true` で透明背景 PNG になるため、`open /tmp/octocat.png` で macOS Preview だと「真っ白」に見える。`Quick Look` ではなく `file /tmp/octocat.png` でメタ情報を確認、または `convert /tmp/octocat.png +matte /tmp/octocat-flat.png` (ImageMagick) で背景を白に塗ってから確認する。

## 6. 既知の制約

- chromedp はバージョンによって ABI が壊れることがあるため、本 spec で固定するバージョンは `go.mod` で `replace` せず最新 stable を追従する。CI で `make test-chromedp` が壊れたらまず chromedp のリリースノートを確認。
- `RecycleEvery` 既定 200 は実測根拠ではなく上流互換値。プロファイリング後に短縮可能 (例 50 回毎) なら別 spec で詰める。
- `Sleep(2.4s)` は固定値。短縮するなら `WaitVisible('#metrics-end')` + 動的検出に変える研究が必要 (本 spec 範囲外)。
- chromium 不在環境では `Format=png/jpeg` は **失敗する**。CLI モードで `--output svg` のみ使う運用なら chromium 不要 (`Result.Output` に装飾後 SVG が入る)。

## 7. 関連ドキュメント

- 上流アルゴリズム: [`docs/design/04-rendering.md §3`](../../docs/design/04-rendering.md#3-svg-リサイズ-chromedp)
- 評価スクリプト本体: [`docs/design/13-appendix.md §G`](../../docs/design/13-appendix.md#g-svgresize-の-chromedp-評価スクリプト)
- M2 出力契約 (継承): [`specs/002-output-classic-json/contracts/result-dispatch.md`](../../002-output-classic-json/contracts/result-dispatch.md)
- 全体タスク表 (M3 該当部): [`docs/design/16-tasks-mvp.md`](../../../docs/design/16-tasks-mvp.md) — T-031 / T-032 / T-033 / T-034 / T-035 / T-038 / T-039
