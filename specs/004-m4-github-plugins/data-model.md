# Phase 1 Data Model: GitHub プラグイン群 (M4)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は採用 21 プラグインと base 拡張の Go 型を整理する。各エンティティは **Entity / Source package / Public fields / Validation / Owned by** の四項目で記述する。各 PluginResult の `json:` tag と field 型は上流 `data.plugins.<name>` と完全互換 (constitution 原則 II)。

---

## E-001: PluginResult (共通基底コンセプト)

- **Source package**: 各 plugin の `<name>.go` (構造体は plugin ごとに独立、共通基底クラスは作らない)
- **共通フィールド** (上流互換のため全 plugin に存在):
  - `Skipped bool ` json:"skipped,omitempty"` ` — scope 不足 / 機能未実装 / chromedp 不在等で実行スキップした旨を JSON 表現する
  - `SkippedReason string ` json:"-"` ` — 内部ログ用、JSON 出力には含めない
- **Validation**: `Skipped=true` のとき他フィールドは zero value で初期化 MUST。`omitempty` により skipped=false の plugin では `skipped` キーが JSON に出ない (上流挙動と一致)。
- **Owned by**: 各 plugin パッケージ。共通基底は意図的に作らない (Go の interface 型断片的依存を避けるため duck typing で運用)。

---

## E-010: LanguagesResult (P1: T-041)

- **Source package**: `internal/plugins/languages`
- **Public fields**:
  - `Skipped bool ` json:"skipped,omitempty"` `
  - `Favorites []LanguageStat ` json:"favorites"` ` — 上位 `plugin_languages_limit` (既定 8) 件
  - `Other LanguageStat ` json:"other"` ` — limit 外の合算 ("Other")
  - `Sections []string ` json:"sections"` ` — `plugin_languages_sections` 入力値 (`["most-used"]` 既定)
  - `Mostly LanguageStat ` json:"mostly"` ` — 最大シェアの 1 件
  - `Colors map[string]string ` json:"colors"` ` — `<lang> -> #RGB` (linguist 色)
- **LanguageStat** (struct):
  - `Name string ` json:"name"` `
  - `Color string ` json:"color"` ` — `#RGB` または空
  - `Size int ` json:"size"` ` — byte 合計
  - `Count int ` json:"count"` ` — repository 出現数
  - `Value float64 ` json:"value"` ` — 全体シェア (0..1)
- **Validation**: `Favorites` は `Value` 降順。`Mostly` は `Favorites[0]` または `Other` の大きい方 (`plugin_languages_threshold` 既定 0%)。`Sections` は `["most-used", "recently-used", "most-used-charts"]` のサブセット。
- **Owned by**: `languages` plugin。

---

## E-011: LanguagesRecentResult (P3: T-042)

- **Source package**: `internal/plugins/languages` (`recent.go`, build tag `heavy`)
- **Public fields**: `Skipped`, `Favorites`, `Other`, `Days int ` json:"days"` `, `Load int ` json:"load"` `, `Repos []string ` json:"repos"` `
- **Days**: `plugin_languages_recent_days` 既定 14
- **Load**: 取得済 PushEvent 件数 (上限は `plugin_languages_recent_load` 既定 100)
- **Repos**: 解析対象 repository 名一覧
- **Validation**: `Skipped=true` の典型ケース: `metrics.run.linguist=false` または PushEvent 0 件。
- **Owned by**: `languages.recent` モード。`LanguagesResult` とは別キーで JSON 出力 (`data.Plugins.languages.recent`)。

---

## E-012: LanguagesIndepthResult (P3: T-043)

- **Source package**: `internal/plugins/languages` (`indepth.go`, build tag `heavy`)
- **Public fields**: `Skipped`, `Repositories map[string]LanguageBytes ` json:"repositories"` `, `Total LanguageBytes ` json:"total"` `, `Analyzed []string ` json:"analyzed"` `, `Errors []string ` json:"errors,omitempty"` `
- **LanguageBytes** (struct):
  - `Bytes map[string]int64 ` json:"bytes"` ` — `<lang> -> 累計バイト数`
- **Validation**: `extras.metrics.cpu.overuse=false` または `metrics.run.git=false` または `metrics.run.linguist=false` のいずれかで `Skipped=true`。タイムアウト (`plugin_languages_analysis_timeout_repositories` 既定 7.5min / repo, 全体 15min) を超えた repo は `Errors` に文字列追加。
- **Owned by**: `languages.indepth` モード。

---

## E-013: ActivityResult (P1: T-044)

- **Source package**: `internal/plugins/activity`
- **Public fields**: `Skipped`, `Events []ActivityEvent ` json:"events"` `, `Days int ` json:"days"` `
- **ActivityEvent** (struct):
  - `Type string ` json:"type"` ` — `"PushEvent"` / `"PullRequestEvent"` / etc
  - `Repo string ` json:"repo"` ` — `owner/name`
  - `Date time.Time ` json:"date"` ` — RFC3339
  - `Visibility string ` json:"visibility"` ` — `"public"` / `"private"`
- **Validation**: `Events` は `Date` 降順、最大 `plugin_activity_limit` (既定 100) 件。`plugin_activity_visibility` (`public` / `private` / `all`) で filter。
- **Owned by**: `activity` plugin。

---

## E-014: AchievementsResult (P1: T-045)

- **Source package**: `internal/plugins/achievements`
- **Public fields**: `Skipped`, `List []Achievement ` json:"list"` `, `Ranks map[string]string ` json:"ranks"` `
- **Achievement** (struct):
  - `ID string ` json:"id"` ` — `"commits"`, `"repositories"`, `"stars"`, `"followers"`, `"organizations"`, etc
  - `Rank string ` json:"rank"` ` — `"S"` / `"A"` / `"B"` / `"C"` / `"X"` (X = 未達)
  - `Title string ` json:"title"` `
  - `Description string ` json:"description"` `
  - `Icon string ` json:"icon"` ` — octicon 名 or 内蔵 SVG name
  - `Value int ` json:"value"` ` — 統計値
- **Validation**: `plugin_achievements_threshold` (`"S"` / `"A"` / `"B"` / `"C"`) 以上の rank のみ `List` に含める。`plugin_achievements_only` / `_ignored` でフィルタ。
- **Owned by**: `achievements` plugin。base の computed statistics を消費 (新規 API 呼び出しゼロ)。

---

## E-015: RepositoriesResult (P1: T-046)

- **Source package**: `internal/plugins/repositories`
- **Public fields**: `Skipped`, `Featured []Repository ` json:"featured"` `, `Pinned []Repository ` json:"pinned,omitempty"` `, `Starred []Repository ` json:"starred,omitempty"` `, `Random []Repository ` json:"random,omitempty"` `
- **Repository** (struct):
  - `NameWithOwner string ` json:"nameWithOwner"` `
  - `Description string ` json:"description"` `
  - `Stars int ` json:"stars"` `
  - `Forks int ` json:"forks"` `
  - `Language *LanguageStat ` json:"language,omitempty"` ` — primary
  - `URL string ` json:"url"` `
  - `Visibility string ` json:"visibility"` ` — `"public"` / `"private"`
- **Validation**: `plugin_repositories_order` (`stars` / `forks` / `updated`) で sort 順を決定。`plugin_repositories_pinned=true` で pinnedItems を取得 (`Pinned` フィールド)、同様に `_starred`, `_random` で個別 fetch。`Featured` は base.Computed.Repositories からフィルタ済の上位。
- **Owned by**: `repositories` plugin。

---

## E-016: IsocalendarResult (P1: T-049)

- **Source package**: `internal/plugins/isocalendar`
- **Public fields**: `Skipped`, `Weeks []ISOWeek ` json:"weeks"` `, `Streak Streak ` json:"streak"` `, `Average float64 ` json:"average"` `, `Sum int ` json:"sum"` `, `Duration string ` json:"duration"` `
- **ISOWeek** (struct): `Year int`, `Week int`, `Days [7]int` (Mon-Sun, contribution count)
- **Streak** (struct): `Max int`, `Current int`
- **Validation**: `plugin_isocalendar_duration` ∈ `{"half-year", "full-year"}`。前者で 26 週、後者で 53 週分の `Weeks`。
- **Owned by**: `isocalendar` plugin。

---

## E-020: CalendarResult (P2: T-050)

- **Source package**: `internal/plugins/calendar`
- **Public fields**: `Skipped`, `Years []YearCalendar ` json:"years"` `, `Limit int ` json:"limit"` `
- **YearCalendar**: `Year int`, `Total int`, `Months [12]int`
- **Validation**: `plugin_calendar_limit` (既定 0 = 制限なし) で年数を切り詰める。
- **Owned by**: `calendar` plugin。

---

## E-021: HabitsResult (P2: T-051)

- **Source package**: `internal/plugins/habits`
- **Public fields**: `Skipped`, `Days int ` json:"days"` `, `Facts HabitFacts ` json:"facts"` `, `Charts HabitCharts ` json:"charts"` `, `From int ` json:"from"` `, `Trim bool ` json:"trim"` `
- **HabitFacts** (struct): `LinesPerCommit float64`, `CharsPerLine float64`, `CommitsPerDay float64`, `IndentStyle string` (`"spaces"` / `"tabs"`)
- **HabitCharts** (struct): `Hours [24]int`, `Days [7]int`
- **Validation**: `plugin_habits_from` (既定 200) 件の PushEvent を解析。各 commit の diff (`/repos/X/commits/Y`) から indent/lines を集計。
- **Owned by**: `habits` plugin。

---

## E-022: StarsResult (P2: T-052)

- **Source package**: `internal/plugins/stars`
- **Public fields**: `Skipped`, `List []StarredRepo ` json:"list"` `, `Limit int ` json:"limit"` `
- **StarredRepo**: `NameWithOwner`, `Description`, `Stars int`, `StarredAt time.Time`
- **Validation**: `plugin_stars_limit` (既定 4) 件。`user.starredRepositories(orderBy:STARRED_AT, last:N)`。
- **Owned by**: `stars` plugin。

---

## E-023: TopicsResult (P3: T-053, chromedp)

- **Source package**: `internal/plugins/topics` (build tag chromedp テストあり)
- **Public fields**: `Skipped`, `List []Topic ` json:"list"` `, `Mode string ` json:"mode"` `, `Limit int ` json:"limit"` `, `Sort string ` json:"sort"` `
- **Topic**: `Name string`, `Description string`, `Icon string` (URL or octicon name), `URL string`
- **Validation**: `plugin_topics_mode` ∈ `{"icons", "list"}`、`plugin_topics_limit` 既定 15、`plugin_topics_sort` ∈ `{"name", "starred-at"}`。chromedp が `https://github.com/stars/<login>/topics` を巡回。
- **Owned by**: `topics` plugin。

---

## E-024: StarlistsResult (P3: T-054, chromedp)

- **Source package**: `internal/plugins/starlists` (build tag chromedp)
- **Public fields**: `Skipped`, `List []Starlist ` json:"list"` `, `Languages bool ` json:"languages,omitempty"` `
- **Starlist**: `Name string`, `Description string`, `Count int`, `Languages []LanguageStat ` json:"languages,omitempty"` ` (when `_languages=true`)
- **Validation**: `plugin_starlists_languages=true` のとき各 starlist 内 repository に対して T-041 標準言語ロジックを再利用。
- **Owned by**: `starlists` plugin。

---

## E-025: PeopleResult (P2: T-055)

- **Source package**: `internal/plugins/people`
- **Public fields**: `Skipped`, `Types map[string][]Person ` json:"types"` `
- **Person**: `Login string`, `Name string`, `AvatarURL string`
- **Validation**: `plugin_people_types` ∈ `{"followers","following","sponsors","contributors","stargazers","watchers","members"}` のサブセット。各 type について `plugin_people_limit` (既定 26) 件。
- **Owned by**: `people` plugin。

---

## E-026: NotableResult (P2: T-056)

- **Source package**: `internal/plugins/notable`
- **Public fields**: `Skipped`, `List []NotableContrib ` json:"list"` `
- **NotableContrib**: `Login string`, `Repo string`, `Title string`, `Type string` (`"contributions"` / `"organization"`), `Indepth bool`
- **Validation**: `plugin_notable_filter` (default true) で重要度フィルタ。`plugin_notable_indepth=true` で詳細クエリ。
- **Owned by**: `notable` plugin。

---

## E-027: ContributorsResult (P2: T-059)

- **Source package**: `internal/plugins/contributors`
- **Public fields**: `Skipped`, `List []Contributor ` json:"list"` `, `Sections []string ` json:"sections"` `, `Base string ` json:"base"` `, `Head string ` json:"head"` `
- **Contributor**: `Login string`, `AvatarURL string`, `Commits int`, `Additions int`, `Deletions int`
- **Validation**: account kind=`repository` 限定。`plugin_contributors_base` / `_head` で commit 範囲指定。
- **Owned by**: `contributors` plugin。

---

## E-028: ReactionsResult (P2: T-062)

- **Source package**: `internal/plugins/reactions`
- **Public fields**: `Skipped`, `Issues int ` json:"issues"` `, `Comments int ` json:"comments"` `, `Discussions int ` json:"discussions"` `, `Total int ` json:"total"` `, `Days int ` json:"days"` `, `Details map[string]int ` json:"details,omitempty"` `
- **Details**: emoji → count map (`"+1": N, "-1": N, ...`)
- **Validation**: `plugin_reactions_limit_*` で各種上限。`plugin_reactions_details=true` で Details を埋める。
- **Owned by**: `reactions` plugin。

---

## E-029: ProjectsResult (P2: T-063)

- **Source package**: `internal/plugins/projects`
- **Public fields**: `Skipped`, `List []Project ` json:"list"` `, `Limit int ` json:"limit"` `
- **Project**: `Name string`, `Description string`, `URL string`, `UpdatedAt time.Time`
- **Validation**: `read:project` scope 不在で `Skipped=true`。`plugin_projects_repositories=true` で repository projects も含む。
- **Owned by**: `projects` plugin。

---

## E-030: SponsorsResult (P2: T-064)

- **Source package**: `internal/plugins/sponsors`
- **Public fields**: `Skipped`, `Sections []string ` json:"sections"` `, `Sponsors []Sponsor ` json:"sponsors"` `, `Past []Sponsor ` json:"past,omitempty"` `
- **Sponsor**: `Login string`, `Tier string`, `Since time.Time`
- **Validation**: `read:user` + `read:org` scope 不在で `Skipped=true`。`plugin_sponsors_sections` で section 切替。
- **Owned by**: `sponsors` plugin。

---

## E-031: SponsorshipsResult (P2: T-065)

- **Source package**: `internal/plugins/sponsorships`
- **Public fields**: `Skipped`, `Active []Sponsored ` json:"active"` `, `Past []Sponsored ` json:"past,omitempty"` `
- **Sponsored**: `Login string`, `Tier string`, `Since time.Time`, `Until *time.Time`
- **Validation**: `viewer.sponsorshipsAsSponsor(activeOnly:false)`。
- **Owned by**: `sponsorships` plugin。

---

## E-032: StargazersResult (P2: T-066, worldmap は M4 では nil)

- **Source package**: `internal/plugins/stargazers`
- **Public fields**: `Skipped`, `List []Stargazer ` json:"list"` `, `Charts StargazersCharts ` json:"charts"` `, `Worldmap *StargazersWorldmap ` json:"worldmap,omitempty"` ` (M4 では常に `nil`、R-012)
- **Stargazer**: `Login string`, `StarredAt time.Time`, `Location string`
- **StargazersCharts**: `Type string` (`"classic"` / `"chartist"`), `Series []ChartPoint`
- **Validation**: `plugin_stargazers_worldmap=true` の入力は受け取るが M4 では実装しない (warning ログ + `worldmap=nil`)。
- **Owned by**: `stargazers` plugin。

---

## E-033: TrafficResult (P2: T-068)

- **Source package**: `internal/plugins/traffic`
- **Public fields**: `Skipped`, `Views map[string]TrafficView ` json:"views"` ` (`<repo> -> view stats`), `Total TrafficView ` json:"total"` `
- **TrafficView**: `Count int`, `Uniques int`
- **Validation**: `repo` scope 不在で `Skipped=true`。
- **Owned by**: `traffic` plugin。

---

## E-040: Base 拡張 (organization branch / indepth / repositories paging)

- **Source package**: `internal/plugins/base`
- **新規 public fields** (`plugins.Data.Computed` 内、M1 で型のみ予約済):
  - `Computed.Repositories []Repository ` json:"repositories"` ` — base が一括取得した repository 一覧 (paging 完了後)
  - `Computed.Activity ActivityStats ` json:"activity"` ` — base.user.contributionsCollection の集約
  - `Computed.Followers int`, `Computed.Following int`, `Computed.Organizations int`, ...
- **Validation**: `Data.Account` ∈ `{"", "user", "organization"}` で分岐。organization では `Data.Organization` を埋める。user では `Data.User`。repository (M7 territory) は M4 で扱わない。
- **Owned by**: `base` plugin。pagination ロジックは `repositories.go` に分離 (cursor 再帰 + batch-halving)。
- **State transitions**:

  ```text
  Run start
   └── account kind 判定
        ├── user      → runUser → User クエリ → Computed.Repositories paging
        ├── organization → runOrganization → Organization クエリ → Members + Repos paging
        └── repository → return (nil, nil)  ← M7 で実装
  ```

---

## E-050: ScopesHelper (新規 helper)

- **Source package**: `internal/githubapi` (`scopes.go`)
- **Public API**: `(r *REST) Scopes() ([]string, error)` — 初回呼び出しで `GET /` を打ち `X-OAuth-Scopes` ヘッダから token scopes を取得しキャッシュ。以降の呼び出しはキャッシュ返却。
- **Validation**: scope なし (未認証) のとき `[]string{}` を返す (`err == nil`)。HTTP error 時は error を返し plugin 側が `*RetryableError` でラップする。
- **Owned by**: `internal/githubapi`。各 plugin (projects / sponsors / traffic) が共有。

---

## E-099: Test fixtures (`tests/fixtures/plugins/<name>/`)

- **Format**: 各 plugin に対し以下 4 種類の JSON を vendor する:
  1. `<name>_octocat.json` — 正常系。base + plugin が完全に動く mocked レスポンス。
  2. `<name>_empty.json` — 空応答。`Skipped=false` だが `List` などが zero value。
  3. `<name>_scope_missing.json` — scope 不足 (token に必要スコープなし)。`Skipped=true` 期待。
  4. `<name>_api_error.json` — GraphQL/REST 5xx。`*RetryableError` 期待。
- **Validation**: 各 fixture は table test の独立ケースとして消費される。
- **Owned by**: 各 plugin の `<name>_test.go`。
