# 06. プラグイン詳細仕様

各プラグインの責務、依存サービス、主要入力、出力データ構造を一覧化する。

> 表記:
>
> - **scopes** … GitHub PAT に必要なスコープ。`public_access` は公開情報のみ。
> - **supports** … `U`=user, `O`=organization, `R`=repository。
> - **extras** … 有効化に追加 flag (`metrics.run.puppeteer.scrapping` など) が必要なもの。
> - **deps** … 外部 API / ライブラリ依存。

## 目次

- [1. コアプラグイン](#1-コアプラグイン)
- [2. GitHub データ系プラグイン](#2-github-データ系プラグイン)
- [3. Social / 外部 API プラグイン](#3-social--外部-api-プラグイン)
- [4. community プラグイン](#4-community-プラグイン)

---

## 1. コアプラグイン

### 1.1 `base`

| 項目             | 内容                                                                                                                                                                                                                   |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| カテゴリ         | core                                                                                                                                                                                                                   |
| supports         | U, O, R                                                                                                                                                                                                                |
| scopes           | public_access                                                                                                                                                                                                          |
| deps             | GitHub GraphQL (`queries/*.graphql`)                                                                                                                                                                                   |
| 主要 inputs      | `base` (sections), `base_indepth`, `base_hireable`, `base_skip`, `repositories`, `repositories_batch`, `repositories_forks`, `repositories_affiliations`, `repositories_skipped`, `users_ignored`, `commits_authoring` |
| 出力 (data 書換) | `data.User`, `data.User.Calendar`, `data.User.ContributionsCollection`, `data.User.Repositories`, `data.Base[<section>]` ほか                                                                                          |

詳細は [05-plugins.md §5](./05-plugins.md#5-base-プラグインの特殊扱い)。

### 1.2 `core`

| 項目                      | 内容                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| カテゴリ                  | core                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| supports                  | U, O, R                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| deps                      | -                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 主要 inputs (web)         | `config.timezone`, `config.animations`, `config.display`, `config.padding`, `config.output`, `config.order`, `config.twemoji`, `config.gemoji`, `config.octicon`, `config.base64`, `config.presets`, `experimental.features`, `debug.flags`, `extras.css`, `extras.js` ほか                                                                                                                                                                            |
| 主要 inputs (action 専用) | `user`, `repo`, `token`, `template`, `query`, `filename`, `optimize`, `verify`, `markdown.cache`, `debug`, `debug.print`, `use.mocked.data`, `dryrun`, `plugins.errors.fatal`, `committer.*`, `use.prebuilt.image`, `retries`, `retries.delay`, `retries.output.action`, `retries.delay.output.action`, `output.action`, `output.condition`, `delay`, `quota.required.*`, `notice.release`, `clean.workflows`, `github.api.rest`, `github.api.graphql` |
| 出力 (data 書換)          | `data.Computed`, `data.Animated`, `data.Large`, `data.Columns`, `data.Config.Timezone`, `data.PostScripts`                                                                                                                                                                                                                                                                                                                                             |

詳細は [05-plugins.md §6](./05-plugins.md#6-core-プラグインの役割)。

---

## 2. GitHub データ系プラグイン

### 2.1 `achievements`

| 項目        | 内容                                                                                                                                                                                                                                        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O                                                                                                                                                                                                                                        |
| scopes      | public_access                                                                                                                                                                                                                               |
| deps        | GitHub GraphQL/REST + (option) chromedp (Achievements ページ scrape)                                                                                                                                                                        |
| 主要 inputs | `plugin_achievements`, `plugin_achievements_threshold` (S/A/B/C/X), `plugin_achievements_secrets`, `plugin_achievements_display` (detailed/compact), `plugin_achievements_limit`, `plugin_achievements_ignored`, `plugin_achievements_only` |
| 出力        | `[{name, rank: "S"/"A"/"B"/"C"/"X"/"$", title, text, icon, progress, value, secret}]`                                                                                                                                                       |
| 備考        | ランク判定は `rank(x, [c, b, a, s, m])` ロジック ([13-appendix.md §B](./13-appendix.md) と同様の閾値テーブル) を採用。`x>=s` → S 段階、`x>=a` → A、… のように決定。                                                                         |

### 2.2 `activity`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                               |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                                                                                                                                                                                                            |
| scopes      | public_access                                                                                                                                                                                                                                                                                                                      |
| deps        | REST `/users/{login}/events`, `/users/{login}/events/orgs/{org}`                                                                                                                                                                                                                                                                   |
| 主要 inputs | `plugin_activity_limit` (5), `plugin_activity_load` (300), `plugin_activity_days` (14), `plugin_activity_filter` (push/issue/pr/release/review/ref/comment/star/fork/member/public/wiki/all), `plugin_activity_visibility` (public/all), `plugin_activity_timestamps` (bool), `plugin_activity_skipped`, `plugin_activity_ignored` |
| 出力        | `[]Event` … `{Type, Timestamp, Action, Activity, Repo, Issue?, Refs?, Files?}`                                                                                                                                                                                                                                                     |

### 2.3 `calendar`

| 項目        | 内容                                                                   |
| ----------- | ---------------------------------------------------------------------- |
| supports    | U                                                                      |
| scopes      | public_access                                                          |
| deps        | GraphQL `contributionsCollection.contributionCalendar` × 年ごと        |
| 主要 inputs | `plugin_calendar_limit` (年数, 0=全期間)                               |
| 出力        | `{Years: [{Year, Weeks: [{FirstDay, Days: [{Count, Date, Level}]}]}]}` |

### 2.4 `code`

| 項目         | 内容                                                                                                                                                              |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports     | U, O                                                                                                                                                              |
| scopes       | public_access                                                                                                                                                     |
| deps         | REST `/users/.../events`, `/repos/.../commits/{sha}`                                                                                                              |
| 主要 inputs  | `plugin_code_lines` (12), `plugin_code_load` (300), `plugin_code_days` (0), `plugin_code_visibility` (public/all), `plugin_code_skipped`, `plugin_code_languages` |
| 出力         | `{Snippet: { Owner, Repo, File, Language, Sha, Patch, Url }}`                                                                                                     |
| セキュリティ | プライベートリポジトリの内容が露出する恐れがあるため、ユーザーに警告。                                                                                            |

### 2.5 `contributors`

| 項目        | 内容                                                                                                                                                       |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | R                                                                                                                                                          |
| scopes      | public_access                                                                                                                                              |
| deps        | REST `/repos/.../stats/contributors`, GraphQL `commits(:after:..., :since:..., :until:...)`                                                                |
| 主要 inputs | `plugin_contributors_base`, `plugin_contributors_head`, `plugin_contributors_ignored`, `plugin_contributors_contributions`, `plugin_contributors_sections` |
| 出力        | `[]{Login, Avatar, Contributions, Categories}`                                                                                                             |

### 2.6 `discussions`

| 項目        | 内容                                                                         |
| ----------- | ---------------------------------------------------------------------------- |
| supports    | U                                                                            |
| scopes      | public_access                                                                |
| deps        | GraphQL `user.repositoryDiscussions`, `user.repositoryDiscussionComments`    |
| 主要 inputs | `plugin_discussions_categories` (bool)                                       |
| 出力        | `{Started, Answered, Comments, Upvotes, Categories: [{Name, Emoji, Count}]}` |

### 2.7 `followup`

| 項目        | 内容                                                                                              |
| ----------- | ------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                           |
| scopes      | public_access                                                                                     |
| deps        | GraphQL `repositoryOwner.repositories`, `search(type:ISSUE, query:'is:pr ...')`                   |
| 主要 inputs | `plugin_followup_sections` (repositories/user, default `repositories`), `plugin_followup_indepth` |
| 出力        | `{Issues: {Open, Closed}, PRs: {Open, Merged, Closed}}` × repositories/user                       |

### 2.8 `gists`

| 項目        | 内容                                                 |
| ----------- | ---------------------------------------------------- |
| supports    | U                                                    |
| scopes      | public_access                                        |
| deps        | GraphQL `user.gists` (`pageInfo.hasNextPage` ループ) |
| 主要 inputs | (なし、enable のみ)                                  |
| 出力        | `{TotalCount, Forks, Stargazers, Comments, Files}`   |

### 2.9 `habits`

| 項目        | 内容                                                                                                                                                                                                                                                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O                                                                                                                                                                                                                                                          |
| scopes      | public_access                                                                                                                                                                                                                                                 |
| deps        | REST `/users/.../events`, `/repos/.../commits/{sha}` (patch parsing)                                                                                                                                                                                          |
| 主要 inputs | `plugin_habits_from` (200), `plugin_habits_days` (14), `plugin_habits_facts` (true), `plugin_habits_charts` (false), `plugin_habits_charts_type` (classic/graph), `plugin_habits_trim` (false)                                                                |
| 出力        | `{Commits: {Days: map[Weekday]int, Day: string, Hours: map[int]int, Hour: string}, Indents: {Spaces, Tabs, Style}, Lines: {Average: {Chars: float}}, Linguist: { Available, Ordered: []Language }, Languages: { Available, Recent: map[string]int }, Charts}` |

### 2.10 `introduction`

| 項目        | 内容                                              |
| ----------- | ------------------------------------------------- |
| supports    | U, O, R                                           |
| scopes      | public_access                                     |
| deps        | GraphQL user/organization/repository              |
| 主要 inputs | `plugin_introduction_title`                       |
| 出力        | `{Text: string, Title: string}` (Markdown 化済み) |

### 2.11 `isocalendar`

| 項目        | 内容                                                                                                                    |
| ----------- | ----------------------------------------------------------------------------------------------------------------------- |
| supports    | U                                                                                                                       |
| scopes      | public_access                                                                                                           |
| deps        | GraphQL `contributionsCollection.contributionCalendar` (半年/1年)                                                       |
| 主要 inputs | `plugin_isocalendar_duration` (`half-year`/`full-year`)                                                                 |
| 出力        | `{Streak: {Max, Current}, Average: int, Sum: int, Reference: time.Time, Weeks: [...{Days: [...{Date, Count, Level}]}]}` |

### 2.12 `languages`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| scopes      | public_access                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| deps        | GraphQL `repository.languages`、(indepth) git clone + `go-enry`、(recent) REST events + commits                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 主要 inputs | `plugin_languages_ignored`, `plugin_languages_skipped`, `plugin_languages_limit` (8), `plugin_languages_threshold` (0%), `plugin_languages_other` (bool), `plugin_languages_colors` (json), `plugin_languages_aliases`, `plugin_languages_sections` (`most-used`/`recently-used`), `plugin_languages_details` (`bytes-size`/`percentage`/`lines`), `plugin_languages_indepth` (bool), `plugin_languages_indepth_custom`, `plugin_languages_analysis_timeout` (15m), `plugin_languages_analysis_timeout_repositories` (7.5m), `plugin_languages_recent_categories` (`markup`/`programming`), `plugin_languages_recent_load` (300), `plugin_languages_recent_days` (14), `plugin_languages_recent_loaded_paginated` |
| 出力        | §2 [プラグインインタフェース](./05-plugins.md#2-プラグインインタフェース) 参照                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

### 2.13 `licenses`

| 項目        | 内容                                                                                                                                      |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | R                                                                                                                                         |
| scopes      | public_access                                                                                                                             |
| deps        | リポジトリ clone (`license_detector` 系) または `licensee/licensed` バイナリ呼び出し                                                      |
| 主要 inputs | `plugin_licenses_setup`, `plugin_licenses_ratio` (bool), `plugin_licenses_legal` (bool)                                                   |
| 出力        | `{Default, Permissions: [{Name, Required}], Limitations, Conditions, Dependencies: {Total, Licenses, Unknown}}`                           |
| 備考        | コマンド注入リスクあり (Web で無効化推奨)。Go 版では外部バイナリ呼び出しを無効、Go ライブラリ (`github.com/google/licensecheck`) を採用。 |

### 2.14 `lines`

| 項目        | 内容                                                                                                                                                       |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                                    |
| scopes      | public_access                                                                                                                                              |
| deps        | REST `/repos/.../stats/contributors`                                                                                                                       |
| 主要 inputs | `plugin_lines_sections` (`base`/`repositories`/`history`), `plugin_lines_repositories_limit` (4), `plugin_lines_history_limit` (1), `plugin_lines_skipped` |
| 出力        | `{Added, Removed, Changed, Repositories: [{Repo, Added, Removed}], History}`                                                                               |

### 2.15 `notable`

| 項目        | 内容                                                                                                                                                                                                                                    |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U                                                                                                                                                                                                                                       |
| scopes      | public_access                                                                                                                                                                                                                           |
| deps        | GraphQL repositoryOwner, REST commits / Issues, organizations                                                                                                                                                                           |
| 主要 inputs | `plugin_notable_filter` (bool), `plugin_notable_repositories` (bool), `plugin_notable_indepth` (bool), `plugin_notable_types` (`commit`/`pull_request`/`issue`), `plugin_notable_from` (organization/all), `plugin_notable_self` (bool) |
| 出力        | `[]{Name, Url, Avatar, Stargazers, Handle, Description, Stats}`                                                                                                                                                                         |

### 2.16 `people`

| 項目        | 内容                                                                                                                                                                                                                                                                       |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                                                                                                                                                    |
| scopes      | public_access                                                                                                                                                                                                                                                              |
| deps        | GraphQL followers/following/sponsors/contributors/stargazers/watchers/members                                                                                                                                                                                              |
| 主要 inputs | `plugin_people_limit` (24), `plugin_people_types` (followers/following/...), `plugin_people_size` (28), `plugin_people_identicons` (bool), `plugin_people_identicons_hide` (bool), `plugin_people_thanks`, `plugin_people_sponsors_custom`, `plugin_people_shuffle` (bool) |
| 出力        | `{Types: []string, <type>: []{Login, Avatar, Url}, Size, Hide}`                                                                                                                                                                                                            |

### 2.17 `projects`

| 項目        | 内容                                                                                               |
| ----------- | -------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                            |
| scopes      | public_access, public_repo, read:project                                                           |
| deps        | GraphQL `user.projects`, `repository.projects`                                                     |
| 主要 inputs | `plugin_projects_limit` (4), `plugin_projects_repositories`, `plugin_projects_descriptions` (bool) |
| 出力        | `[]{Name, Body, Updated, Progress: {Done, InProgress, Todo, Enabled}}`                             |

### 2.18 `reactions`

| 項目        | 内容                                                                                                                                                                                                                                                                                          |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U                                                                                                                                                                                                                                                                                             |
| scopes      | public_access                                                                                                                                                                                                                                                                                 |
| deps        | GraphQL issues/comments                                                                                                                                                                                                                                                                       |
| 主要 inputs | `plugin_reactions_limit` (200), `plugin_reactions_limit_issues` (100), `plugin_reactions_limit_discussions` (100), `plugin_reactions_limit_discussions_comments` (100), `plugin_reactions_days` (0), `plugin_reactions_details` (`percentage`/`count`/`absolute`), `plugin_reactions_ignored` |
| 出力        | `{Total, Sample, Categories: map[Emoji]int}`                                                                                                                                                                                                                                                  |

### 2.19 `repositories`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                                                                 |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O                                                                                                                                                                                                                                                                                                                                                                 |
| scopes      | public_access                                                                                                                                                                                                                                                                                                                                                        |
| deps        | GraphQL `user.pinnedItems`, `repository`, `user.repositories`                                                                                                                                                                                                                                                                                                        |
| 主要 inputs | `plugin_repositories_featured`, `plugin_repositories_pinned` (number), `plugin_repositories_starred` (number), `plugin_repositories_random` (number), `plugin_repositories_affiliations`, `plugin_repositories_forks` (bool), `plugin_repositories_skipped`, `plugin_repositories_batch` (100), `plugin_repositories_order` (`featured`/`pinned`/`starred`/`random`) |
| 出力        | `{List: []Repository}`                                                                                                                                                                                                                                                                                                                                               |

### 2.20 `skyline`

| 項目        | 内容                                                                                                                                                                     |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| supports    | U                                                                                                                                                                        |
| deps        | `https://skyline.github.com/<login>/<year>.json` の 3D model、自前で SVG 化                                                                                              |
| 主要 inputs | `plugin_skyline_year`, `plugin_skyline_frames` (60), `plugin_skyline_quality` (1), `plugin_skyline_compatibility` (bool), `plugin_skyline_settings` (json: city mode 等) |
| 出力        | `{Frames: []ImageDataURI, City: bool, Year: int}`                                                                                                                        |

### 2.21 `sponsors`

| 項目        | 内容                                                                                                                                            |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                         |
| scopes      | read:user, read:org                                                                                                                             |
| deps        | GraphQL `user.sponsorsListing`, `sponsorsForViewer`                                                                                             |
| 主要 inputs | `plugin_sponsors_sections` (`sponsors`/`featured`/`about`), `plugin_sponsors_size` (28), `plugin_sponsors_title`, `plugin_sponsors_past` (bool) |
| 出力        | `{Sponsors: []Sponsor, About, Featured, Title}`                                                                                                 |

### 2.22 `sponsorships`

| 項目        | 内容                                                            |
| ----------- | --------------------------------------------------------------- |
| supports    | U, O                                                            |
| scopes      | read:user, read:org                                             |
| deps        | GraphQL `viewer.sponsorshipsAsSponsor` (`activeOnly: false`)    |
| 主要 inputs | `plugin_sponsorships_sections` (`amount`/`obtained`/`featured`) |
| 出力        | `{Amount: float, Obtained, Sponsorships: []Sponsorship}`        |

### 2.23 `stargazers`

| 項目        | 内容                                                                                                                                                                                                                         |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U, O, R                                                                                                                                                                                                                      |
| scopes      | public_access                                                                                                                                                                                                                |
| deps        | GraphQL `repository.stargazers(orderBy: STARRED_AT)`, location → geocoding                                                                                                                                                   |
| 主要 inputs | `plugin_stargazers_charts` (bool), `plugin_stargazers_charts_type` (`classic`/`graph`), `plugin_stargazers_worldmap` (bool), `plugin_stargazers_worldmap_token` (Google Maps API), `plugin_stargazers_worldmap_sample` (200) |
| 出力        | `{Total, Charts: [...], Worldmap: { Locations: []GeoLoc }}`                                                                                                                                                                  |

### 2.24 `starlists`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                                                                                               |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U                                                                                                                                                                                                                                                                                                                                                                                                  |
| extras      | `metrics.run.puppeteer.scrapping`                                                                                                                                                                                                                                                                                                                                                                  |
| deps        | chromedp (GitHub starred lists ページ scrape)                                                                                                                                                                                                                                                                                                                                                      |
| 主要 inputs | `plugin_starlists_languages` (bool), `plugin_starlists_limit` (8), `plugin_starlists_limit_repositories` (4), `plugin_starlists_languages_limit` (8), `plugin_starlists_ignored`, `plugin_starlists_languages_ignored`, `plugin_starlists_only`, `plugin_starlists_languages_details` (`bytes-size`/`percentage`/`lines`), `plugin_starlists_languages_indepth` (bool), `plugin_starlists_shuffle` |
| 出力        | `{Lists: []{Name, Description, Repositories: [...]}, Languages: {Stats, Colors, Total}}`                                                                                                                                                                                                                                                                                                           |

### 2.25 `stars`

| 項目        | 内容                                                    |
| ----------- | ------------------------------------------------------- |
| supports    | U                                                       |
| scopes      | public_access                                           |
| deps        | GraphQL `user.starredRepositories(orderBy: STARRED_AT)` |
| 主要 inputs | `plugin_stars_limit` (4)                                |
| 出力        | `{List: []Repository}`                                  |

### 2.26 `support` (deprecated)

| 項目     | 内容                                                                      |
| -------- | ------------------------------------------------------------------------- |
| supports | U                                                                         |
| extras   | `metrics.run.puppeteer.scrapping`                                         |
| deps     | chromedp (github.community プロファイル scrape)                           |
| 出力     | `{Topics, Replies, Posts, Reactions, Badges, Solutions, Likes}`           |
| 備考     | 上流が GitHub Discussions に移行。Go 版では deprecated とし実装は最小限。 |

### 2.27 `topics`

| 項目        | 内容                                                                                                                                                       |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| supports    | U                                                                                                                                                          |
| extras      | `metrics.run.puppeteer.scrapping`                                                                                                                          |
| deps        | chromedp (`https://github.com/stars/<login>/topics`)                                                                                                       |
| 主要 inputs | `plugin_topics_mode` (`icons`/`labels`/`mastered`), `plugin_topics_limit` (15), `plugin_topics_sort` (`stars`/`activity`/`recent`/`alphabetical`/`random`) |
| 出力        | `{List: []{Name, Description, Icon, Url}, Mode, Sort}`                                                                                                     |

### 2.28 `traffic`

| 項目        | 内容                            |
| ----------- | ------------------------------- |
| supports    | U, O, R                         |
| scopes      | repo                            |
| deps        | REST `/repos/.../traffic/views` |
| 主要 inputs | `plugin_traffic_skipped`        |
| 出力        | `{Views, ViewsUnique}`          |

---

## 3. Social / 外部 API プラグイン

### 3.1 `anilist`

| 項目        | 内容                                                                                                                                                                                                                                                               |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| deps        | AniList GraphQL `https://graphql.anilist.co`                                                                                                                                                                                                                       |
| 主要 inputs | `plugin_anilist_user`, `plugin_anilist_medias` (anime/manga/all), `plugin_anilist_sections` (favorites/characters/watching/reading), `plugin_anilist_limit`, `plugin_anilist_limit_characters` (22), `plugin_anilist_limit_reviews` (10), `plugin_anilist_shuffle` |
| 出力        | `{Anime: Stats, Manga: Stats, Characters: [...]}`                                                                                                                                                                                                                  |

### 3.2 `leetcode`

| 項目        | 内容                                                                                                                                                                                 |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| deps        | LeetCode GraphQL `https://leetcode.com/graphql`                                                                                                                                      |
| 主要 inputs | `plugin_leetcode_user`, `plugin_leetcode_sections` (solved/skills/recent), `plugin_leetcode_limit_skills` (15), `plugin_leetcode_ignored_skills`, `plugin_leetcode_limit_recent` (3) |
| 出力        | `{Solved: {Easy, Medium, Hard, Acceptance}, Skills, Recent}`                                                                                                                         |

### 3.3 `music`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                                                               |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| deps        | Spotify Web API, LastFM, AppleMusic, YouTube Music (provider 切替)                                                                                                                                                                                                                                                                                                 |
| 主要 inputs | `plugin_music_provider` (spotify/apple/lastfm/...), `plugin_music_token` (Spotify refresh token 等), `plugin_music_mode` (recent/playlist/top), `plugin_music_playlist` (URL), `plugin_music_limit` (4), `plugin_music_user` (LastFM 用), `plugin_music_played_at` (bool), `plugin_music_top_type` (tracks/artists), `plugin_music_time_range` (short/medium/long) |
| 出力        | `{Tracks: [{Artist, Name, Artwork, Duration, Played}]}`                                                                                                                                                                                                                                                                                                            |

### 3.4 `pagespeed`

| 項目        | 内容                                                                                                                                                      |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| deps        | `https://www.googleapis.com/pagespeedonline/v5/runPagespeed`                                                                                              |
| 主要 inputs | `plugin_pagespeed_token`, `plugin_pagespeed_url`, `plugin_pagespeed_detailed` (bool), `plugin_pagespeed_screenshot` (bool), `plugin_pagespeed_pwa` (bool) |
| 出力        | `{Scores: [{Score, Title}], Metrics: [{Name, Value}], Screenshot: data-uri}`                                                                              |

### 3.5 `posts`

| 項目        | 内容                                                                                                                                                                  |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| deps        | dev.to REST, Hashnode GraphQL, Medium (RSS)                                                                                                                           |
| 主要 inputs | `plugin_posts_source` (`dev.to`/`hashnode`/`medium`), `plugin_posts_user`, `plugin_posts_limit` (4), `plugin_posts_descriptions` (bool), `plugin_posts_covers` (bool) |
| 出力        | `{Articles: [{Title, Description, Cover, Url, Published}]}`                                                                                                           |

### 3.6 `rss`

| 項目        | 内容                                              |
| ----------- | ------------------------------------------------- |
| deps        | 任意 RSS フィード (`gofeed`)                      |
| 主要 inputs | `plugin_rss_source` (URL), `plugin_rss_limit` (4) |
| 出力        | `{Title, Items: [{Title, Url, Published}]}`       |

### 3.7 `stackoverflow`

| 項目        | 内容                                                                                                                                                                                                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| deps        | Stack Exchange API `https://api.stackexchange.com/2.3`                                                                                                                                                        |
| 主要 inputs | `plugin_stackoverflow_user`, `plugin_stackoverflow_sections` (answers-top/questions-top/answers-recent/questions-recent/lines), `plugin_stackoverflow_limit` (4), `plugin_stackoverflow_lines_snippet` (bool) |
| 出力        | `{Stats: { Reputation, Gold, Silver, Bronze }, Sections: map[string][]Post}`                                                                                                                                  |

### 3.8 `steam`

| 項目        | 内容                                                                                                                                                                                                                                                                                   |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| deps        | Steam Web API                                                                                                                                                                                                                                                                          |
| 主要 inputs | `plugin_steam_token`, `plugin_steam_user`, `plugin_steam_sections` (player/recent/most-played/playtime/achievements), `plugin_steam_recent_games_limit` (2), `plugin_steam_most_played_games_limit` (2), `plugin_steam_playtime_threshold` (10), `plugin_steam_achievements_limit` (2) |
| 出力        | `{Player: {...}, Recent: [...], MostPlayed: [...], Playtime: int, Achievements: [...]}`                                                                                                                                                                                                |

### 3.9 `tweets` (deprecated)

| 項目        | 内容                                                                                                       |
| ----------- | ---------------------------------------------------------------------------------------------------------- |
| deps        | Twitter API v2 (有料化につき deprecated)                                                                   |
| 主要 inputs | `plugin_tweets_token`, `plugin_tweets_user`, `plugin_tweets_attachments` (bool), `plugin_tweets_limit` (2) |
| 出力        | `{Tweets: [...]}`                                                                                          |
| 備考        | デフォルト無効。実装は残すが、無効化警告。                                                                 |

### 3.10 `wakatime`

| 項目        | 内容                                                                                                                                                                                                                                                                                                                                                                                       |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| deps        | `https://wakatime.com/api/v1/users/<user>/stats/<range>?api_key=<token>`                                                                                                                                                                                                                                                                                                                   |
| 主要 inputs | `plugin_wakatime_token`, `plugin_wakatime_user`, `plugin_wakatime_url` (self-hosted), `plugin_wakatime_sections` (time/projects/projects-graphs/languages/languages-graphs/editors/os), `plugin_wakatime_days` (7), `plugin_wakatime_limit` (4), `plugin_wakatime_languages_other` (bool), `plugin_wakatime_languages_ignored`, `plugin_wakatime_repositories_visibility` (`public`/`all`) |
| 出力        | §[02-action.md] のサンプル参照: `{Sections, Days, Total, Daily, Projects, Languages, Editors, OperatingSystems}`                                                                                                                                                                                                                                                                           |

---

## 4. community プラグイン

初版で取り込む community プラグイン対象:

| プラグイン        | カテゴリ | 概要                                    |
| ----------------- | -------- | --------------------------------------- |
| `crypto`          | social   | CoinGecko 等から暗号通貨価格を取得      |
| `nightscout`      | social   | Nightscout API から血糖値統計を取得     |
| `stock`           | social   | Alpha Vantage 等から株価を取得          |
| `chess`           | social   | chess.com API からチェス統計            |
| `splatoon`        | social   | Splatoon ランク (data scraping)         |
| `fortune`         | social   | ランダム fortune (ロード文字列)         |
| `poopmap`         | social   | poopmap.com からトイレマップ            |
| `screenshot`      | social   | 任意 URL の chromedp スクリーンショット |
| `16personalities` | social   | 16personalities.com 結果ページの scrape |

詳細仕様は別途、各 community plugin の `metadata.yml` に従う(全 input は同一規約)。
