# 14. 機能選定ガイド

「何を移行するか」を **あなたが今から決める** ための判断資料です。
本文書は **「どれを選んでも壊れない最小依存集合がわかる」** ようにカタログ化しています。
最終的には [§9 選定ワークシート](#9-選定ワークシート) を埋めてもらえば、必要な T-XXX タスク集合が確定します。

## 目次

- [1. 使い方](#1-使い方)
- [2. 判断軸 (5 つの大きな選択)](#2-判断軸-5-つの大きな選択)
- [3. 機能カタログ — エントリポイント](#3-機能カタログ--エントリポイント)
- [4. 機能カタログ — 出力形式](#4-機能カタログ--出力形式)
- [5. 機能カタログ — テンプレート](#5-機能カタログ--テンプレート)
- [6. 機能カタログ — プラグイン (42 個)](#6-機能カタログ--プラグイン-42-個)
- [7. 機能カタログ — 周辺機能](#7-機能カタログ--周辺機能)
- [8. 依存早見表](#8-依存早見表)
- [9. 選定ワークシート](#9-選定ワークシート)
- [10. 参考プロファイル例](#10-参考プロファイル例)

---

## 1. 使い方

1. [§2](#2-判断軸-5-つの大きな選択) で **大きな 5 つの判断** を済ませる (エントリ点 / 出力形式 / テンプレート / プラグイン / 周辺機能)。
2. [§3〜§7](#3-機能カタログ--エントリポイント) の各機能カタログを読み、`採用` / `不採用` / `保留` をメモする。
3. 各機能カードの **必要タスク** / **必須依存** / **任意依存** / **外部依存** / **コスト** / **スキップ時の影響** を見て判断。
4. [§9 選定ワークシート](#9-選定ワークシート) を埋める → 必要タスク集合と除外可能タスクが確定する。
5. [§8 依存早見表](#8-依存早見表) で「これを抜くと何が壊れるか」を最終チェック。

### 1.1 用語

- **必要タスク** — その機能を作るために必須の T-XXX 集合 (`12-tasks.md`)。
- **必須依存** — その機能を採用するなら **必ず一緒に採用** しないと動かない他機能。
- **任意依存** — 一緒に採用すると体験が向上するが、なくても動く他機能。
- **外部依存** — Chrome 等の同梱物、API 鍵、ネットワーク到達性、ディスク I/O。
- **コスト目安** — 実装ボリュームの肌感: **S**=数時間〜1 日、**M**=2〜3 日、**L**=1 週間、**XL**=2 週間以上。
- **スキップ時の影響** — 採用しなかった場合、どの他機能が連動して使えなくなるか。

### 1.2 必ず採用すべき "土台"

下記は **どの機能を選んでも必須** なので、選定対象から外して固定する:

| 区分 | タスク |
|------|--------|
| 基盤一式 | T-001 (repo init), T-002 (logger/errors), T-013 (formatters), T-007 (HTTP client) |
| 設定/メタデータ | T-003 (settings), T-004 (metadata.yml), T-005 (inputs), T-012 (embed.FS) |
| GitHub クライアント | T-008 (REST), T-009 (GraphQL), T-010 (rate tracker) |
| Engine 核 | T-014 (Plugin I/F), T-015 (Template I/F), T-016 (Compute), T-021 (core plugin), T-022 (並列実行) |
| base プラグイン | T-017, T-018, T-020 (user/org/repositories ページング) |

→ 14 タスク、目安 **3〜4 週間** がスタート最小投資。

---

## 2. 判断軸 (5 つの大きな選択)

最初に下記の 5 個だけ決めれば、残りの取捨は機械的に絞り込めます。

### Q1: 起動形態は?

| 選択肢 | 内容 | 該当 §3 |
|------|------|---------|
| (a) Action のみ | GitHub Actions workflow から `uses: <repo>` で呼び出す形だけ | §3.1 |
| (b) Web インスタンスのみ | 自前サーバを立てて URL アクセス | §3.2 |
| (c) CLI のみ | ローカル開発用に手動実行 | §3.3 |
| (d) Action + Web 両方 | (a) + (b) | §3.1 + §3.2 |

→ 大半のユーザは **(a) Action のみ** で十分なはず。

### Q2: 出力形式はどれが必要か?

| 形式 | 必要なもの | 該当 §4 |
|------|-----------|---------|
| SVG | chromedp (高さ計測のため) ※軽量版もあり | §4.1 |
| PNG / JPEG | chromedp 必須 (スクリーンショット) | §4.2 |
| JSON | (なし) — 一番安い | §4.3 |
| Markdown | base + 主要プラグイン + EJS 評価 | §4.4 |
| Markdown-PDF | chromedp + PDF レンダリング | §4.5 |
| Insights (HTML) | Web インスタンス必須 (自己 fetch) | §4.6 |

→ "SVG だけで十分" なら chromedp が必要、"JSON だけ" なら chromedp は不要。

### Q3: テンプレートはどれを使う?

| テンプレート | 用途 | 該当 §5 |
|------------|------|---------|
| classic | 個人/組織アカウントの定番 | §5.1 |
| repository | リポジトリ向け | §5.2 |
| terminal | SSH 風 (見た目重視) | §5.3 |
| markdown | Markdown ファイル生成 | §5.4 |
| community | 第三者作成テンプレ | §5.5 (初版非推奨) |

→ "classic のみ" が最小。partial 数は 46 個あるので、後述のとおり **使うプラグインの partial だけ** 実装すれば良い。

### Q4: プラグインはどれを使う?

[§6](#6-機能カタログ--プラグイン-42-個) の 42 個から **採用するものだけチェック**。
ヒント:

- **README 用バッジ** だけが目的なら 3〜5 個で足りる (例: languages, activity, isocalendar, achievements, repositories)。
- **ダッシュボード総覧** 用途なら 10〜15 個。
- **全部入り** はオリジナル仕様だが、移行初版で 42 全部は重い。

### Q5: 周辺機能はどこまで?

[§7](#7-機能カタログ--周辺機能) から判断:

- **OAuth** (Web 版でユーザ別 token を扱う) — Web を立てるなら検討、Action 専用なら不要。
- **PR / Gist / commit** 出力アクション — Action なら README コミットに必要、Web には無関係。
- **presets** — 利便性機能。後回しでも OK。
- **community templates / community plugins** — 初版で見送り推奨。
- **SVG 最適化 (SVGO)** — オリジナルでも experimental。初版省略可。

---

## 3. 機能カタログ — エントリポイント

### 3.1 GitHub Action として動く

| 項目 | 内容 |
|------|------|
| 必要タスク | T-105〜T-117 (12 個) |
| 必須依存 | "土台 14 タスク" + 出力形式 1 つ以上 + テンプレ 1 つ以上 |
| 任意依存 | committer (commit/PR/gist) — README に書き込みたいなら必須 |
| 外部依存 | `INPUTS` 環境変数経由で実行される前提、`/renders` ボリューム |
| コスト | **L** (1 週間〜10 日) |
| スキップ時 | GitHub Actions から使えなくなる |

### 3.2 Web インスタンスとして動く

| 項目 | 内容 |
|------|------|
| 必要タスク | T-094〜T-103 (10 個) |
| 必須依存 | "土台" + キャッシュ (T-011) + Compute 全般 |
| 任意依存 | OAuth (T-100)、Control endpoint (T-101) |
| 外部依存 | listen ポート (3000)、httprate, chi |
| コスト | **L** |
| スキップ時 | Web からアクセスできない (insights HTML も生成不可) |

### 3.3 CLI 単独実行 (`metrics-action --config inputs.yaml`)

| 項目 | 内容 |
|------|------|
| 必要タスク | T-117 (1 個。ただし T-105/106 と密結合) |
| 必須依存 | "土台" + 出力形式 + テンプレ |
| 外部依存 | なし (ローカル実行) |
| コスト | **S** (Action の入力経路を CLI flag 化するだけ) |
| スキップ時 | ローカルデバッグ・スクリプト化ができない |

---

## 4. 機能カタログ — 出力形式

### 4.1 SVG

| 項目 | 内容 |
|------|------|
| 必要タスク | T-031 (chromedp), T-032 (svg.Resize), 任意で T-034 (CSS 最適化), T-036/37/38 (絵文字置換) |
| 必須依存 | chromedp ランタイム |
| 外部依存 | Chrome / Chromium バイナリ (Docker に同梱) |
| コスト | **M** |
| スキップ時 | SVG が出力できない (= ほぼ全機能が成立しない) |

> **メモ**: chromedp 抜きで SVG を返すことも技術的には可能だが、`height` 属性を実測できないため見栄えが崩れる。実用するなら chromedp は必須。

### 4.2 PNG / JPEG

| 項目 | 内容 |
|------|------|
| 必要タスク | T-039 (PNG/JPEG 統合) |
| 必須依存 | §4.1 SVG |
| 外部依存 | chromedp |
| コスト | **S** (svg.Resize の output を screenshot するだけ) |
| スキップ時 | バッジ画像として PNG/JPEG が選べない (SVG のみ) |

### 4.3 JSON

| 項目 | 内容 |
|------|------|
| 必要タスク | T-029 (JSON 出力) |
| 必須依存 | "土台" のみ (chromedp 不要) |
| 外部依存 | なし |
| コスト | **S** |
| スキップ時 | 機械処理 (Insights / 自前ツール) が不能 |

> **最も軽い**。chromedp が要らないので「JSON だけ出せる軽量 CLI」を作る選択も可。

### 4.4 Markdown

| 項目 | 内容 |
|------|------|
| 必要タスク | T-091 (markdown template) |
| 必須依存 | EJS 評価 + base 系 + 採用したプラグイン群 |
| 任意依存 | T-113 (markdown キャッシュ commit) — Action で画像を README に貼るなら必須 |
| 外部依存 | 取得元 (`q.markdown`) のネットワーク到達性 |
| コスト | **L** (EJS の 2 パス評価が重い) |
| スキップ時 | Markdown ファイルベースの動的レンダリングができない |

### 4.5 Markdown-PDF

| 項目 | 内容 |
|------|------|
| 必要タスク | T-040 (PDF) + T-092 (markdown-pdf 統合) |
| 必須依存 | §4.4 Markdown + chromedp |
| 外部依存 | Chromium、フォントセット (CJK 含む) |
| コスト | **M** |
| スキップ時 | PDF 出力ができない |

### 4.6 Insights (HTML ダッシュボード)

| 項目 | 内容 |
|------|------|
| 必要タスク | T-030 (Compute Insights), T-099 (insights endpoint), T-116 (Action 経由起動) |
| 必須依存 | §3.2 Web instance + chromedp + 固定セットの主要プラグイン |
| 外部依存 | 自分自身への HTTP fetch (localhost) |
| コスト | **XL** (固定 13 プラグインがすべて必要) |
| スキップ時 | 対話型ダッシュボードが見られない |

> **重い**。初版では省略推奨。

---

## 5. 機能カタログ — テンプレート

### 5.1 classic

| 項目 | 内容 |
|------|------|
| 必要タスク | T-023 (skeleton) + T-024〜T-028 (5 つの base 系 partial) + **採用プラグイン分の partial** |
| supports | user / organization |
| formats | svg / png / jpeg / json |
| 必須依存 | base プラグイン (土台) |
| コスト | **M** (skeleton + 5 partial) + プラグインごとに **S〜M** |
| スキップ時 | 最も汎用のテンプレが使えない |

> partial は 46 個あるが、**採用したプラグインの partial だけ実装すれば動く**。残りは "存在しないキーで空文字列を返す" 仕様なので silently skip される。

### 5.2 repository

| 項目 | 内容 |
|------|------|
| 必要タスク | T-089 |
| supports | repository |
| 必須依存 | repository 用 partial 群 |
| コスト | **M** |
| スキップ時 | リポジトリ単体メトリクスができない |

### 5.3 terminal

| 項目 | 内容 |
|------|------|
| 必要タスク | T-090 |
| supports | user / organization |
| 必須依存 | classic と同じ partial 集合 |
| コスト | **S** (classic を流用、見た目だけ変える) |
| スキップ時 | SSH 風の見た目選択がなくなる (機能差は CSS のみ) |

### 5.4 markdown

| 項目 | 内容 |
|------|------|
| 必要タスク | T-091, T-092 |
| supports | user / organization / repository |
| formats | markdown / markdown-pdf / json |
| 必須依存 | EJS 2 パス評価 + `embed()` 関数 |
| コスト | **L** |
| スキップ時 | §4.4 Markdown が使えない |

### 5.5 community (動的 git clone)

| 項目 | 内容 |
|------|------|
| 必要タスク | T-093 (loader skeleton のみ) |
| 必須依存 | git clone, テンプレート JS 実行 (将来は WASM) |
| コスト | 初版は **無効化のみ実装** (S) |
| スキップ時 | サードパーティテンプレが使えない |

> **初版省略推奨**。loader だけ用意して動的取得は warning で skip。

---

## 6. 機能カタログ — プラグイン (42 個)

各プラグインの "**個別カード**" 形式。
**コスト** は本体実装 + テスト + partial 移植まで含む目安。
**外部依存** に "GitHub" だけ書いてあるものは、追加の API トークン不要 (PAT のみ)。

### 6.1 必須コア (常に採用)

| プラグイン | タスク | 必須依存 | コスト | 役割 |
|----------|-------|---------|-------|------|
| `base` | T-017, T-018, T-019, T-020 | "土台" | **L** | ユーザ/組織/リポジトリ基本データ |
| `core` | T-021, T-022 | 同上 | **M** | 設定注入と plugins 並列実行 |

### 6.2 GitHub データ系 (28 個)

⭐ = 採用率の高いおすすめ。

| プラグイン | タスク | 必須依存 | 外部依存 | コスト | 主な出力 | 採用検討ポイント |
|----------|-------|---------|---------|-------|---------|-----------------|
| ⭐ `languages` (基本) | T-041 | base | GitHub | **M** | 言語使用比率 | ほぼ必須級の見栄え |
| `languages.recent` | T-042 | T-041 | REST events + commit diff | **M** | 最近使用言語 | API 消費大 |
| `languages.indepth` | T-043 | T-041, git clone | git + go-enry | **L** | 行数まで含む詳細 | CPU/ディスク負荷大、extras 必要 |
| ⭐ `activity` | T-044 | base | REST events | **S** | 最近のアクティビティ | API 1 種で済み軽い |
| ⭐ `achievements` | T-045 | base | GitHub or chromedp | **M** | バッジ表示 | 見た目派手で人気 |
| ⭐ `repositories` | T-046 | base | GitHub | **S** | featured/pinned 表示 | base.repositories と別 |
| `lines` | T-047 | base | REST stats | **M** | 追加/削除行数 | REST stats を多用、レート消費注意 |
| `gists` | T-048 | base | GraphQL | **S** | Gist 統計 | 軽い、good-first-issue |
| ⭐ `isocalendar` | T-049 | base | GitHub | **M** | 3D 等尺カレンダー | 見た目人気 |
| `calendar` | T-050 | base | GitHub | **M** | 複数年カレンダー | 年数指定でクエリ増 |
| `habits` | T-051 | - | REST events + commit diff | **M** | 曜日/時間帯傾向 | API 消費大 |
| `stars` | T-052 | base | GraphQL | **S** | 最近スターしたリポジトリ | 軽い |
| `topics` | T-053 | chromedp | chromedp scrape | **M** | スターしたトピック | chromedp 必須、extras 必要 |
| `starlists` | T-054 | T-053, T-041 | chromedp | **L** | Star Lists 表示 | 大きい |
| `people` | T-055 | base | GraphQL | **M** | フォロワー一覧等 | 種類が多い |
| `notable` | T-056 | base | GitHub | **M** | 注目コントリビューション | indepth で API 重い |
| `followup` | T-057 | base | GraphQL search | **S** | open/closed 比 | |
| `discussions` | T-058 | base | GraphQL | **S** | Discussions 統計 | |
| `contributors` | T-059 | base | REST | **M** | リポジトリ貢献者 | repository テンプレ向け |
| `code` | T-060 | - | REST events | **M** | ランダムコード片 | プライベートコード漏洩リスクあり |
| `introduction` | T-061 | base | GitHub | **S** | bio/description | good-first-issue |
| `reactions` | T-062 | base | GraphQL | **M** | リアクション集計 | |
| `projects` | T-063 | - | GraphQL | **M** | GitHub Projects | `read:project` scope 必要 |
| `sponsors` | T-064 | - | GraphQL | **S** | スポンサー紹介 | `read:user`, `read:org` 必要 |
| `sponsorships` | T-065 | - | GraphQL | **S** | スポンサーシップ履歴 | 同上 |
| `stargazers` | T-066 | base | GraphQL (+ optional Google Maps) | **L** | スターチャート/世界地図 | 世界地図は Google API key 必要 |
| `skyline` | T-067 | chromedp | skyline.github.com | **L** | 3D city/skyline | 重い、GIF アニメ |
| `traffic` | T-068 | base | REST traffic | **S** | アクセス数 | `repo` scope 必要 |
| `support` (deprecated) | T-069 | chromedp | chromedp | **S** | 旧 Support 統計 | 上流終了済 |

### 6.3 ソーシャル/外部 API 系 (10 個)

`(API key)` = ユーザがトークン/鍵を用意する必要あり。

| プラグイン | タスク | 外部依存 | コスト | 採用検討ポイント |
|----------|-------|---------|-------|----------------|
| `wakatime` | T-070 | WakaTime API (key) | **M** | 利用者多い |
| `pagespeed` | T-071 | Google PageSpeed (key) | **S** | ウェブサイト持ちのみ |
| `posts` | T-072 | dev.to / hashnode / medium | **S** | ブロガー向け |
| `rss` | T-073 | 任意 RSS | **S** | good-first-issue |
| `stackoverflow` | T-074 | Stack Exchange (key) | **S** | SO 利用者向け |
| `leetcode` | T-075 | LeetCode | **S** | コーディング学習者向け |
| `anilist` | T-076 | AniList | **S** | アニメ好き向け |
| `music` | T-077 | Spotify/Apple/LastFM (key/OAuth) | **L** | プロバイダ複数で複雑 |
| `steam` | T-078 | Steam Web API (key) | **M** | ゲーム好き向け |
| `tweets` (deprecated) | T-079 | Twitter API v2 (paid) | **S** | 上流有料化 |

### 6.4 community 系 (9 個、初版は省略可)

| プラグイン | タスク | 外部依存 | コスト |
|----------|-------|---------|-------|
| `crypto` | T-080 | CoinGecko | **S** |
| `nightscout` | T-081 | Nightscout | **S** |
| `stock` | T-082 | Alpha Vantage (key) | **S** |
| `chess` | T-083 | chess.com | **S** |
| `splatoon` | T-084 | splatoon3.ink 等 | **S** |
| `fortune` | T-085 | (内蔵) | **S** |
| `poopmap` | T-086 | poopmap.com | **S** |
| `screenshot` | T-087 | chromedp | **S** |
| `16personalities` | T-088 | chromedp scrape | **S** |

> **初版省略推奨**。需要が出てから個別に追加できる。

---

## 7. 機能カタログ — 周辺機能

### 7.1 出力アクション (Action 専用)

| 機能 | タスク | コスト | 必要性 |
|------|-------|--------|-------|
| なし (`output_action=none`) | (T-109 のファイル書出のみ) | - | 必須 |
| `commit` (リポジトリへ commit) | T-110 | **M** | README バッジ用には必須 |
| `pull-request` (+merge/squash/rebase) | T-111 | **M** | レビューを挟むなら |
| `gist` | T-112 | **S** | 軽い別領域 |
| `markdown` キャッシュ commit | T-113 | **M** | Markdown 出力使うなら必須 |
| `data-changed` 判定 | T-114 | **S** | 無駄 commit を防ぐ |
| Workflow run cleanup | T-115 | **S** | 任意 |

### 7.2 認証 (Web 専用)

| 機能 | タスク | コスト | 必要性 |
|------|-------|--------|-------|
| OAuth フロー | T-100 | **M** | ユーザ別 token / extras.logged を扱うなら |
| Control endpoint (`/.control/stop`) | T-101 | **S** | 運用ツール |
| Restricted users (allow-list) | T-098 内 | (含む) | 多人数共有時のセキュリティ |
| Rate limit middleware | T-095 | (含む) | パブリック公開時は必須 |

### 7.3 入力支援

| 機能 | タスク | コスト | 必要性 |
|------|-------|--------|-------|
| `config_presets` | T-006 | **S** | 任意。利便性機能 |
| 動的プレースホルダ (`.user.login`) | T-005 内 | (含む) | metadata default を活かすなら必須 |

### 7.4 出力最適化

| 機能 | タスク | コスト | 必要性 |
|------|-------|--------|-------|
| CSS purge + minify | T-034 | **M** | SVG サイズ削減 (推奨だが必須ではない) |
| XML 整形 | T-035 | **S** | 任意 |
| SVG 最適化 (SVGO 相当) | (実装) | **L** | 初版省略 ok |
| 絵文字 (twemoji) | T-036 | **M** | classic フッタ等で利用 |
| 絵文字 (gemoji `:name:`) | T-037 | **S** | 一部 plugin で使用 |
| octicons (`:octicon-X-Y:`) | T-038 | **S** | 多数の partial で使用、推奨 |

### 7.5 mocking / テスト

| 機能 | タスク | コスト | 必要性 |
|------|-------|--------|-------|
| REST mock | T-118 | **M** | テスト必須 |
| GraphQL mock | T-119 | **M** | 同上 |
| golden test framework | T-120 | **M** | 同上 |

---

## 8. 依存早見表

「**A を抜くと B が壊れる**」関係。選定後に必ず確認すること。

| 抜く機能 | 連鎖して壊れるもの |
|---------|-------------------|
| chromedp (T-031) | SVG (実用品質)、PNG、JPEG、PDF、Insights、topics、starlists、achievements (scrape mode)、support、screenshot |
| Web インスタンス | Insights (HTML)、OAuth、Control endpoint、`/:login` 経由のメトリクス取得 |
| base プラグイン | **全プラグイン** (ほぼ全プラグインが `data.User` を参照) |
| markdown template | `embed()`、Markdown 出力、Markdown-PDF |
| OAuth | extras.logged の昇格、user PAT 経由 fetch |
| presets | `config_presets` 入力 (任意なので影響小) |
| octicon 置換 | base.header フッタ、各 partial の icon 表示 |
| twemoji 置換 | フッタ「Last updated 📅」等の表示 (ASCII 代替で続行は可) |
| committer (commit) | README バッジ運用 (Action モード) |
| committer (markdown cache) | Markdown 出力 (Action モード) |
| CSS purge | SVG サイズが大きくなるだけ (動作は OK) |
| SVGO 最適化 | 同上 (動作は OK) |
| repositories ページング (T-020) | `languages`, `lines`, `notable`, `traffic`, `repositories`, `licenses` 等の repositories 配列依存プラグイン |

---

## 9. 選定ワークシート

下記をコピーして埋めると **必要タスク集合が確定** します。
完了したら GitHub issue / Notion / 別 markdown に保存推奨。

```yaml
# ==========================================================
# metrics Go 移行 — 機能選定ワークシート
# ==========================================================

# --- Q1: 起動形態 ---
entry_point:
  action: yes / no
  web:    yes / no
  cli:    yes / no

# --- Q2: 出力形式 (使うものだけ yes) ---
output_formats:
  svg:           yes / no
  png:           yes / no
  jpeg:          yes / no
  json:          yes / no
  markdown:      yes / no
  markdown_pdf:  yes / no
  insights_html: yes / no

# --- Q3: テンプレート ---
templates:
  classic:    yes / no
  repository: yes / no
  terminal:   yes / no
  markdown:   yes / no
  community:  no   # 初版固定で no

# --- Q4: プラグイン (採用するものだけ yes、迷ったら no) ---
plugins:
  # コア (固定 yes)
  base: yes
  core: yes

  # GitHub データ系
  languages:           yes / no
  languages_recent:    yes / no   # languages を yes にした場合のみ意味あり
  languages_indepth:   yes / no   # 同上
  activity:            yes / no
  achievements:        yes / no
  repositories:        yes / no
  lines:               yes / no
  gists:               yes / no
  isocalendar:         yes / no
  calendar:            yes / no
  habits:              yes / no
  stars:               yes / no
  topics:              yes / no
  starlists:           yes / no
  people:              yes / no
  notable:             yes / no
  followup:            yes / no
  discussions:         yes / no
  contributors:        yes / no
  code:                yes / no
  introduction:        yes / no
  reactions:           yes / no
  projects:            yes / no
  sponsors:            yes / no
  sponsorships:        yes / no
  stargazers:          yes / no
  skyline:             yes / no
  traffic:             yes / no
  support:             no   # deprecated

  # ソーシャル/外部 API
  wakatime:      yes / no
  pagespeed:     yes / no
  posts:         yes / no
  rss:           yes / no
  stackoverflow: yes / no
  leetcode:      yes / no
  anilist:       yes / no
  music:         yes / no
  steam:         yes / no
  tweets:        no   # deprecated

  # community (初版は no 推奨)
  crypto:           no
  nightscout:       no
  stock:            no
  chess:            no
  splatoon:         no
  fortune:          no
  poopmap:          no
  screenshot:       no
  16personalities:  no

# --- Q5: 周辺機能 ---
output_actions:        # action: yes のときのみ意味あり
  commit:        yes / no
  pull_request:  yes / no
  gist:          yes / no
  markdown_cache: yes / no
  data_changed:  yes / no
  workflow_cleanup: yes / no

auth:                  # web: yes のときのみ意味あり
  oauth:   yes / no
  control: yes / no
  restricted: yes / no

extras:
  presets:      yes / no
  twemoji:      yes / no    # 推奨 yes
  gemoji:       yes / no
  octicons:     yes         # ほぼ必須なので推奨固定
  css_purge:    yes / no
  xml_format:   yes / no
  svgo:         no          # 初版省略推奨

# --- 出力 ---
# このワークシートを埋めたら、12-tasks.md と突き合わせて
# 「採用機能の必要タスク T-XXX」を列挙する。
# 一括スクリプトでの自動化は §10 参照。
```

### 9.1 ワークシート → タスク集合の作り方 (手順)

1. **土台 14 タスク** を採用 ([§1.2](#12-必ず採用すべき-土台))。
2. ワークシートで yes にした各機能の **必要タスク** 列を [§3〜§7](#3-機能カタログ--エントリポイント) から集める。
3. 各機能の **必須依存** を辿り、漏れているタスクを追加。
4. [§8 依存早見表](#8-依存早見表) で「抜いて良いはずが連鎖で壊れる」ものを再確認。
5. テスト系 (T-118, T-119, T-120) を最低限採用 (どんな MVP でも必須)。

例: "Action モードで JSON + SVG + classic + languages + activity + isocalendar + repositories + commit 出力" を選んだ場合、

- 土台 14 個
- T-023 (classic skeleton) + T-024〜T-028 (5 partial)
- T-029 (JSON) + T-031, T-032, T-039 (SVG/PNG)
- T-041 (languages), T-044 (activity), T-049 (isocalendar), T-046 (repositories) + 各プラグインの partial
- T-105〜T-110 (Action + commit)
- T-118〜T-120 (テスト基盤)

合計 **30〜35 タスク前後** に収まる ≈ 4〜6 週間スコープ。

---

## 10. 参考プロファイル例

「自分で 0 から考える時間が惜しい」場合のクイックスタート例。**そのまま採用しても良いし、ベースにして調整しても良い**。

### 10.1 プロファイル A: 最小 README バッジ (Action + SVG)

> 個人 README に "言語使用率 + 最近のアクティビティ + アチーブメント" を貼る最小構成。

- Entry: Action のみ
- Output: SVG (PNG/JPEG/JSON 不要なら省略可)
- Template: classic のみ
- Plugins: `base`, `core`, `languages`, `activity`, `achievements`, `isocalendar`, `repositories`
- 周辺: commit, octicons, twemoji (任意), data-changed
- **約 30 タスク / 4〜5 週間**

### 10.2 プロファイル B: JSON 軽量データソース (CLI のみ)

> 自前ダッシュボードのバックエンド用に "GitHub の集計済 JSON" だけ欲しい。chromedp 不要。

- Entry: CLI のみ
- Output: JSON のみ
- Template: classic (JSON 出力なら image.svg 評価は不要だが、partial 並びは使う)
- Plugins: 採用したいデータ系を選ぶ (例: `base`, `core`, `languages`, `lines`, `habits`)
- 周辺: presets (任意)
- **約 22 タスク / 3 週間** — **最も軽い**

### 10.3 プロファイル C: フル Web インスタンス

> 第三者にも公開できる Web サーバ。OAuth 付き。

- Entry: Web + Action (両方)
- Output: SVG, PNG, JSON
- Template: classic, repository
- Plugins: 主要 15 個
- 周辺: OAuth, rate-limit, cache, presets, twemoji, octicons, gemoji, CSS purge
- **約 80 タスク / 12〜15 週間** — **重い**

### 10.4 プロファイル D: Markdown 中心

> Markdown 出力で README を動的生成したい。

- Entry: Action のみ
- Output: Markdown + Markdown-PDF (+ JSON)
- Template: markdown + classic (embed 用)
- Plugins: 採用 5〜10 個
- 周辺: markdown cache commit, chromedp (PDF 用)
- **約 45 タスク / 6〜8 週間**

---

## 11. 次のアクション

1. [§9 選定ワークシート](#9-選定ワークシート) を埋める。
2. 必要タスク集合を確定したら、[12-tasks.md](./12-tasks.md) からその T-XXX を抜粋した issue 作成リストを作る。
3. 不採用タスクは backlog に置き、必要になったときに採用する。

ワークシートが埋まったらレビューします (この specs 上で議論する形でも、新規ファイル `15-selection-result.md` を作る形でも可)。
