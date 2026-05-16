# Contract: P2 GraphQL/REST 単体プラグイン 12 個

**Date**: 2026-05-16 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) (E-020〜E-033)

本書は P2 12 個の plugin の Run 規約を確定する。対象: **calendar / habits / stars / people / notable / contributors / reactions / projects / sponsors / sponsorships / stargazers / traffic**。各 plugin は GraphQL または REST のみで完結し、chromedp / heavy 依存なし。

共通規約は [plugin-p1-mvp.md §共通契約](./plugin-p1-mvp.md#共通契約) と同じ。本書では plugin 固有差分のみ列挙する。

## 1. calendar (T-050)

- **入力**: `plugin_calendar_limit` (int, 既定 0 = 制限なし)
- **データソース**: GraphQL `user.contributionsCollection(from:$from, to:$to)` を年単位でループ取得
- **アルゴリズム**: account 作成年から現在年まで `plugin_calendar_limit` 件分ループ
- **出力**: [E-020 CalendarResult](../data-model.md#E-020)
- **エッジケース**: account 作成年が今年と同じ → `Years: [<1 件>]`
- **性能予算**: < 500ms (mocked、5 年分)

## 2. habits (T-051)

- **入力**: `plugin_habits_from` (int, 既定 200), `_days` (int, 既定 14), `_facts` (bool, 既定 true), `_charts` (bool, 既定 false), `_charts_type` (string, `"classic"`/`"chartist"`), `_trim` (bool, 既定 false), `_languages` (csv) `_languages.recent` (csv) — `_languages` 系は languages plugin の入力と同じ alias
- **データソース**: REST `/users/X/events` で PushEvent を `_from` 件まで取得、各 commit について `/repos/X/commits/Y` で diff を fetch (上限あり)
- **アルゴリズム**:
  1. PushEvent から最新 N コミット抽出
  2. 各 commit の diff から indent (`"spaces"` / `"tabs"` 判定)、line per commit、char per line を集計
  3. `Days[7]` / `Hours[24]` 配列に commit 数を曜日 / 時刻別に集計
- **出力**: [E-021 HabitsResult](../data-model.md#E-021)
- **エッジケース**: 取得 commit 0 → `Skipped=true, SkippedReason="no recent commits"`
- **性能予算**: < 1.5s (mocked、200 commits × 1 GET = 200 リクエスト相当だが mocked HTTP では batch fixture)

## 3. stars (T-052)

- **入力**: `plugin_stars_limit` (int, 既定 4)
- **データソース**: GraphQL `user.starredRepositories(orderBy:STARRED_AT, last:$limit)`
- **アルゴリズム**: そのまま `List` に格納、`StarredAt` 降順
- **出力**: [E-022 StarsResult](../data-model.md#E-022)
- **性能予算**: < 200ms (GraphQL 1 リクエスト)

## 4. people (T-055)

- **入力**: `plugin_people_types` (csv, 既定 `"followers,following"`), `_limit` (int, 既定 26), `_size` (int, UI のみ, 既定 28), `_shuffle` (bool, 既定 false), `_identicons` (bool, 既定 false)
- **データソース**: GraphQL `user.{followers,following,sponsors,sponsoring}` または `repository.contributors` / `_stargazers` / `_watchers` / `organization.membersWithRole`、`_types` の各要素ごとに別クエリ
- **アルゴリズム**: 各 type について `_limit` 件 fetch、`_shuffle=true` なら出力前に fisher-yates
- **出力**: [E-025 PeopleResult](../data-model.md#E-025)
- **エッジケース**: `_types` に未知の値が含まれる → 無視 + WARN ログ
- **性能予算**: < 800ms (mocked、type 数だけ並列 GraphQL)

## 5. notable (T-056)

- **入力**: `plugin_notable_filter` (bool, 既定 true), `_repositories` (bool, 既定 true), `_indepth` (bool, 既定 false), `_types` (csv, 既定 `"organization,company"`), `_from` (string, 既定 `"all"`), `_self` (bool, 既定 false)
- **データソース**: GraphQL `repositoryOwner(login:$login)` + 関連 organization / company
- **アルゴリズム**: 上流 `docs/design/06-plugins-detail.md §2.15` のフィルタ手順を再現
- **出力**: [E-026 NotableResult](../data-model.md#E-026)
- **性能予算**: < 600ms (`_indepth=true` のときは < 1.2s)

## 6. contributors (T-059)

- **入力**: `plugin_contributors_base` (string), `_head` (string), `_contributions` (bool, 既定 false), `_sections` (csv), `_ignored` (csv) — repository テンプレ向け
- **データソース**: REST `/repos/{owner}/{repo}/stats/contributors`, GraphQL `repository.refs.commits` で base/head 範囲取得
- **アルゴリズム**: account kind=`"repository"` のときのみ動作 (M4 では account kind 検証のみ、実装は M7 で repository テンプレ整備後に活性化)
- **出力**: [E-027 ContributorsResult](../data-model.md#E-027)
- **エッジケース**: M4 段階 (account kind が user/organization のみ存在) では `Skipped=true, SkippedReason="repository template not yet available"` で空応答返却 OK
- **性能予算**: < 800ms (mocked)

## 7. reactions (T-062)

- **入力**: `plugin_reactions_limit_issues` (int, 既定 100), `_limit_comments` (int, 既定 100), `_limit_discussions` (int, 既定 100), `_days` (int, 既定 30), `_details` (bool, 既定 false), `_ignored` (csv)
- **データソース**: GraphQL `user.issues / issueComments / discussionComments` の reactions を fetch
- **アルゴリズム**: 各種 reaction (`+1`, `-1`, `LAUGH`, `HOORAY`, `CONFUSED`, `HEART`, `ROCKET`, `EYES`) をカウント
- **出力**: [E-028 ReactionsResult](../data-model.md#E-028)
- **性能予算**: < 1s (3 種類の追加 GraphQL)

## 8. projects (T-063)

- **入力**: `plugin_projects_limit` (int, 既定 4), `_repositories` (bool, 既定 false), `_descriptions` (bool, 既定 false)
- **データソース**: GraphQL `user.projects` (+ `_repositories=true` のとき `user.repositories.nodes.projects`)
- **アルゴリズム**: `read:project` scope 不在で `Skipped=true`、有なら paging で `_limit` 件
- **出力**: [E-029 ProjectsResult](../data-model.md#E-029)
- **エッジケース**: scope 不在 → `Skipped=true, SkippedReason="missing read:project scope"`
- **性能予算**: < 500ms

## 9. sponsors (T-064)

- **入力**: `plugin_sponsors_sections` (csv, 既定 `"sponsors"`), `_size` (int, UI), `_title` (string, UI), `_past` (bool, 既定 false)
- **データソース**: GraphQL `user.sponsorsListing` + `user.sponsors(first:N)`
- **アルゴリズム**: `read:user` + `read:org` scope のいずれかが必要、両方不在で `Skipped=true`
- **出力**: [E-030 SponsorsResult](../data-model.md#E-030)
- **性能予算**: < 500ms

## 10. sponsorships (T-065)

- **入力**: `plugin_sponsorships_sections` (csv)
- **データソース**: GraphQL `viewer.sponsorshipsAsSponsor(activeOnly:false)`
- **アルゴリズム**: そのまま `Active` / `Past` に振り分け
- **出力**: [E-031 SponsorshipsResult](../data-model.md#E-031)
- **性能予算**: < 400ms

## 11. stargazers (T-066, worldmap は nil)

- **入力**: `plugin_stargazers_charts` (bool, 既定 true), `_charts_type` (string, 既定 `"classic"`), `_worldmap` (bool, 既定 false), `_worldmap_token` (string), `_worldmap_sample` (int, 既定 200)
- **データソース**: GraphQL `repository.stargazers(orderBy:STARRED_AT, first:N)` を paging
- **アルゴリズム**:
  - `Charts`: stargazer の `starredAt` を時系列に sort、`_charts_type` に応じて出力フォーマット決定
  - `Worldmap`: **M4 では未実装** (R-012)。`_worldmap=true` が入力に来ても `worldmap=nil` を返し、WARN ログ「`stargazers: worldmap is not yet implemented in M4 (planned as N-task)`」を出力
- **出力**: [E-032 StargazersResult](../data-model.md#E-032)
- **エッジケース**: account kind=`"repository"` 限定。user/org では `Skipped=true`
- **性能予算**: < 1s

## 12. traffic (T-068)

- **入力**: `plugin_traffic` (bool トリガ、追加 input なし)
- **データソース**: REST `/repos/X/Y/traffic/views` を `base.Computed.Repositories` 全件に対し並列発行
- **アルゴリズム**:
  - `repo` scope 不在で `Skipped=true, SkippedReason="missing repo scope"`
  - 各 repository の `count` / `uniques` を `Views` map に格納、`Total` に合計
- **出力**: [E-033 TrafficResult](../data-model.md#E-033)
- **エッジケース**: traffic API は **owner repo のみ** 統計取得可能 (collaborator では 403)、403 を返した repo は `Views` から除外しつつ continue
- **性能予算**: < 2s (100 repo 並列、mocked)

## 13. テスト共通規約

各 plugin に対し:

- `tests/golden/json/m4/<name>.json` (出力 shape) と `tests/golden/classic/m4/<name>.svg` (partial fragment) を `-update` で生成
- 最低 5 ケースのテーブルテスト (P1 と同基準)
- `tests/integration/plugins_p2_test.go` で 12 plugin を分割した複数 e2e (3〜4 plugin ずつのバンドル) — 1 ケースで全 12 を同時に動かすと表面エラーが識別しにくい

## 14. 合計性能予算

12 plugin が並列実行されるが、最も重い `habits` (< 1.5s) + `traffic` (< 2s) でも理論最大は 2s。実測で並列化オーバヘッド込み < 3s を目標とする。base (< 3s) + P1 (< 1.5s) + P2 (< 3s) の並列実行で **< 5s** が成立する予算配分。
