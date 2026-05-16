# Phase 0 Research: GitHub プラグイン群 (M4)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は M4 spec の Technical Context に含まれる決定事項を整理する。各決定は **Decision / Rationale / Alternatives considered** の三項目で記述する。Phase 0 終了時点で `NEEDS CLARIFICATION` は **0 件**。

## R-001: 21 プラグインの実装単位と分割粒度

- **Decision**: 1 プラグイン = 1 Go パッケージ (`internal/plugins/<name>/`)、1 ファイル目に `<name>.go` (実装) + 2 ファイル目に `<name>_test.go` (テーブルテスト) + partial template (`internal/templates/classic/partials/<name>.svg.tmpl`) の三点セットを基本とする。`languages` だけ `recent.go` / `indepth.go` のサブモジュールを追加し、それぞれ `//go:build heavy` で隔離する。
- **Rationale**: M1 で確立した `plugins.Plugin` インタフェース (`Name() / Metadata() / Run()`) が既に最小単位で、これを 21 回反復する形が最も理解しやすく、PR レビューも plugin 単位で完結する。並列開発時の merge conflict も最小化できる。
- **Alternatives considered**:
  - 巨大な単一パッケージ `internal/plugins/github/` に 21 plugin を集約: import cycle のリスクと、PR 単位の変更行数膨張を招く。却下。
  - plugin 機能ごとに横断的に整理 (`internal/plugins/graphql/`, `internal/plugins/rest/`): API 種別と plugin が 1:1 でないため (`achievements` は base 結果再利用のみで API 呼び出しなし、`languages.indepth` は git+enry のみ) 分類が崩壊。却下。

## R-002: 言語分類ライブラリの選択 (linguist 代替)

- **Decision**: `github.com/go-enry/go-enry/v2` (latest stable) を採用。`languages.recent` (T-042) と `languages.indepth` (T-043) で commit diff 内のファイル拡張子 / shebang / コンテンツヒューリスティクスから言語を分類する。
- **Rationale**: 上流 Node 版は `linguist-js` を使っているが、これは GitHub の linguist (Ruby) を JS に再移植したもので、本来 Ruby 実装と完全一致しない。go-enry は **GitHub linguist の Go 公式移植** (src-d 由来、現在は go-enry org) で、`languages.yml` を埋め込み、拡張子・shebang・vendoring 判定がオリジナルと整合している。pure Go なので cgo 不要、クロスコンパイル維持、Star 数 2.5k+、最終更新 2025、現役メンテ。
- **Alternatives considered**:
  - `src-d/enry/v2`: go-enry の旧 org 名。現在 archived、import path を変えるだけで move 済。go-enry に統一。
  - `github.com/getsentry/linguist`: 廃止済 fork。却下。
  - 独自実装 (拡張子マップを手書き): 上流挙動と一致させる工数が大きく (1000+ 言語、vendoring 判定ロジック含む)、constitution 原則 II (出力契約) を守れない。却下。
  - `os/exec` で linguist CLI (Ruby) 呼び出し: Ruby + bundler 依存を Docker に追加することになり、配布バイナリの単一性 (constitution Technical Constraints) を破壊。却下。

## R-003: git クライアントライブラリの選択 (languages.indepth 用)

- **Decision**: `github.com/go-git/go-git/v5` (latest stable) を採用。`languages.indepth` で `git.PlainClone(tempDir, isBare:=false, &git.CloneOptions{Depth:1, URL:...})` 形式の浅い clone を実行し、HEAD のファイル木を `tree.Files()` で走査して各ファイルを go-enry に通す。
- **Rationale**: pure Go、cgo 不要、`os/exec` で git CLI を呼ぶ必要なし。Docker イメージに git バイナリを追加しなくて済む。`Depth:1` shallow clone により大規模 repo でも数秒で済む。go-git は etcd や argo-cd などの実績あり、本 spec の用途 (浅い clone + tree 走査) はライブラリ機能の中心パスで安定。
- **Alternatives considered**:
  - libgit2 (`github.com/libgit2/git2go`): cgo 必須、クロスコンパイル困難 (linux/amd64, linux/arm64, darwin/arm64, windows/amd64 でのビルド対応コスト)。却下。
  - `os/exec` で git CLI 呼び出し: Docker / 配布バイナリに git 同梱が必要。runtime exec のエラーハンドリングが複雑。却下。
  - GitHub Trees API (`GET /repos/X/Y/git/trees/...`) で tree だけ取得: ファイル内容まで取得すると 1 req/file になり rate limit を即枯渇させる。tree のみだとファイル内容を見ない言語推定になり精度が落ちる。却下。

## R-004: chromedp 経路 plugin (topics / starlists) の Browser 共有戦略

- **Decision**: `topics` / `starlists` plugin は `pc.Imports.Get("render")` のような新規 import 経由ではなく、`engine.Compute` 側で構築済の `*render.Browser` (M3 で導入) を `Deps.Render` から **直接参照** する。各 plugin は `Deps.Render` interface を実装する型に対し型 assertion で `*render.Browser` を取り出し、`browser.NewTab(ctx)` でタブを開く。
- **Rationale**: M3 で `Deps.Render` は `Renderer` interface (`Resize(ctx, in, opts)`) として既に engine から見えている。型 assertion 1 行で `*render.Browser` を露出させれば新規依存を作らずに済む。Browser の Recycle (M3 R-002) もそのまま動く。型 assertion は internal package 同士で完全に閉じる (外部公開なし)。
- **Alternatives considered**:
  - 新規 plugin ごとに `chromedp.NewContext` を直接生成: M3 で確立した Recycle が効かず、Compute 内で chromedp プロセスが 3 つ (M3 svg.Resize + topics + starlists) 生まれて性能予算 (SC-003) 圧迫。却下。
  - `Renderer` interface に `NewTab(ctx) (Tab, func())` を追加: M3 の最小 interface を肥大化させ、FakeRenderer (M3 で導入) も追従改修が必要。却下。
  - chromedp scrape を別バイナリに分離: M6 Action は 1 バイナリ前提 (constitution Technical Constraints)。却下。

## R-005: scope 不足検出と `skipped` フラグ

- **Decision**: `pc.REST.Scopes() []string` (M1 で `internal/githubapi/rest.go` に存在を想定。なければ M4 で `scopes.go` を新規追加) で token に含まれる scopes を取得し、各 plugin が `requiredScopes` (定数) と比較して不足を判定する。不足時は `data.Plugins[<name>] = &Result{Skipped: true, SkippedReason: "..."}` を書き、WARN ログ 1 行、`Result.Errors` には追加しない。
- **Rationale**: 上流 Node 版は同じ挙動 (`docs/design/06-plugins-detail.md §2.17 / §2.21 / §2.28`)。scope は呼び出し側 (Action 利用者の PAT 構成) の責任であり、エラーとして扱うと CI が壊れるので致命でなく degraded path を提供する。
- **Alternatives considered**:
  - scope 不足を `Result.Errors` に追加: PR を切る・data-changed mode を破壊するため上流挙動と非互換。
  - 起動時に scope を一括検証して足りない plugin を Run から除外: token の rotation を考慮すると runtime 判定の方が安全。
  - GitHub の `GET /` で `X-OAuth-Scopes` header を毎回読む: 1 リクエスト目で一度だけ読んでキャッシュする方が効率的。`pc.REST.Scopes()` がキャッシュ済 helper を提供する前提。

## R-006: organization branch + indepth + repositories paging の base 拡張

- **Decision**: M1 で `base.go` に user branch のみ実装済の `base` プラグインを、M4 で以下 3 つの未完部分を完成形に拡張する:
  1. **organization branch** (T-018): `organization.members.nodes(first:N)` を `plugin_repositories_batch` 既定値でページングし、`Data.Organization.Members` に集計する
  2. **indepth クエリ** (T-019): `plugin_repositories=true` かつ indepth 系フラグ (`plugin_repositories_pinned`, `plugin_followup`, `plugin_isocalendar` 等) のいずれかが真のとき、commits / issues / pull-requests の追加 GraphQL クエリを発行
  3. **repositories paging** (T-020): 100 件超の repositories を `after` cursor で再帰取得、batch-halving (上流 `app/metrics/utils.mjs::nearestRepositoriesBatchSize` 由来) で 502 / timeout 時に batch を半分にしてリトライ
- **Rationale**: 多くの M4 plugin (`repositories`, `languages.recent`, `traffic`, `contributors`, `notable`, `stargazers` 等) が `Data.Computed.Repositories` を消費する前提なので、これらが用意できないと plugin 個別の実装が空転する。M2 で `base` 実装をスケルトンにとどめたのは「M4 で完成形に拡張する」前提があったため (M2 plan.md の付記参照)。
- **Alternatives considered**:
  - 各 plugin が自前で `repositories` ページングを実装: コード重複 + 並列 fetch で rate limit を即枯渇。却下。
  - `repositories` 取得を別 plugin に切り出す: 既存の `base` を分割すると M1/M2 のテスト互換性を破壊。却下。
  - organization 分岐を `runOrganization` plugin に分離: account kind による分岐は `base.Run` 内の自然な構造なので分けない方がシンプル。

## R-007: 並列実行と `pc.Data.Plugins` の排他制御

- **Decision**: M2 で実装済の `core.RunPlugins` (`internal/plugins/core/run_plugins.go`) は既に `pc.Data.Plugins` への書き込みを mutex で排他化している。M4 では plugin 側で `pc.Data` を mutate しない (返り値で表現する) ことを **規約として徹底** する。`pc.Data.Computed` だけは base が書き込み、他 plugin は read-only でアクセスする。
- **Rationale**: M2 で実装済みの並列基盤を再利用するのが最もリスクが低い。plugin 内で `pc.Data` を直接 mutate すると、core の mutex が効かず data race の温床になる。
- **Alternatives considered**:
  - 各 plugin に専用 mutex を持たせる: 21 個に分散すると排他粒度がバラバラになり、core で集約する方が安全。却下。
  - 直列実行 (並列化を捨てる): SC-003 (< 5 秒) を達成できない。GraphQL は単独で 1 リクエスト 200ms 程度かかるため、直列だと 21 plugin で 4 秒は普通に超過。却下。

## R-008: classic テンプレートと partial の組み立て規約

- **Decision**: classic テンプレートのトップレベル (`internal/templates/classic/classic.go`) で `pc.Inputs` の `plugin_<name>` フラグを順に評価し、true の plugin に対応する partial template (`partials/<name>.svg.tmpl`) を Go の `text/template` で render → 結果を `<g class="<name>">` でラップして concat する。partial 同士の DOM は独立。partial test は plugin ごとに `tests/golden/classic/m4/<name>.svg` と比較する。
- **Rationale**: M2 で classic SVG の最上位構造 (`<svg> <foreignObject><div class="items-wrapper">...</div></foreignObject></svg>`) は確定している。各 partial は `<div class="..."` の中身を返すだけのシンプルな responsibility。partial を 1 ファイル = 1 plugin に対応させると、新規 plugin 追加時に他 partial を touched せずに済む (constitution 原則 III の scope 規律にも合う)。
- **Alternatives considered**:
  - 単一巨大 `classic.svg.tmpl` に 21 partial を inline 記述: ファイル肥大化 + plugin 追加時に必ず classic.svg.tmpl の merge conflict 発生。却下。
  - partial を Go コード生成 (`fmt.Sprintf` + 構造体タグ): デザイン変更時の編集容易性が `text/template` より大幅に劣る。却下。

## R-009: テスト戦略 (build tag 分離)

- **Decision**: M3 で確立した `//go:build chromedp` を `topics` / `starlists` に踏襲。`//go:build heavy` を **新規導入** し、`languages.recent` / `languages.indepth` を隔離する。Makefile に `test` (デフォルト、chromedp も heavy も除外) / `test-chromedp` (M3 既存) / `test-heavy` (新規) の 3 ターゲットを作る。CI は 3 ジョブを並列実行 (`test` + `test-chromedp` + `test-heavy`)。
- **Rationale**: 通常開発で go-enry の言語マッピング DB (`internal/data/embedded.go` 由来、約 6MB) を毎回ロードするのは I/O コスト。go-git の clone を試すと外部ネットワーク (mocked fixture でも fake server を起動) が増える。これらを通常 CI から切り離せば `go test ./...` の wall time を 1 桁短く保てる。
- **Alternatives considered**:
  - 単一 build tag で chromedp + heavy をまとめる: 通常開発でどちらかだけ走らせたい局面で不便。CI でも chromedp jobs と heavy jobs を分離した方が並列度が上がる。
  - 環境変数 (`METRICS_TEST_HEAVY=1`) で skip: build tag の方が `go test -tags=heavy` 一行で明示的、Makefile も統一感が出る。

## R-010: 上流互換性テストの完成形フィクスチャ

- **Decision**: `tests/fixtures/upstream/octocat.json` (M2 で sync-fixtures が生成) を M4 完了時に **採用 21 plugin 分すべて含むフルバージョン** に再生成する。同じ `sync-fixtures` ツールを `--full` フラグ付きで再実行し、得られた JSON を vendor。本 spec の SC-004 (key diff 0) の判定対象とする。
- **Rationale**: M2 fixture は base + core + 一部 plugin (3 個程度) しか含まれておらず、M4 で 21 plugin 分追加した状態の互換性を検証できない。`sync-fixtures` はすでに M2 で動いており、フラグ追加程度の差分で済む。
- **Alternatives considered**:
  - plugin 単位で個別の `tests/fixtures/upstream/<plugin>.json` に分離: 1 octocat に対する 21 個の fixture が同一の base data を重複保持することになる。disk waste。
  - 上流互換性テストを skip し、SC-004 を緩める: constitution 原則 II の核心を放棄することになる。却下。

## R-011: `Result.Skipped` フラグの JSON 表現

- **Decision**: 各 plugin の Result 構造体に `Skipped bool ` json:"skipped,omitempty"` ` フィールドを追加する。`skipped=true` のとき他フィールドは空 (zero value)。上流互換性: 上流 metrics.json も同名のキーで同じ意味を表現している (`docs/design/06-plugins-detail.md §2.28 traffic` 等の skipped pattern)。
- **Rationale**: 上流挙動との完全互換が constitution 原則 II の核。`omitempty` により skipped=false (正常) の plugin では JSON 出力に `skipped` キーが現れない (上流挙動)。
- **Alternatives considered**:
  - 別途 `Error string` フィールドで「skip 理由」を文字列保持: 上流は `skipped: true` 単独で表現しているのでキー追加は互換性を破る。却下。
  - skipped 時に `data.Plugins[<name>]` キー自体を欠落させる: 上流は `skipped` フィールドを伴う object を出力するので、キー欠落は互換性違反。却下。

## R-012: `worldmap` / `stargazers` の地理 API スコープアウト

- **Decision**: `stargazers` plugin の `plugin_stargazers_worldmap=true` 経路 (Google Maps Geocoding API で stargazer の location → 緯度経度) は **M4 では実装しない**。`data.Plugins.stargazers.worldmap = nil` を返し、worldmap セクションだけ SVG partial から省く。warning ログ 1 行で「worldmap is not yet implemented in M4」を出す。
- **Rationale**: Google Maps Geocoding は外部 API key (`plugin_stargazers_worldmap_token`) を要求し、`docs/design/15-selection-answer.md §6` の採用判断では「採用するが M4 では未実装、後続 N-task で実装」と整理されている。本 spec は採用 21 plugin の「動く骨格」を最優先するため、worldmap は除外して MVP を圧縮する。
- **Alternatives considered**:
  - worldmap を M4 内で実装: Google Maps API のレート制限・課金・request 戦略 (sampling) の設計が独立した題材になり M4 のスコープを膨張させる。却下。
  - `stargazers` plugin 全体を P3 に格下げ: 基本 chart (chromedp 不要) は P2 ですぐ動くため、全体格下げは無駄。worldmap だけ NTask 化が最適解。

## R-013: 性能予算 (21 plugin 並列下の wall time)

- **Decision**: `Compute` フルパス (21 plugin、mocked deps) p95 < 5 秒、各 plugin 単体 p95 < 1 秒。`languages.indepth` のみ 7.5 分 / repo × 全体 15 分 (上流既定値) の特例。benchmark を `internal/engine/bench_full_test.go` に追加し PR 本文で実測値報告。
- **Rationale**: SC-003 と constitution Technical Constraints の整合。21 plugin 並列なら理論値で max(各 plugin 時間) なので、各 1 秒なら全体 ~1-2 秒で済むはず。`languages.indepth` だけ shallow clone 時間が変動するため別予算が必要。
- **Alternatives considered**:
  - 全 plugin 同一の 1 秒予算: indepth に対し非現実的。実 git clone の所要時間は repo サイズ依存で、上流既定 7.5 分は妥当な上限。
  - 予算を緩めて 10 秒: GitHub Action の typical timeout (6 min) には十分余裕があるが、ユーザー体感 (README badge 表示まで) の劣化を招く。
