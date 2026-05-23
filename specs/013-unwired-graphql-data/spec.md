# Feature Specification: 未配線 GraphQL plugin の data-fetch wiring (013)

**Feature Branch**: `013-unwired-graphql-data`

**Created**: 2026-05-23

**Status**: Draft

**Input**: User description: "未配線の採用 plugin 6 個 (sponsors / sponsorships / projects / notable / stargazers / repositories.Pinned) の GraphQL data-fetch wiring を完成させる。各 plugin の Run() で適切な GraphQL query を実行し、Result.Skipped を解消、JSON / SVG partial への配線を実 GitHub データで動作可能にする。partial 自体は spec 011 で配線済 (HTML/SVG DOM は upstream parity)。本 spec は data fetch 側のみを対象とする。採用機能定義 (docs/design/15-selection-answer.md) の §4.2 で採用が明示されている。"

## Clarifications

### Session 2026-05-23

- Q: notable plugin の取得範囲は basic のみ / pinned discussion 込み / 全 indepth のいずれか → A: **basic モードのみ** (stargazer / fork / name)。pinned discussion / 最新 release / indepth は 014 以降で対応。理由は rate limit cost 抑制と spec 011 の partial が basic カードのみを期待する DOM。
- Q: stargazers の Chart.Series 粒度は week / month / day のいずれか → A: **month**。upstream parity の表示粒度に揃え、stargazers connection の `starredAt` を月単位で bucketize して cumulative count を吐く。day だと rate cost と SVG ノード数が増えすぎ、week は upstream 非対称。
- Q: sponsors plugin の tier (`monthlyPriceInDollars`) を JSON / partial に載せるか → A: **載せない**。privacy 優先で `login` / `avatarUrl` / `entityType` のみ。tier 表示は将来オプション化 (`plugin_sponsors_tier=yes` 等) 検討。
- Q: stargazers / notable / sponsors / sponsorships / projects / repositories.Pinned の対象アカウント kind は user / organization / 両方のうちどれか → A: **user-mode のみ**。Assumptions に既述のとおり organization は 014 以降。今 spec の Run はすべて `viewer` 起点 (= 認証 token の owner = user) で実装する。
- Q: GraphQL fetch が SC-003 (rate cost +50) を超えそうな場合の挙動は Skipped / 部分 fetch / 全件強行 のどれか → A: **per-plugin Skipped + Data.Errors に `*RetryableError`**。`go-github`/`pc.GraphQL` の rate tracker (T-010) で残り cost を測れるので、各 plugin Run 冒頭で `pc.GraphQL.RateBudget() < estimated_cost` のとき Skipped + Reason=`rate budget too low` で早期 return。他 plugin は継続。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sponsors / Sponsorships の実データ表示 (Priority: P1)

GitHub Sponsors の maintainer 側 (受け取り) と sponsor 側 (送金) の関係を、token に `read:user` および `read:org` scope がある状態で actual に取得し、classic テンプレートの「Sponsors」「Sponsorships」セクションに反映する。これまでは Run が空 List を返し partial が "Sponsors (0)" のように空表示になっていた。

**Why this priority**: 採用機能定義 §4.2 で `sponsors` / `sponsorships` が採用済。実際の sponsor 関係は GitHub プロファイル表現の中心的なソーシャル情報であり、現状の "空 List" は誤情報 (関係が無いように見える)。

**Independent Test**: token に必要 scope を付け、`go run ./cmd/metrics-cli --plugin "plugin_sponsors=yes"` を実行して JSON の `data.plugins.sponsors.list` が non-empty で active sponsor の avatar URL / login を含むこと、classic SVG の `<section data-section="sponsors">` 内に対応 avatar が render されることで検証。

**Acceptance Scenarios**:

1. **Given** token に `read:user` / `read:org` scope あり、対象ユーザーが GitHub Sponsors の maintainer、**When** sponsors plugin を enable して metrics を render、**Then** JSON 出力に `sponsors.list[]` (active sponsor の login / avatar) が現れ、SVG にも対応 avatar が visible
2. **Given** 同 scope 付き token、対象ユーザーが他者を sponsor している、**When** sponsorships plugin を enable して render、**Then** JSON 出力に `sponsorships.list[]` (target maintainer の login / avatar) と goal / about が現れる
3. **Given** scope 不足、**When** plugin を enable、**Then** Skipped=true + 既存の scope-gate メッセージ ("missing read:user / read:org scope") を保ち、partial は何も出さない (現状維持)

---

### User Story 2 - Projects / Notable / Stargazers の実データ表示 (Priority: P1)

`projects` (GitHub Projects v2 一覧)、`notable` (重要 contribution の一覧)、`stargazers` (自分が貰った star の時系列)を、necessary scope を持つ token で実取得し、partial に反映する。これらは現在いずれも Skipped または 空 List を返している。

**Why this priority**: いずれも採用済 (§4.2)。`notable` と `stargazers` は contributor profile の "重み" を伝える主要な指標で、partial 側 (011) ではすでに完全な DOM を出力できる状態にあるため、Run の wiring だけで User Story が成立する。

**Independent Test**: token を付けて各 plugin を個別に enable し、対応する JSON フィールド (`projects.list` / `notable.list` / `stargazers.chart`) が非空でレンダリングされること、partial で `<section data-section="projects">` 等が空にならないことを golden / e2e で確認。

**Acceptance Scenarios**:

1. **Given** `read:project` scope 付き token、`projects` plugin enable、**When** render、**Then** JSON `projects.list[]` に project name / description / updated date が現れ、partial に列挙される
2. **Given** PAT (public scope のみ) + `notable` plugin enable、**When** render、**Then** JSON `notable.list[]` に repositoryOwner traversal の結果 (notable contribution の repo + role 集計) が現れる
3. **Given** PAT、`stargazers` plugin enable、**When** render、**Then** JSON `stargazers.chart.series[]` に時系列の累積 star count が現れ、partial の bar chart に反映される

---

### User Story 3 - repositories.Pinned の Featured 連動表示 (Priority: P2)

`repositories.Pinned` は `viewer.pinnedItems` (GitHub プロファイル上で pin した repository) を取得する必要がある。現状は Featured のコピーが入っており、本来表示すべき "ピン留め" を反映していない。

**Why this priority**: 採用済 (§4.2) だが、Featured が代わりに表示されているため一見動作する偽装状態。ユーザーが Pinned を意図的に enable する利用シナリオの正確性のために P2 で実装する。なお Starred は 012 PR で別途実装済。

**Independent Test**: 対象 user が pin した repository を GitHub 上で設定、`plugin_repositories_pinned=yes` を渡して render、JSON `repositories.pinned[]` に Featured と異なる repo が含まれていることを検証。

**Acceptance Scenarios**:

1. **Given** target user が 3 repos を pin している、PAT 付き、**When** `plugin_repositories_pinned=yes` で render、**Then** `repositories.pinned[]` に 3 repos が pin 順で現れる (Featured と内容が独立)
2. **Given** target user が一切 pin していない、**When** 同上、**Then** `repositories.pinned[]` は空配列 (Skipped にはしない)
3. **Given** `pc.GraphQL == nil` (test path)、**When** 同上、**Then** Featured のコピーが返る (M4 baseline の後方互換)

---

### Edge Cases

- **Rate limit hit**: GraphQL の rate limit cost が大きいクエリ (notable は user の owned repos を traverse する可能性あり) で 429 / secondary limit に達した場合、`*RetryableError` を Data.Errors に積み、その plugin の Result.Skipped=true + SkippedReason="rate limit exceeded" を返す。他 plugin は継続。
- **Token has no required scope**: 既存 scope-gate ロジック (`pc.GraphQL.Scopes(ctx)` 経由) を流用。scope 不足は Skipped を返し SkippedReason をユーザーに分かる形で残す。
- **GraphQL API 一時的な失敗 (5xx)**: `*RetryableError` を Data.Errors にし、partial は空文字列を返す (partial 側で Skipped 判定済)。
- **空データ (sponsor 0 人 / pin 0 件 / etc.)**: Skipped=false で list を空配列にする。partial が "0 Sponsor(s)" のような 0-state を出すかどうかは partial 既存挙動に従う。
- **Account kind ミスマッチ**: stargazers は元来 repository / user / organization の三モード対応。Run が現在 "stargazers requires repository account kind (M7 territory)" で常時 Skipped を返している点を見直し、user-account モード (自分の所有 repos の累積 stargazers chart) も生成可能にする。

## Requirements *(mandatory)*

### Functional Requirements

#### 全 plugin 共通

- **FR-001**: 各 plugin の Run() は、`pc.GraphQL == nil` の場合に既存の Skipped path (または M4 baseline の fallback) を維持しなければならない (FR-006 of 012 と同様、test path の後方互換)。
- **FR-002**: 各 plugin の Run() は、GraphQL fetch が失敗した場合に `*RetryableError` を `pc.Data.Errors` に追加し、`Result.Skipped=true` + 人間可読な `SkippedReason` を返さなければならない。同 PR の他 plugin に副作用が及んではならない。
- **FR-003**: 各 plugin の Run() は、token が必要な OAuth scope を持たない場合に Skipped=true を返し、SkippedReason に明示的な scope 名を含めなければならない (既存実装と一致)。
- **FR-004**: JSON 出力の shape (フィールド名 / 型) は M4 baseline と互換でなければならない (追加 OK、削除 / rename NG)。
- **FR-005**: 各 plugin の partial.go (spec 011 の DOM 構造) は変更してはならない。本 spec は data fetch 側のみを対象とする。

#### plugin 固有

- **FR-006** (sponsors): `viewer.sponsorshipsAsMaintainer(first: N)` を取得し、各 sponsorship の `sponsor { login, avatarUrl, __typename }`、および `viewer.sponsorsListing { goal { title, percentComplete, description } }` を `Result.Goal` / `Result.Active` / `Result.Past` にマップしなければならない。tier の価格 (`tier.monthlyPriceInDollars` 等) は privacy 配慮で JSON にも partial にも載せない。M4 で定義済の `Result` フィールドを優先。
- **FR-007** (sponsorships): `viewer.sponsorshipsAsSponsor(first: N)` を取得し、対象 maintainer の `login` / `avatarUrl` / `__typename` を `Result.List` にマップしなければならない。
- **FR-008** (projects): `viewer.projectsV2(first: N, query: $filter)` を取得し、各 project の `title` / `shortDescription` / `updatedAt` / `url` / `closed` を `Result.List` にマップ。`read:project` scope 不足は既存 Skipped path を維持。
- **FR-009** (notable): `repositoryOwner(login: $login)` 経由で対象 user の owned repositories (`first: N`) を取得し、各 repo の `stargazerCount` / `forkCount` / `nameWithOwner` / `description` を集約。indepth (pinned discussion / 最新 release 等) は本 spec のスコープ外 (014 以降)。
- **FR-010** (stargazers): mode を `user` に拡張し (現状は repository のみ M7 territory として Skipped、organization は backlog)、user モードでは `viewer.repositories(first: N, ownerAffiliations: [OWNER])` から各 repo の `stargazers(first: 100) { nodes { starredAt } }` を **月単位で bucketize** し、`Chart.Series` (X=月の開始日、Y=cumulative star count) を組み立てる。週単位 / 日単位への変更は将来オプション化を検討するが本 spec のスコープ外。
- **FR-011** (repositories.Pinned): `viewer.pinnedItems(first: 6, types: [REPOSITORY])` を取得し、各 repo を `plugins.Repository` 型にマップして `Result.Pinned` に格納。`pc.GraphQL == nil` の場合は M4 baseline の Featured コピー fallback を維持。

#### テストと品質

- **FR-012**: 各 plugin に「happy path (GraphQL から N 件取得)」「scope 不足」「GraphQL 5xx」「空データ」の最低 4 ケースの table-driven test を追加しなければならない。
- **FR-013**: `tests/compliance/compliance_test.go` の `TestCompliance_M4_AdoptedPlugins` を破らない (採用 19 plugin の集合は変更しない)。
- **FR-014**: 既存の golden test (tests/golden/classic/m4/ 等) は本 spec で破壊してはならない。partial が変わらないため golden 不変が期待される。新しい mock fixture を fetch test 用に追加するのは可。
- **FR-015**: 視覚検証として、6 plugin それぞれの実 GitHub データ render (PNG または SVG) を spec dir 配下に置き、PR 説明から参照できる形で残さなければならない。

### Key Entities

- **Sponsorship**: maintainer → sponsor (受け取り) または sponsor → maintainer (送金) の関係。属性: `login`, `avatarUrl`, `entityType (User|Organization)`, `tierMonthlyPriceUSD` (option), `isActive`。
- **ProjectV2**: GitHub Projects v2 の単一プロジェクト。属性: `title`, `shortDescription`, `updatedAt`, `url`, `closed`。
- **NotableContribution**: 対象 owner の note-worthy repo。属性: `nameWithOwner`, `stargazerCount`, `forkCount`, `description`, `role (Maintainer|Contributor)`。
- **StargazerSeries**: 時系列 star 累積。属性: `series []Point` ここで `Point = {Date time.Time, CumulativeCount int}`。
- **PinnedRepository**: pinned items 表示用 repository。属性: 既存 `plugins.Repository` と同型 (NameWithOwner, Description, StargazerCount, etc.)。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 採用 plugin の Skipped 数が、`go run ./cmd/metrics-cli --plugin all=yes` (もしくは equivalents) において 6 plugin 減少する (sponsors / sponsorships / projects / notable / stargazers / repositories.Pinned が non-Skipped path に乗る)。
- **SC-002**: 実 GitHub data (mjun0812 アカウント、PAT 付き) で render した classic SVG に、上記 6 plugin に対応する `<section data-section="...">` がそれぞれ visible に表示される。
- **SC-003**: GraphQL rate limit cost が 1 render あたりの追加分で +50 points 以下に収まる (既存ベースから +50 / 5000 points = 1% 程度に抑える)。
- **SC-004**: 新規 table-driven test 全パス、既存 golden test を 1 件も壊さない、`go test ./... -race` で 100% green。
- **SC-005**: PR 説明に 6 plugin それぞれの render 結果 (PNG または SVG) リンクが含まれており、レビュアーが視覚的に動作確認可能。

## Assumptions

- 採用機能定義 `docs/design/15-selection-answer.md` §4.2 が source of truth。
- GraphQL fetch のクライアントは `internal/githubapi/graphql.go` + `internal/githubapi/graphql_gen.go` 経由。新クエリの追加は `internal/githubapi/queries/*.graphql` + `gen-graphql` ツールで生成。
- partial.go の DOM は spec 011 で配線済で、本 spec では変更しない。
- 既存の `pc.GraphQL.Scopes(ctx)` 経路でscope探知が可能。新しい OAuth flow は必要ない。
- token は既存の `INPUT_TOKEN` 入力経路で渡される (Action / CLI 共通)。
- 視覚検証は spec 011 同様 `scripts/capture-mjun-references.sh` の手順を流用可能 (token と Chromium が手元にある前提)。
- 本 PR が merge される時点で PR #383 (011) と PR #384 (012) は main に取り込まれていること (両 PR の data shape を前提とする)。
- `notable` / `stargazers` の rate cost 削減のため、初版は user-mode (= viewer の owned repos のみ) に限定し、organization-mode は backlog (014 以降) に回す。
