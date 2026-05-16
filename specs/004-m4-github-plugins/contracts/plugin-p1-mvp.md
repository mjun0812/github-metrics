# Contract: P1 MVP プラグイン 5 個

**Date**: 2026-05-16 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) (E-010, E-013, E-014, E-015, E-016)

本書は P1 (MVP) 5 個の plugin の Run 規約 / 入力 / 出力 / エラー処理を確定する。対象: **languages 標準モード** (T-041), **activity** (T-044), **achievements** (T-045), **repositories** (T-046), **isocalendar** (T-049)。

## 共通契約

- 各 plugin は `init()` で `plugins.Register(Plugin)` を呼ぶ。
- `Name()` は plugin slug を小文字で返す (`"languages"`, `"activity"`, ...)。
- `Run(ctx, pc)` の返り値は `(<plugin>.Result, error)` の形 (具体的構造体は data-model.md 参照)。`error` は `*xerrors.RetryableError` または `*xerrors.InputError` のいずれか、それ以外は `nil` で返す。
- `pc.Inputs["plugin_<name>"]==true` が false のとき plugin は **そもそも Run されない** (core 側で skip)、本契約は `true` のときの挙動のみ規定。
- 各 plugin の入力 (`plugin_<name>_<opt>`) は上流 `metadata.yml` と完全互換 (constitution 原則 I)。新規キー追加 MUST NOT。

## 1. languages 標準モード (T-041)

### 1.1 入力
- `plugin_languages_limit` (int, 既定 8) — `Favorites` に含める上位件数
- `plugin_languages_threshold` (string `"%"` suffix, 既定 `"0%"`) — `Other` に集約する閾値
- `plugin_languages_ignored` (csv string, 既定 `""`) — 除外言語リスト
- `plugin_languages_skipped` (csv string, 既定 `""`) — 除外 repository リスト
- `plugin_languages_aliases` (csv `<from>:<to>` 形式) — 言語別名
- `plugin_languages_colors` (csv `<lang>:<#RGB>` 形式) — 色上書き
- `plugin_languages_other` (bool, 既定 true) — `Other` 集約を表示するか
- `plugin_languages_sections` (csv) — `["most-used"]` 既定。`["recently-used"]` / `["most-used-charts"]` は P3 で扱う

### 1.2 データソース
`base.Computed.Repositories[].Languages` を集計する。base の indepth は不要 (標準モードは表面的なバイト集計のみ)。

### 1.3 アルゴリズム
1. 各 repository の `languages.edges[].(size, node.{name,color})` を全 repo にわたって合計
2. `aliases` / `ignored` / `skipped` を適用
3. 合計バイトで降順 sort
4. 上位 `limit` 件を `Favorites`、残りを `Other` に集約 (`Other.Name="Other"`, `Other.Color="#cccccc"`)
5. 各エントリの `Value` = `size / total_bytes` (0..1)
6. `Mostly = Favorites[0]` または `Other.Value > threshold` のとき `Other`

### 1.4 出力
[E-010 LanguagesResult](../data-model.md#E-010) の構造体を返す。SVG partial `partials/languages.svg.tmpl` がこれを消費。

### 1.5 エッジケース
- `Computed.Repositories` が空 → `Skipped=true`, `SkippedReason="no repositories"`
- `total_bytes == 0` → 同上
- `aliases` で 2 言語が 1 言語に合流 → `count` は合流前の repository 出現数を加算

## 2. activity (T-044)

### 2.1 入力
- `plugin_activity_limit` (int, 既定 100)
- `plugin_activity_load` (int, 既定 300) — REST `/users/X/events` から取得する最大件数
- `plugin_activity_days` (int, 既定 14) — 何日前まで遡るか
- `plugin_activity_visibility` (string, 既定 `"public"`) — `public` / `private` / `all`
- `plugin_activity_timestamps` (bool, 既定 true)
- `plugin_activity_skipped` (csv) — 除外する event type
- `plugin_activity_ignored` (csv) — 除外する repo

### 2.2 データソース
REST `/users/{login}/events?per_page=100` を `plugin_activity_load` 件まで paging。

### 2.3 アルゴリズム
1. 各 event について `type` (PushEvent / IssuesEvent / PullRequestEvent / ReleaseEvent / WatchEvent / CreateEvent / ForkEvent) を identify
2. `visibility` と `days` で filter
3. `skipped` / `ignored` を適用
4. `Date` 降順で `limit` 件に切り詰め

### 2.4 出力
[E-013 ActivityResult](../data-model.md#E-013)。

### 2.5 エッジケース
- 該当 event 0 件 → `Skipped=false`, `Events: []`
- REST 5xx → `*RetryableError` を返し plugin スキップ (core が `Result.Errors` に追加)

## 3. achievements (T-045)

### 3.1 入力
- `plugin_achievements_threshold` (string, 既定 `"C"`) — `"S"` / `"A"` / `"B"` / `"C"` 以上
- `plugin_achievements_secrets` (bool, 既定 false) — 秘密 achievement を含める
- `plugin_achievements_display` (string, 既定 `"detailed"`) — `"detailed"` / `"compact"`
- `plugin_achievements_limit` (int, 既定 0) — 0=制限なし
- `plugin_achievements_only` (csv) — 限定する achievement ID
- `plugin_achievements_ignored` (csv)

### 3.2 データソース
**新規 API 呼び出しゼロ**。`base.Computed.{Followers, Following, Repositories, Stars, Organizations, ...}` の集計値のみ消費。

### 3.3 アルゴリズム
1. 内蔵 rank テーブル (`docs/design/06-plugins-detail.md §2.1` の表をハードコード) で各統計値を `"S" / "A" / "B" / "C" / "X"` に変換
2. `threshold` 以上の achievement のみ `List` に追加
3. `only` / `ignored` でフィルタ

### 3.4 出力
[E-014 AchievementsResult](../data-model.md#E-014)。

### 3.5 エッジケース
- `base` が失敗していて `Computed` が空 → `Skipped=true`, `SkippedReason="base data unavailable"`
- 全 achievement が rank `X` → `List: []`, `Ranks: map[<id>]"X"`

## 4. repositories (T-046)

### 4.1 入力
- `plugin_repositories_order` (string, 既定 `"stars"`) — `stars` / `forks` / `updated` / `pushed`
- `plugin_repositories_pinned` (bool, 既定 false) — pinnedItems 取得
- `plugin_repositories_starred` (bool, 既定 false) — starred repos 取得
- `plugin_repositories_random` (bool, 既定 false) — random N 件
- `plugin_repositories_affiliations` (csv, 既定 `"OWNER,COLLABORATOR"`)
- `plugin_repositories_forks` (bool, 既定 false) — fork を含めるか
- `plugin_repositories_skipped` (csv) — 除外 repo
- `plugin_repositories_batch` (int, 既定 100) — base paging size

### 4.2 データソース
- `Featured`: `base.Computed.Repositories` から `affiliations` / `forks` / `skipped` をフィルタしつつ `order` で sort、上位 N 件
- `Pinned`: GraphQL `user.pinnedItems(first:6)`
- `Starred`: GraphQL `user.starredRepositories(orderBy:STARRED_AT)` を 4 件
- `Random`: `Featured` から fisher-yates で 4 件抜粋

### 4.3 アルゴリズム
各セクションは並列 fetch (errgroup) で実行。`Pinned` / `Starred` の追加 GraphQL リクエストは plugin 内で完結 (base に依存しない)。

### 4.4 出力
[E-015 RepositoriesResult](../data-model.md#E-015)。

### 4.5 エッジケース
- `pinned` クエリが no permissions (private) → `Pinned: nil` (omitempty で JSON から消える)
- `affiliations` フィルタで 0 件 → `Featured: []`

## 5. isocalendar (T-049)

### 5.1 入力
- `plugin_isocalendar_duration` (string, 既定 `"half-year"`) — `half-year` / `full-year`

### 5.2 データソース
GraphQL `user.contributionsCollection(from:$from, to:$to) { contributionCalendar { ... } }`。`from` は `now - 26weeks` または `now - 53weeks`。

### 5.3 アルゴリズム
1. `contributionCalendar.weeks[].contributionDays[].(date, contributionCount)` を週 × 曜日の 2 次元配列に整形
2. `Streak.Max` / `Streak.Current` を貪欲アルゴリズムで計算 (0 でない日が連続する最長 / 現在の連続)
3. `Sum` = 全期間の contribution 合計、`Average` = `Sum / 期間日数`

### 5.4 出力
[E-016 IsocalendarResult](../data-model.md#E-016)。

### 5.5 エッジケース
- 期間内 contribution 0 → `Streak.Max=0`, `Streak.Current=0`, `Sum=0`, `Average=0.0`
- `Data.Account == "organization"` → `Skipped=true`, `SkippedReason="not applicable to organizations"`

## 6. テスト共通規約

- 各 plugin に `<name>_test.go` で最低 5 ケース:
  1. **正常系**: octocat 相当 fixture で `Skipped=false`、出力構造体の主要フィールド (上位 3 つ) を assert
  2. **入力上限到達**: `_limit` 等を 1 に絞り、N+1 件 fixture でも上限が守られる
  3. **scope 不足 / 空応答**: applicable な plugin のみ、`Skipped=true` を assert
  4. **API エラー (5xx)**: mocked が 5xx 返却、`*RetryableError` を assert
  5. **入力エイリアス / フィルタ**: `aliases` / `ignored` / `skipped` などのフィルタが効くこと
- 各 plugin に `tests/golden/json/m4/<name>.json` (出力 shape) と `tests/golden/classic/m4/<name>.svg` (partial SVG fragment) を `-update` で生成・commit。
- `tests/integration/plugins_p1_test.go` で 5 plugin すべてを `engine.Compute(Format:"svg")` 経由で同時に動かす e2e を 1 ケース。

## 7. 性能予算

| plugin | mocked 単独 (p95) | 備考 |
|---|---|---|
| languages 標準 | < 50ms | base 結果集計のみ、API 呼び出し 0 |
| activity | < 800ms | REST `/users/X/events` を 300 件 paging |
| achievements | < 20ms | base 結果集計のみ、API 呼び出し 0 |
| repositories | < 500ms | pinned + starred の追加 GraphQL 2 リクエスト |
| isocalendar | < 200ms | GraphQL 1 リクエスト |

合計 P1 5 plugin で < 1.5s。base (< 3s) + P1 (< 1.5s) で **< 5s** に収まる予算配分。
