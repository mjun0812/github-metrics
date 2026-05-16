# Feature Specification: GitHub プラグイン群 (M4)

**Feature Branch**: `004-m4-github-plugins`

**Created**: 2026-05-15

**Status**: Draft

**Input**: User description: "M4: GitHub プラグイン (T-031..T-051 相当, 21 タスク)"

## User Scenarios & Testing *(mandatory)*

本機能の「ユーザー」は 2 種類存在する。

- **GitHub Action 利用者 (一次)**: 自分の README に貼る SVG / PNG に、上流 `lowlighter/metrics` で見慣れた **言語ランキング・活動ログ・achievements バッジ・pinned repositories・contribution カレンダー** などのデータが入っていることを期待する利用者。M3 までは classic テンプレートに base/core プラグイン経由の最小プロフィール (avatar, login, name, bio, base statistics) しか入っておらず、SVG はほぼ空だった。
- **JSON 消費 downstream (二次)**: `output: json` で得られる `data.Plugins[<name>]` を CI / ダッシュボード / Bot 等で読む消費者。constitution 原則 II により、各プラグインの JSON shape は上流 `lowlighter/metrics` と完全互換でなければならない。

採用プラグイン 21 個 (`docs/design/15-selection-answer.md` §6 確定) を以下 3 つの user story に分割する。1 つでも届けば「README に意味あるデータが乗る」MVP が成立し、それぞれ独立にテスト可能。

### User Story 1 - MVP プラグイン 5 個で README が「意味あるデータ」を表示する (Priority: P1)

GitHub Action 利用者が、classic テンプレート出力 SVG に **languages 標準モード / activity / achievements / repositories / isocalendar** の 5 プラグイン分のデータが入った状態を得る。この 5 プラグインだけで上流 metrics の最頻採用構成 (主要言語の棒グラフ + 直近活動ログ + ランクバッジ + pinned 一覧 + contribution カレンダー) を再現する。

**Why this priority**: M3 PR #112 マージ時点では SVG は「avatar と login が載るだけの空白に近い画像」だった。本ストーリー完走で初めて「他人に見せたくなる画像」になる、M4 で最も価値が高いマイルストーン。GitHub PAT に追加スコープを要求しない (`public_access` のみで完結する) ので、認証回りの追加設計も不要で MVP に最適。

**Independent Test**: mocked `GraphQLMux` + `RESTMux` (M1 で導入済み) で octocat 相当のレスポンスを返し、`engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼ぶ。`Result.Output` の SVG に以下 5 つの DOM 痕跡がすべて含まれることを golden file で assert する:
1. `<g class="languages-progress">` などの languages partial 出力
2. `<text class="activity-event">` などの activity partial 出力
3. `<svg class="achievement">` または `data-achievement="<rank>"` のいずれかが 1 件以上
4. `<a class="repository">` で pinned repository が ≥ 1 件
5. `<g class="calendar">` の月格子 (isocalendar の半年分=26 週相当)

JSON 経路 (`Format:"json"`) では `data.Plugins.languages`, `.activity`, `.achievements`, `.repositories`, `.isocalendar` の 5 キーが non-null で含まれることを assert。

**Acceptance Scenarios**:

1. **Given** classic テンプレートと採用済みプラグインがすべて registry に登録され、mocked GraphQL/REST が octocat の `user`, `events`, `repositories`, `contributionsCollection`, `pinnedItems` を返す状態で **When** `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼ぶ **Then** `Result.MIME == "image/svg+xml"`、`Result.Output` には上記 5 つの DOM 痕跡がすべて含まれ、`Result.Errors` は空。
2. **Given** 同条件で **When** `engine.Compute(Format:"json")` を呼ぶ **Then** `data.Plugins` に上記 5 キーが non-null で含まれ、各キーの構造は上流 `lowlighter/metrics` の `data/plugins.<name>` と key / 型レベルで一致する (DOM ではなく key 一致、constitution 原則 II)。
3. **Given** `plugin_languages_limit=3` が `inputs` に渡る状態で **When** Compute を呼ぶ **Then** SVG 内の言語バーは上位 3 件のみ表示され、4 件目以降は `Other` 集約バーに合算される (上流既定挙動)。
4. **Given** `plugin_achievements_threshold="A"` が指定された状態で **When** Compute を呼ぶ **Then** rank `S`, `A` の achievement のみ SVG に出力され、`B` 以下は除外される。
5. **Given** mocked REST が 5xx を返す `events` エンドポイントを 1 つ含む状態で **When** Compute を呼ぶ **Then** activity プラグインのみ `Result.Errors` に `*RetryableError` を 1 件追加し、他 4 プラグインの出力は SVG に通常通り含まれる (部分失敗の許容)。

---

### User Story 2 - GraphQL/REST 単体で完結する拡張プラグイン群 (Priority: P2)

GitHub Action 利用者が、classic テンプレートに **habits / calendar / stars / people / notable / contributors / reactions / projects / sponsors / sponsorships / stargazers / traffic** の 12 プラグインのいずれかを有効化したとき、対応する partial と JSON が出力に含まれる。これらは既存 GraphQL/REST クライアントだけで動き、chromedp や新規ヘビーライブラリの追加が不要。

**Why this priority**: P1 後の自然拡張で、ユーザーが README をより豊かにできる。実装単位は粒度が揃っており並列実行可能 (各プラグインが独立パッケージ)。P2 完走で採用 21 個中 17 個 (= 81%) が動き、上流互換性ベンチマークの大半をカバーできる。

**Independent Test**: 各プラグイン単位で table test + golden file。代表として `traffic` のテストは: mocked REST `/repos/X/traffic/views` を 200 で返す + token に `repo` scope 相当 fixture を渡し、`engine.Compute` 後に `data.Plugins.traffic.views` が non-empty かつ `Result.Errors` 空であること、加えて scope 不在 fixture では `data.Plugins.traffic.skipped == true` + WARN ログが出ることを assert。

**Acceptance Scenarios**:

1. **Given** 12 プラグインそれぞれが registry に登録され、mocked GraphQL/REST が対応するレスポンスを返す状態で **When** `engine.Compute(Format:"json")` を呼ぶ **Then** `data.Plugins[<name>]` が non-null で、各構造体のフィールド名・型・null 表現は上流 `metrics.json` と完全互換。
2. **Given** `plugin_traffic` が有効かつ token に `repo` scope が無い状態で **When** Compute を呼ぶ **Then** traffic プラグインは `data.Plugins.traffic.skipped=true` を記録し、WARN ログ「`plugin traffic: missing repo scope, skipping`」を出力し、`Result.Errors` には何も追加しない (scope 不足は致命でない設計)。
3. **Given** `plugin_projects` 有効 + token に `read:project` 無し **When** Compute を呼ぶ **Then** 上記と同様 `skipped=true` + WARN、`Result.Errors` 空。
4. **Given** `plugin_stargazers_worldmap=true` が指定されたが `plugin_stargazers_worldmap_token` 未指定 **When** Compute を呼ぶ **Then** stargazers の基本 chart は出力されるが worldmap セクションだけ抜け落ち、`data.Plugins.stargazers.worldmap` は `null`、WARN ログを出す。
5. **Given** `plugin_sponsors_sections=["sponsors","past"]` (上流既定値の上書き) **When** Compute を呼ぶ **Then** sponsors partial には現役 + 過去スポンサーの両セクションが SVG に表示される。

---

### User Story 3 - chromedp / 重外部ライブラリ依存プラグイン (Priority: P3)

GitHub Action 利用者が、**languages.recent / languages.indepth / topics / starlists** の 4 プラグインを有効化したとき、それぞれが要求する追加ランタイム (chromedp スクレイピング、または go-git + go-enry のローカル clone 解析) を経由してデータを取得し、partial と JSON に組み込まれる。

**Why this priority**: 上流互換性の最後の隙間を埋めるが、運用上の制約 (CI で chromium 必須、ローカル clone のディスク・帯域消費、scope 不足時の degraded path) が他プラグインより重く、MVP には載らなくても許容できる。M3 で整えた `internal/render` chromedp 基盤を再利用するため M3 完了が前提。

**Independent Test**: 各プラグインを `//go:build chromedp` (topics / starlists) または `//go:build heavy` (languages.recent / languages.indepth) build tag で隔離した unit test に閉じる。CI の chromedp ジョブと heavy ジョブで個別に green を確認。代表として `languages.indepth` のテストは: 小さな git repository fixture を `t.TempDir()` 配下に clone し go-enry で 3 ファイル分類 → 結果が `data.Plugins.languages.indepth.bytes` map に集計されることを assert。

**Acceptance Scenarios**:

1. **Given** `plugin_languages_sections=["recently-used"]` + mocked REST `/users/.../events` が PushEvent 3 件を返し、各 commit に対する `/repos/X/commits/Y` が 200 を返す状態で **When** Compute を呼ぶ **Then** `data.Plugins.languages.recent` の集計に push commit 分の言語が反映され、`Result.Errors` 空。
2. **Given** `plugin_languages_indepth=true` かつ `metrics.cpu.overuse=true` (上流の extras フラグ) が未指定の状態で **When** Compute を呼ぶ **Then** languages.indepth は `skipped=true` を記録し WARN ログを出し、他プラグインの出力は影響を受けない。
3. **Given** `plugin_topics` 有効 + chromedp が利用可能 (`METRICS_CHROME_PATH`) + mocked HTTP 経由で `github.com/stars/<login>/topics` ページに 5 件の topic カードを返す状態で **When** Compute を呼ぶ **Then** `data.Plugins.topics` に 5 件のエントリ (name, description, icon, url) が入る。
4. **Given** `plugin_starlists` 有効 + chromedp 不在環境で **When** Compute を呼ぶ **Then** starlists は `data.Plugins.starlists.skipped=true` + WARN ログを出し、`Result.Errors` には `*RetryableError` を 1 件追加 (chromedp 不在は将来回復可能なので retryable 扱い)。

---

### Edge Cases

- **GraphQL secondary rate limit** (HTTP 200 + `errors[].type=="RATE_LIMITED"`): プラグイン単位で `*RetryableError` を `Result.Errors` に追加、当該プラグインの `data.Plugins[<name>]` は `null` または部分結果。他プラグインの実行は継続。
- **REST 1h hard cap** (`X-RateLimit-Remaining: 0`): リクエスト発出側 (`internal/githubapi/rest.go`) で wait 判断するが、ここに到達して block されたプラグインは個別 timeout で `*RetryableError`。
- **複数プラグインが同じ GraphQL ノード (例 `user.repositories`) を要求**: M2 base plugin が `Data.Computed.Repositories` に prefetch 済みの結果を `PluginImports.Get("base")` 経由で他プラグインが読む。`base` を 2 度実行しない MUST。
- **登録済みプラグインが Run 内で panic**: `core.RunPlugins` (M2 で実装済み) が `recover()` し `*PanicError` を `Result.Errors` に追加、他プラグインは継続。
- **`plugin_<name>` 未指定 (= default false)**: 該当プラグインは Run しない。`data.Plugins[<name>]` キーは欠落 (null ではない) MUST — 上流挙動に合わせる。
- **入力エイリアス**: `plugin_languages_limit` と `plugin_languages.limit` (上流 settings.json 経由) の両方が指定された場合、settings.json 経路の優先度を上流に合わせる (`metadata.yml` で resolution 規則を保持)。
- **大量ページング**: 1000 件超の repositories / events / stargazers がある場合、各プラグインは上流既定の上限値 (`plugin_<name>_limit` 系) で切り詰める。デフォルト無制限のキーは無制限のまま (例 `plugin_traffic` は repositories 全件)。
- **scope 不足**: `read:project` (projects), `read:user` (sponsors), `read:org` (sponsors org), `repo` (traffic) のいずれかが PAT に欠ける場合、該当プラグインのみ `skipped=true` を記録して continue。`Result.Errors` には追加しない (scope 設計は呼び出し側の責任)。
- **non-existent login**: `base` プラグインが `user/organization not found` を返した時点で全プラグインの Run はキャンセル MUST (依存元の `Data.User` が無いと意味がないため)。
- **`output_action=data-changed`**: 本 spec の範囲外 (M6 T-114)。ただし JSON 出力の決定性 (key 順序、null 表現) は data-changed の hash 比較が成立する前提なので、本 M4 のすべての JSON 出力で安定 marshaling を MUST 保つ。

## Requirements *(mandatory)*

### Functional Requirements

#### プラグイン登録・実行

- **FR-001**: System MUST 採用 21 プラグインそれぞれを `internal/plugins/<name>/<name>.go` に独立パッケージとして実装し、`init()` から `plugins.Register(Plugin)` で global registry に登録する (M1 で確立済みの規約に従う)。
- **FR-002**: System MUST 各プラグインの `Run(ctx, pc)` が以下を満たす:
  - `pc.Data.Account` に応じて user / organization / repository モードを分岐し、自身が `supports` (`docs/design/06-plugins-detail.md` 各節) に該当しないアカウント種別では即座に `(nil, nil)` を返す
  - 返り値の `any` は `pc.Data.Plugins[<name>]` に core 経由で格納される構造体ポインタ
  - context cancellation を検知して即座に return する
- **FR-003**: System MUST 各プラグインの公開構造体 (例 `languages.Result`, `activity.Result`) のフィールド名 / 型 / `json:` タグを上流 `data.plugins.<name>` と完全一致させる (constitution 原則 II)。
- **FR-004**: System MUST `pc.Inputs` から `plugin_<name>` (boolean) が `true` または truthy 文字列 (`"yes"`, `"on"`) のときのみ該当プラグインを Run する。default false。
- **FR-005**: System MUST 各プラグインが解釈する `plugin_<name>_*` キー名・既定値・型を上流 `metadata.yml` と完全一致させる (constitution 原則 I)。未知キーは黙って素通り MUST。

#### データ整合性 (base / 他プラグイン間の依存)

- **FR-006**: System MUST `base` プラグインを M4 で完成形に拡張する:
  - **organization** ブランチで `organization.members.nodes(first:N)` を `plugin_repositories_batch` 既定値でページング (T-018)
  - **indepth** モード (`plugin_repositories=true` かつ `plugin_repositories_pinned` 等の indepth フラグが真) で commits / issues / pull-requests の追加クエリを発行 (T-019)
  - **repositories paging** で 100 件超を `after` cursor で再帰取得し、batch-halving (上流挙動) で 502 / timeout 時に batch を半分にしてリトライ (T-020)
- **FR-007**: System MUST `base` 完了後の `pc.Data.Computed.Repositories`, `pc.Data.User`, `pc.Data.Organization` を他プラグインが `PluginImports.Get("base")` で読めるようにする。base の重複実行は MUST NOT。
- **FR-008**: System MUST 各プラグインの相互依存 (`languages.recent` が `languages` の言語分類ロジックを再利用する、`starlists` が `topics` のスクレイピング基盤を再利用するなど) を `PluginImports.Get()` 経由で表現し、import sort 等に依存した実行順序前提を作らない MUST。

#### エラー処理・部分失敗

- **FR-009**: System MUST 1 プラグインが返したエラーは `Result.Errors` に typed error として追加し、他プラグインの実行を継続 MUST。`die=true` 入力が指定された場合のみ最初のエラーで Compute を短絡 MAY (上流挙動)。
- **FR-010**: System MUST scope 不足 (`read:project`, `read:user`, `read:org`, `repo`) を検出したプラグインは `data.Plugins[<name>].skipped=true` を記録し、`Result.Errors` には追加しない MUST。WARN ログ 1 行は出す MUST。
- **FR-011**: System MUST chromedp / heavy 依存プラグイン (`languages.recent`, `languages.indepth`, `topics`, `starlists`) が必要ランタイム不在で Run できないとき、`skipped=true` + WARN ログ + `*RetryableError` を 1 件 `Result.Errors` に追加する。

#### 性能・並列性

- **FR-012**: System MUST 各プラグインを `core.RunPlugins` (M2 実装済) 経由で並列実行可能とする。プラグインは pure function (`pc` 以外の global state 書き込み MUST NOT) であること。`pc.Data` への書き込みは core が排他制御。
- **FR-013**: System MUST 各プラグインの p95 実行時間を mocked 環境で 1 秒以下 (`Compute` 全体 5 秒以下) に収める (constitution Technical Constraints)。

#### テスト

- **FR-014**: System MUST 各プラグインに対し `internal/plugins/<name>/<name>_test.go` でテーブルテストを提供する。最低 5 ケース (正常系 + 入力上限 + scope 不足 + API エラー + 空応答)。
- **FR-015**: System MUST 各プラグインの SVG partial 出力を `tests/golden/classic/<plugin>.svg` の golden file で検証する。golden 更新は `-update` フラグ経由で明示的に行う MUST (constitution 原則 IV)。
- **FR-016**: System MUST 上流互換性テスト `tests/compatibility/json_test.go` を全プラグインを含む完全 JSON で再走させ、上流 `metrics.json` と key レベル diff 0 を達成する。
- **FR-017**: System MUST chromedp / heavy プラグインを `//go:build chromedp` / `//go:build heavy` で隔離し、CI の通常ジョブ (`go test ./...`) は chromium / linguist 不要で緑になる MUST。

#### スコープ規律

- **FR-018**: System MUST 不採用プラグインのコードを M4 で **追加しない** MUST (constitution 原則 III)。具体的には:
  - **GitHub-system 系 9 個** (`docs/design/06-plugins-detail.md §2`): `code`, `discussions`, `followup`, `gists`, `introduction`, `licenses`, `lines`, `skyline`, `support` (deprecated)
  - **Community 系 10 個** (`docs/design/06-plugins-detail.md §3`): `anilist`, `leetcode`, `music`, `pagespeed`, `posts`, `rss`, `stackoverflow`, `steam`, `tweets` (deprecated), `wakatime`

  `tests/compliance/compliance_test.go` (M2 で導入済) の scan 対象に `internal/plugins/` 配下を含めて regression を防ぐ。

### Key Entities

- **Plugin**: 1 つの GitHub データソースを表す Go パッケージ。`Name() / Metadata() / Run()` を実装する `plugins.Plugin` インタフェース satisfier。21 個。
- **PluginContext**: `Run` に渡される I/O 束 (settings / inputs / logger / REST / GraphQL / data / imports / metadata)。M1 で確立済み、M4 では一切変更しない MUST。
- **Data.Plugins**: `map[string]any` の thread-safe な書き込み先。各プラグインは自身の名前をキーに結果を 1 件だけ書く。重複書き込みは last-write-wins (core が直列化済)。
- **PluginResult shape** (各プラグイン固有): 上流 `lowlighter/metrics` の `data.plugins.<name>` と完全互換な構造体。
- **PluginMetadata**: `metadata.yml` 由来の入力スキーマ (key 名 / 既定値 / 型)。各プラグインは自身の `Metadata()` で対応するメタデータをロードし、入力検証を委譲する MAY。
- **PluginImports**: 他プラグインの結果を読む read-only ハンドル。実装は core が `Data.Plugins` を wrap する。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 採用 21 プラグインすべてが `internal/plugins/<name>/` パッケージとして実装され、`internal/plugins.Registry()` 経由で名前解決でき、各プラグインの `Run` が mocked GraphQL/REST 下で error なく完走する (`go test ./internal/plugins/... -count=1` 緑、build tag 隔離テストは別ジョブ)。
- **SC-002**: classic テンプレートが採用 21 プラグインの partial をすべて持ち、`engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` の出力 SVG に **少なくとも 17 プラグイン (= P1+P2)** の DOM 痕跡が含まれる。P3 の 4 プラグインは build tag 経由のみ追加。
- **SC-003**: `engine.Compute` のフルパス (21 プラグイン並列、mocked deps) が **p95 5 秒以下** で完走する (constitution Technical Constraints + M3 SC-003 と整合)。
- **SC-004**: 上流互換性テスト `tests/compatibility/json_test.go` で octocat fixture に対する JSON 出力 (採用 21 プラグイン分) の key/型 diff が **0** (constitution 原則 II)。SVG は DOM diff を 5 件以下に抑える (絶対 0 ではない、見た目同等を維持できれば許容)。
- **SC-005**: 各プラグインの **テスト網羅率**: 行カバレッジ ≥ 70% (`go test ./internal/plugins/... -cover`)、各プラグインに最低 5 ケースのテーブルテスト + 1 つの golden file が存在する。
- **SC-006**: GraphQL secondary rate limit / REST 5xx / scope 不足のいずれかが mocked で発生したとき、**他 20 プラグインは引き続き green**で完走する (= 部分失敗の許容)。
- **SC-007**: 不採用プラグイン (28 個) のコードが `internal/plugins/` 配下に **存在しない** ことを `tests/compliance/compliance_test.go` が機械的に検証する (constitution 原則 III)。
- **SC-008**: chromedp/heavy 隔離テスト (`go test -tags=chromedp ./internal/plugins/topics/...` 等) が CI で green。通常 `go test ./...` は chromium / linguist 不在環境でも green を維持する。
- **SC-009**: `engine.Compute` のフルパスで CPU メモリピークが **800 MB 以下** (mocked 並列 21 プラグイン)。実 GitHub API は呼ばない (mocked)。

## Assumptions

- M2 の `engine.Compute` / `core.RunPlugins` / `base.Plugin` の基本骨格はそのまま再利用する。新規の plugin interface 改変は MUST NOT。
- M3 の `internal/render` パイプライン (`render.Renderer`, `render.Apply`, octicon/CSS/XML 装飾) はそのまま再利用する。
- 上流 `lowlighter/metrics` のソース (`./org_repo`) は git untracked のまま参照 only として手元に存在する (M1 で確立済み, `.gitignore`)。
- `metadata.yml` の入力スキーマは M1 で取り込み済み、本 spec ではキー追加・型変更を伴うほどの差分は無い (採用 21 プラグインのキーはすべて metadata.yml に既出)。
- GitHub PAT のスコープは `public_access` (= 認証なし or 最低限) + 必要に応じ `repo` (traffic) / `read:project` (projects) / `read:user`, `read:org` (sponsors)。Action 利用者が自分で組み合わせる前提で、本 spec では「scope 不足を検出して skipped で抜ける」道のみ用意する。
- chromedp 経路 (P3 の `topics`, `starlists`) は M3 で導入した `render.Browser` を再利用する。スクレイピング用の独立した chromedp インスタンスは作らない MUST。
- heavy 経路 (P3 の `languages.recent`, `languages.indepth`) は `go-enry` + `go-git` の新規依存を `go.mod` に追加する想定。代替検討は `/speckit-plan` の research フェーズで実施。
- 上流 `metrics.json` 互換のため JSON marshaling は **decimal 数値の文字列化なし**、boolean / null 表現は Go 既定の json encoder 結果に従う。順序保証が必要な map は struct / `json.RawMessage` で表現する。
- 本 spec の対象は **プラグイン実装と classic partial の組み立て**まで。`output_action` (commit / pull-request / data-changed) は M6 範疇で扱わない。
