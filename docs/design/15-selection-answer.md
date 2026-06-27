# 15. 選定回答書 (記入用テンプレート)

`14-feature-selection.md` の判断結果を **このファイルに直接記入** してください。
記入が完了すると、必要タスク T-XXX 集合が確定し、issue 化作業に進めます。

> このファイルは **記入用テンプレート** です。記入済の決裁書として保存したい場合は、
> 本ファイルをコピーして `decisions/YYYY-MM-DD-selection.md` 等にリネームしてから埋めてください。

---

## 0. メタ情報

| 項目             | 値                                                   |
| ---------------- | ---------------------------------------------------- |
| プロジェクト名   | `<記入>` (例: `metrics-go-mvp`)                      |
| 起票者           | `<記入>`                                             |
| 起票日           | `<YYYY-MM-DD>`                                       |
| 想定リリース時期 | `<YYYY-MM-DD>` または `<未定>`                       |
| 想定担当者数     | `<記入>` 名                                          |
| 参照             | [14-feature-selection.md](./14-feature-selection.md) |

### 0.1 全体方針 (自由記入)

このプロジェクトで `metrics` を **何のために** 移行するか、**最低限なにができれば成功** か、を 3〜5 行で記述してください。後の判断軸の基礎になります。

```
<例>
- 目的: 個人 README の自動更新バッジを軽量にしたい
- 成功基準: 既存 SVG と同じ見た目で languages / activity / isocalendar が表示できる
- 期間: 6 週間以内
- 担当: 1 名 (兼任)
- 非ゴール: Web インスタンス公開、PDF 出力
```

`<ここに記入>`

---

## 1. Q1: 起動形態

採用するものに `[x]` を付けてください (複数可)。

- [x] (a) GitHub Action として動く
- [ ] (b) Web インスタンスとして動く
- [x] (c) CLI 単独実行
- [ ] (d) Action + Web 両方

**理由 / 補足**:

```
<記入>
必要な機能だけをシンプルに実装するため、Action と CLI のみ採用します。
Web インスタンスは運用コストが高く、当面は不要と判断しました。
```

---

## 2. Q2: 出力形式

採用する形式に `[x]` を付けてください (複数可)。

- [x] SVG (※ chromedp が必要)
- [x] PNG (※ SVG 採用が前提)
- [x] JPEG (※ SVG 採用が前提)
- [x] JSON
- [ ] Markdown
- [ ] Markdown-PDF (※ chromedp + Chromium 必要)
- [ ] Insights HTML (※ Web インスタンス必須)

**理由 / 補足**:

```
<記入>
Visualizationが主目的のため、SVG/PNG/JPEGは必須とします。
JSONは将来の拡張や外部利用を考慮して採用します。
そのほかは不要です。
```

---

## 3. Q3: テンプレート

採用するテンプレートに `[x]` を付けてください (複数可)。

- [x] classic (user / organization 向け定番)
- [x] repository (リポジトリ向け)
- [ ] terminal (SSH 風の見た目)
- [ ] markdown (Markdown ファイル経由)
- [ ] community (動的取得) — **初版省略推奨**

**理由 / 補足**:

```
<記入>
classic は個人/組織両方に対応できるため必須とします。
repository はリポジトリ向けの需要があると考え採用します。
そのほかはメンテナンスコストに見合わないと判断し、初版では採用しません。
```

---

## 4. Q4: プラグイン

### 4.1 コアプラグイン (固定採用)

- [x] `core`

> [!NOTE]
> 旧 `base` plugin は #605 で削除。共有データ取得は `internal/dataprovider`
> の lazy / memoized Provider に集約し、識別情報カードは opt-in な
> `header` plugin (#602) が担当する。



### 4.2 GitHub データ系

⭐ = 採用率の高いおすすめ。

- [x] ⭐ `languages` — 言語使用比率 (M)
  - [x] `languages.recent` — 最近使用言語 (M, API 消費大)
  - [x] `languages.indepth` — 行数まで集計 (L, CPU/ディスク重)
- [x] ⭐ `activity` — 最近のアクティビティ (S)
- [x] ⭐ `achievements` — バッジ表示 (M)
- [x] ⭐ `repositories` — featured/pinned 表示 (S)
- [ ] `lines` — 追加/削除行数 (M, REST stats を多用)
- [ ] `gists` — Gist 統計 (S)
- [x] ⭐ `isocalendar` — 3D 等尺カレンダー (M)
- [x] `calendar` — 複数年カレンダー (M)
- [x] `habits` — 曜日/時間帯傾向 (M, API 消費大)
- [x] `stars` — 最近スターしたリポジトリ (S)
- [x] `topics` — スターしたトピック (M, chromedp 必須)
- [x] `starlists` — Star Lists (L, chromedp)
- [x] `people` — フォロワー一覧等 (M)
- [x] `notable` — 注目コントリビューション (M)
- [ ] `followup` — open/closed 比 (S)
- [ ] `discussions` — Discussions 統計 (S)
- [x] `contributors` — リポジトリ貢献者 (M, repository テンプレ向け)
- [ ] `code` — ランダムコード片 (M, **プライベートコード漏洩リスク注意**)
- [ ] `introduction` — bio/description (S)
- [x] `reactions` — リアクション集計 (M)
- [x] `projects` — GitHub Projects (M, `read:project` scope 必要)
- [x] `sponsors` — スポンサー紹介 (S, `read:user`/`read:org` 必要)
- [x] `sponsorships` — スポンサーシップ履歴 (S)
- [x] `stargazers` — スターチャート/世界地図 (L, 世界地図は Google API key)
- [ ] `skyline` — 3D city/skyline (L, 重い)
- [x] `traffic` — アクセス数 (S, `repo` scope 必要)
- [ ] `support` (deprecated) — 旧 Support 統計 (S, 上流終了済)

### 4.3 ソーシャル/外部 API 系

`(key)` = ユーザがトークン/鍵を用意する必要あり。

- [ ] `wakatime` (key, M) — 利用者多い
- [ ] `pagespeed` (key, S) — ウェブサイト持ちのみ
- [ ] `posts` (dev.to/hashnode/medium, S)
- [ ] `rss` (任意 URL, S)
- [ ] `stackoverflow` (key, S)
- [ ] `leetcode` (S)
- [ ] `anilist` (S)
- [ ] `music` (key/OAuth, L) — provider 複数で複雑
- [ ] `steam` (key, M)
- [ ] `tweets` (deprecated, paid) — **採用非推奨**

### 4.4 community 系

**初版は全て不採用推奨**。需要が出てから個別に追加。

- [ ] `crypto`
- [ ] `nightscout`
- [ ] `stock`
- [ ] `chess`
- [ ] `splatoon`
- [ ] `fortune`
- [ ] `poopmap`
- [ ] `screenshot`
- [ ] `16personalities`

### 4.5 プラグイン採用の理由 / 補足

```
<記入>
例:
- languages, activity, isocalendar などは README バッジの主目的なので必須
- topics, starlists はユーザの特徴を表す人気プラグインで、chromedp のコストを払っても採用する価値があると判断
- そのほかで採用しているものは、自分の README に入れたい、もしくは将来の拡張で必要になりそうなものを選びました。
```

---

## 5. Q5: 周辺機能

### 5.1 出力アクション (Q1 で Action を採用した場合のみ)

- [x] `commit` (リポジトリへ commit)
- [x] `pull-request` (+ merge / squash / rebase)
- [ ] `gist`
- [ ] markdown キャッシュ commit (Markdown 出力採用時は実質必須)
- [x] `data-changed` 判定
- [ ] Workflow run cleanup

### 5.2 認証 (Q1 で Web を採用した場合のみ)

- [ ] OAuth フロー
- [ ] Control endpoint (`/.control/stop`)
- [ ] Restricted users (allow-list)
- [ ] Rate limit middleware (公開 Web なら **必須**)

### 5.3 入力支援

- [x] `config_presets` (利便性、初版は no も可)
- [x] 動的プレースホルダ `.user.login` 等 (T-005 に内包、固定採用)

### 5.4 出力最適化

- [x] CSS purge + minify (SVG サイズ削減)
- [x] XML 整形
- [ ] SVG 最適化 (SVGO 相当) — **初版省略推奨**
- [x] 絵文字 (twemoji)
- [x] 絵文字 (gemoji `:name:`)
- [x] octicons (`:octicon-X-Y:`) — 多数 partial で使用、ほぼ必須

### 5.5 テスト基盤 (固定採用推奨)

- [x] REST mock (T-118)
- [x] GraphQL mock (T-119)
- [x] golden test framework (T-120)

### 5.6 補足

```
- Action + CLI のみで動かし、Web は採用しない方針。OAuth・rate limit middleware は不要。
- SVG/PNG/JPEG/JSON を出力するため chromedp は必須。Markdown 系は不採用。
- README バッジ運用のため Action の commit / pull-request / data-changed 判定を採用。
- SVG サイズ削減のため CSS purge + XML format + 絵文字置換を採用。SVGO は初版省略。
- presets を採用するので T-006 を実装対象に追加。
```

---

## 6. 採用タスク集合 (記入結果)

ここまで埋めた結果を元に、採用する T-XXX を列挙してください。
[14-feature-selection.md §1.2](./14-feature-selection.md#12-必ず採用すべき-土台) の土台 14 タスクは固定です。

### 6.1 土台 (固定)

- [x] T-001, T-002, T-003, T-004, T-005, T-007, T-008, T-009, T-010, T-012, T-013
- [x] T-014, T-015, T-016, T-017, T-018, T-020, T-021, T-022

追加採用 (Q1〜Q5 の選択結果による):

- [x] T-006 (presets) — §5.3 で `config_presets` 採用のため
- [ ] T-011 (cache) — Web 不採用 / Action は run ごとに短命なので不要
- [x] T-019 (base.indepth) — achievements の閾値判定で commits 全期間集計を使うため精度向上目的で **推奨採用**

### 6.2 出力形式分

- [x] T-029 (JSON) — §2 で JSON 採用
- [x] T-031 (chromedp) — §2 で SVG/PNG/JPEG 採用
- [x] T-032 (svg.Resize) — §2 で SVG 採用
- [x] T-033 (svg.Hash) — §5.1 で `data-changed` 判定採用
- [x] T-034 (CSS optimizer) — §5.4 で CSS purge 採用
- [x] T-035 (XML format) — §5.4 で XML 整形採用
- [x] T-036 (twemoji) — §5.4 で twemoji 採用
- [x] T-037 (gemoji) — §5.4 で gemoji 採用
- [x] T-038 (octicons) — §5.4 で octicons 採用 (固定)
- [x] T-039 (PNG/JPEG) — §2 で PNG/JPEG 採用
- [ ] T-040 (PDF) — §2 で Markdown-PDF 不採用

### 6.3 テンプレート分

- [x] T-023 (classic skeleton) — §3 で classic 採用
- [x] T-024〜T-028 (classic base partials × 5) — 同上
- [x] T-089 (repository) — §3 で repository 採用
- [ ] T-090 (terminal) — §3 で不採用
- [ ] T-091 (markdown) — §3 で不採用
- [ ] T-092 (markdown-pdf) — §2 で Markdown-PDF 不採用
- [ ] T-093 (community loader) — §3 で不採用 (初版省略)

### 6.4 プラグイン分

§4 のチェック結果から:

- [x] T-041 (languages)
- [x] T-042 (languages.recent)
- [x] T-043 (languages.indepth)
- [x] T-044 (activity)
- [x] T-045 (achievements)
- [x] T-046 (repositories)
- [ ] T-047 (lines) — §4 で不採用
- [ ] T-048 (gists) — §4 で不採用
- [x] T-049 (isocalendar)
- [x] T-050 (calendar)
- [x] T-051 (habits)
- [x] T-052 (stars)
- [x] T-053 (topics)
- [x] T-054 (starlists)
- [x] T-055 (people)
- [x] T-056 (notable)
- [ ] T-057 (followup) — §4 で不採用
- [ ] T-058 (discussions) — §4 で不採用
- [x] T-059 (contributors)
- [ ] T-060 (code) — §4 で不採用 (プライベートコード漏洩リスク)
- [ ] T-061 (introduction) — §4 で不採用
- [x] T-062 (reactions)
- [x] T-063 (projects)
- [x] T-064 (sponsors)
- [x] T-065 (sponsorships)
- [x] T-066 (stargazers)
- [ ] T-067 (skyline) — §4 で不採用
- [x] T-068 (traffic)
- [ ] T-069 (support) — §4 で不採用 (deprecated)
- [ ] T-070〜T-079 (ソーシャル/外部 API 系) — §4.3 で全て不採用
- [ ] T-080〜T-088 (community 系) — §4.4 で全て不採用

**プラグイン採用合計: 21 タスク**

### 6.5 Web 系

§1 で Web 不採用のため全て **不採用**:

- [ ] T-094 (server skeleton)
- [ ] T-095 (middleware)
- [ ] T-096 (静的アセット配信)
- [ ] T-097 (メタエンドポイント)
- [ ] T-098 (`GET /:login`)
- [ ] T-099 (insights endpoint)
- [ ] T-100 (OAuth)
- [ ] T-101 (control endpoint)
- [ ] T-102 (graceful shutdown + pending dedup)
- [ ] T-103 (rate refresher)

### 6.6 Action 系

- [x] T-105 (action entrypoint)
- [x] T-106 (INPUT\_\* パーサ + presets 統合)
- [x] T-108 (token validation)
- [x] T-109 (render + retry)
- [x] T-110 (commit) — §5.1 で採用
- [x] T-111 (pull-request) — §5.1 で採用
- [ ] T-112 (gist) — §5.1 で不採用
- [ ] T-113 (markdown キャッシュ commit) — §2 で Markdown 不採用
- [x] T-114 (data-changed) — §5.1 で採用
- [ ] T-115 (workflow cleanup) — §5.1 で不採用 (任意機能)
- [ ] T-116 (insights webserver) — §2 で Insights 不採用
- [x] T-117 (CLI flags) — §1 で CLI 採用

### 6.7 テスト/インフラ

- [x] T-118 (REST mock) — 固定
- [x] T-119 (GraphQL mock) — 固定
- [x] T-120 (golden test framework) — 固定
- [x] T-121 (e2e compute) — **推奨**: 採用プラグイン群と classic の最小 e2e
- [x] T-122 (e2e action) — **推奨**: §1 で Action 採用なので必須級
- [ ] T-123 (e2e web) — §1 で Web 不採用
- [ ] T-124 (bench) — **推奨で不採用**: 初版は機能優先、後で追加可
- [x] T-125 (linter/govulncheck) — **推奨**: 公開リリースの品質保証
- [x] T-126 (Dockerfile) — Action 配布に必須
- [ ] T-127 (CLI image) — **推奨で不採用**: CLI も chromedp 同梱版で動くので軽量化は後回し
- [x] T-128 (action.yml 切替) — Action 採用なので必須
- [ ] T-129 (action.yml 自動生成) — **推奨で不採用**: 初版は手書きで運用
- [x] T-130 (release script) — 配布に必須
- [x] T-131 (migration guide) — **推奨**: ユーザ向け移行手順
- [ ] T-132 (JSON Schema) — **推奨で不採用**: 初版は無くても動作する

### 6.8 採用タスク数の合計

| 区分            | 数     | 内訳                                     |
| --------------- | ------ | ---------------------------------------- |
| 土台 (固定)     | 19     | T-001〜005, 007〜010, 012〜018, 020〜022 |
| 土台 (追加)     | 2      | T-006 (presets), T-019 (base.indepth)    |
| 出力形式        | 10     | T-029, 031〜039 (T-040 除く)             |
| テンプレート    | 7      | T-023, 024〜028, 089                     |
| プラグイン      | 21     | §6.4 参照                                |
| Web 系          | 0      | (全て不採用)                             |
| Action 系       | 8      | T-105, 106, 108, 109, 110, 111, 114, 117 |
| テスト/インフラ | 10     | T-118〜122, 125, 126, 128, 130, 131      |
| **合計**        | **77** | (M1 単独着手 19、その後追加 58)          |

---

## 7. 不採用タスク (backlog)

意図的に **採用しない** タスクをここに記録します。後で必要になった時の参照用。

### 7.1 出力形式

- T-040 (Markdown PDF) — Markdown / Markdown-PDF 不採用のため

### 7.2 テンプレート

- T-090 (terminal) — 見た目バリエーション、優先度低
- T-091 (markdown), T-092 (markdown-pdf) — Markdown 出力不採用
- T-093 (community loader) — 初版省略

### 7.3 プラグイン

- T-047 (lines) — 必要性低、API rate 消費大
- T-048 (gists) — 必要性低
- T-057 (followup), T-058 (discussions) — 必要性低
- T-060 (code) — **プライベートコード漏洩リスク** で恒久不採用候補
- T-061 (introduction) — 必要性低
- T-067 (skyline) — 重い、優先度低
- T-069 (support) — deprecated (上流終了)
- T-070〜T-079 (社外 API 系全 10 個) — 初版で全部見送り
- T-080〜T-088 (community 系全 9 個) — 初版で全部見送り

### 7.4 Web 系

- T-094〜T-103 (10 個) — Web インスタンス採用しないため恒久不採用候補

### 7.5 Action 系

- T-112 (gist) — Gist 出力不採用
- T-113 (markdown キャッシュ commit) — Markdown 不採用
- T-115 (workflow cleanup) — 任意機能
- T-116 (insights webserver) — Insights 不採用

### 7.6 テスト/インフラ

- T-123 (e2e web) — Web 不採用
- T-124 (bench) — 後回し
- T-127 (CLI image) — 後回し (軽量化はリリース後)
- T-129 (action.yml 自動生成) — 手書き運用
- T-132 (JSON Schema 公開) — 初版で省略

---

## 8. 既知のリスク / 注意点

選定結果から発生する **想定リスク** を記録します。

| リスク              | 内容                                     | 対策                                        |
| ------------------- | ---------------------------------------- | ------------------------------------------- |
| 例: chromedp 不採用 | SVG の height が auto になり崩れる可能性 | image.svg で `height="auto"` 固定運用にする |
| `<記入>`            |                                          |                                             |

---

## 9. 次のアクション

選定確定後の動きを明示します。

- [x] 採用タスク (§6) を [12-tasks.md](./12-tasks.md) と突き合わせて issue 化 → M1〜M10 phase に分割して順次完了
- [x] 不採用タスク (§7) を backlog として参照 (不採用 plugin の混入は `tests/compliance/compliance_test.go::TestNoUnadoptedPluginReference` で gating 済)

### 9.1 採用機能実装ステータス (2026-05-23 時点)

| Phase | 内容 | 状態 |
|---|---|---|
| M1 (001) | プロジェクト基盤 (logger / errors / settings / HTTP client / GitHub client / Engine / Plugin I/F) | ✅ merged |
| M2 (002) | classic skeleton + JSON 出力 + 5 base partial | ✅ merged |
| M3 (003) | chromedp rendering pipeline (SVG/PNG/JPEG) | ✅ merged |
| M4 (004) | 19 plugin scaffold (全採用 plugin の Result + partial + skeleton Run) | ✅ merged |
| M6 (005) | Action entrypoint + CLI flags + commit/PR/data-changed | ✅ merged |
| M7 (006) | repository template | ✅ merged |
| M9 (007) | test infrastructure consolidation | ✅ merged |
| M10 (008) | release / Docker distribution | ✅ merged |
| 009 | release tag automation | ✅ merged |
| 011 (#383) | 19 plugin partial DOM upstream parity | ✅ merged |
| 012 (#384) | repositories.Starred の REST data fetch | ✅ merged |
| 013 (#385) | 残り 6 plugin (sponsors / sponsorships / projects / notable / stargazers / repositories.Pinned) の GraphQL data fetch | ✅ merged on CI green |

採用機能 §4.2 で `[x]` を付けた **21 plugin** のうち、`base` / `core` (土台) + 011 で partial parity 確定 + 012 で Starred wiring + 013 で残り GraphQL plugin の wiring がすべて main に取り込まれた時点で、**採用機能の core wiring は完成**。`topics` / `starlists` (chromedp scrape 系) は spec 011 で chromedp Navigator interface 経由で wired 済みで、chromedp が利用可能なランタイムでは動作。

### 9.2 backlog (将来 spec)

採用機能完全実装後、優先度に応じて検討:

- **organization mode**: sponsors / sponsorships / projects / notable / stargazers の org 展開 (013 では user-mode のみ実装)
- **notable indepth**: pinned discussion / 最新 release / readme TLDR
- **stargazers worldmap** (R-012, §4.2 でも N-task に該当): Google Maps API 連携、現状 Skipped path
- **stargazers chart pagination**: latest 100 stars 超過の repo を全件 paginate

---

## 付録: 記入チェックリスト

提出前にすべて埋まっているか確認:

- [ ] §0 メタ情報・全体方針
- [ ] §1 Q1 起動形態
- [ ] §2 Q2 出力形式
- [ ] §3 Q3 テンプレート
- [ ] §4 Q4 プラグイン (+ 採用理由)
- [ ] §5 Q5 周辺機能
- [ ] §6 採用タスク集合
- [ ] §7 不採用タスク (backlog)
- [ ] §8 リスク/注意点
- [ ] §9 決裁
- [ ] §10 次のアクション
