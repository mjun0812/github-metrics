# Feature Specification: プロジェクト土台 (M1 19 タスク一括)

**Feature Branch**: `001-project-foundation`

**Created**: 2026-05-15

**Status**: Draft

**Input**: User description: "M1 19 タスク (T-001..005, 007..010, 012..018, 020..022) を 1 つの『土台機能』として spec 化。go.mod / cmd-internal レイアウト / settings.json+action.yml ローダ / metadata.yml / engine スケルトン / mock 基盤など、後続全機能の前提となる骨格。"

## Clarifications

### Session 2026-05-15

- Q: Go module path (`go.mod` の `module` directive) として確定する値は何か? → A: `github.com/mjun0812/github-metrics` (作者 user id: `mjun0812`)

## User Scenarios & Testing *(mandatory)*

本機能の「ユーザー」は 2 種類存在する。

- **コードベース貢献者 (一次ユーザー)**: 本リポジトリで実装・レビュー・CI 運用を行う開発者。土台の上に M2 以降の機能を積む。
- **将来の GitHub Action 利用者 (二次ユーザー)**: M6 完了後に `uses: mjun0812/github-metrics@v1` として参照する README オーナー。土台単体では機能を消費できないが、入力互換性の保証点として最初に検証される。

各 user story はそれぞれ独立にテスト可能であり、上位ストーリーから順に実装することで段階的に土台が稼働状態へ近づく。

### User Story 1 - ビルド可能で CI 緑のプロジェクト骨格 (Priority: P1)

貢献者がリポジトリをクローンして `make build` と `make test` を実行すると、Go モジュールが解決され、`cmd/metrics-action` および `cmd/metrics-cli` の空エントリポイントが両方ビルドできる。CI (PR 時) では `go test ./...` / `go vet ./...` / `golangci-lint run` / `govulncheck ./...` が走り、すべて緑になる。

**Why this priority**: これが満たされなければ後続のいかなる実装も start できない。constitution 原則 V (Go 規約) を満たす土壌そのものであり、最初の `Closes T-001` を成立させる前提でもある。

**Independent Test**: 新規開発マシン上でリポジトリをクローン → `make build && make test` 実行 → CI が PR 上で緑になることを確認するだけで完結する。後続のローダや mock 基盤は不要。

**Acceptance Scenarios**:

1. **Given** クリーンなクローン環境で **When** `make build` を実行する **Then** `bin/metrics-action` と `bin/metrics-cli` の両バイナリが artifacts として生成され、`--help` が exit 0 を返す。
2. **Given** 空のテストパッケージ群 (例: `internal/logger/logger_test.go` の最小ケース) で **When** `make test` を実行する **Then** すべての test がパスする。
3. **Given** PR を開いた状態で **When** `.github/workflows/go-ci.yml` がトリガされる **Then** `go vet` / `golangci-lint run` / `govulncheck ./...` がそれぞれ独立 step として実行され、4 つすべて緑になる。
4. **Given** 共通ロガー (`internal/logger`) と共通エラー型 (`internal/errors`) と context ヘルパ (`WithLogin` / `LoginFromContext`) が用意された状態で **When** 任意のパッケージから `slog.Info("msg", "login", login)` 相当を発行する **Then** ログに `login` 属性が含まれ、`errors.As` / `errors.Is` で各エラー種別 (InputError / NotFoundError / ForbiddenError / UnsupportedFormatError / RetryableError) が判別できる。

---

### User Story 2 - 上流互換の設定入力レイヤ (Priority: P1)

貢献者および GitHub Action 利用者が、上流 `lowlighter/metrics` と同じ `action.yml` インプットおよび `settings.json` キーを与えると、土台がそれを内部正規化済みの入力マップへ変換し、コードから一意に参照できる状態にする。

**Why this priority**: constitution 原則 I (入力互換性) は移植の存在意義そのものであり、骨格段階で satisfaction を確定させないと後続のすべての feature が再度互換性議論を引き起こす。`metadata.yml` と inputs パーサがなければ M2 以降の plugin 実装が始められない。

**Independent Test**: 採用 21 plugin + base/core + classic/repository の `metadata.yml` を `embed.FS` から読み、`tests/fixtures/inputs/*` に置いた YAML ケースを `Inputs.ForAction(env, preset)` に通して期待マップと一致することを golden file 比較で確認する。GitHub API 呼び出しは不要。

**Acceptance Scenarios**:

1. **Given** リポジトリルートに上流互換の `settings.json` (`//` コメントキー含む) が存在する状態で **When** `config.Load(path)` を呼ぶ **Then** `//` キーは破棄され、`Settings.Port` / `Settings.Modes` / `Settings.Plugins[...]` 等のフィールドに上流と同一意味で値が入る。
2. **Given** `Sandbox=true` の設定で **When** loader を起動する **Then** ファイル読み込みがスキップされ、`optimize=true / cached=0 / plugins.default=true / extras.default=true / mocked=true` が強制設定される。
3. **Given** action 環境変数として `INPUTS` JSON および `INPUT_<UPPER>` が与えられた状態で **When** `Inputs.ForAction(env, preset)` を呼ぶ **Then** `metadata.yml` の `type` (string / number / boolean / array / token / json) に従って正規化された値が `q` マップに格納され、`.user.login` プレースホルダは `data.User.Login` で解決される。
4. **Given** `type: token` の入力で値が与えられた状態で **When** その入力を `String()` する **Then** 文字列表現は `(provided)` を返し、生トークンはログ・エラーに漏れない。
5. **Given** 上流の `assets/plugins/<name>/metadata.yml` を全件 (採用 21 個 + base/core) と `assets/templates/{classic,repository}/metadata.yml` を embed した状態で **When** `MetadataLoader.Load(fsys)` を呼ぶ **Then** すべての `inputs` 定義 (`type`, `default`, `format`, `values`, `global`, `preset`, `extras` を含む) が欠落なく構造体に読み込まれる。

---

### User Story 3 - GitHub API クライアントとレート可視化 (Priority: P2)

貢献者が plugin を実装する際に、mocked または本物の GitHub に対する REST / GraphQL / レート照会の薄いラッパを通じてアクセスでき、`MOCKED_TOKEN` 経路では実通信が一切起きないことを保証する。

**Why this priority**: M4 (プラグイン 21 個) と M6 (Action) のすべてが、この層に依存する。レート可視化は token validation (T-108) の前提でもある。原則 IV (mock 経路を通らない外部呼び出しは panic) を骨格段階で固定するため P2 だが、US1/US2 と並行着手可能。

**Independent Test**: `httptest.NewServer` を立てて REST `/rate_limit`、GraphQL endpoint へのリクエストを mock し、リトライ・status code 別挙動・User-Agent ヘッダを assert する。token 種別判定 (`gh[pousr]_` / `github_pat_` / `NOT_NEEDED` / `MOCKED_TOKEN`) も同じテストで網羅。

**Acceptance Scenarios**:

1. **Given** `httptest.Server` が 503 を 2 回返したのち 200 を返すよう設定された状態で **When** HTTP クライアントが GET を発行する **Then** 指数バックオフで合計 3 回試行し、最終的に 200 のレスポンスを返す。4xx に対しては再試行しない。
2. **Given** mocked REST backend が `GET /rate_limit` を返す状態で **When** `RateTracker.Refresh(ctx)` を呼ぶ **Then** `Resources.REST.Remaining` / `Resources.GraphQL.Remaining` / `Resources.Search.Remaining` がレスポンスの値で更新され、race detector でも clean となる。
3. **Given** `genqlient` で生成された base クエリ (`user`, `user.x`) を持つ状態で **When** mocked GraphQL backend を相手に `NewGraphQL(token).RunQuery(ctx, "user", vars)` を呼ぶ **Then** クエリ名で fixture が選択され、デシリアライズ済みの Go 構造体が返る。
4. **Given** token に `github_pat_` プレフィックスが与えられた状態で **When** クライアント生成を試みる **Then** 早期に拒否され `InputError` が返る。
5. **Given** `MOCKED_TOKEN` を環境に設定した状態で **When** 何らかの GitHub クライアントが本物の URL に対し HTTP リクエストを発行する **Then** 即座に panic し、テスト網羅性の取りこぼしを検出する。

---

### User Story 4 - 並列実行可能な plugin / template レジストリ (Priority: P2)

貢献者が plugin および template を `init()` で `Register()` するだけで、`engine` から名前解決でき、`core` プラグインが errgroup で並列実行し、panic は recover、エラーは `data.Plugins[name] = err` に集約される。

**Why this priority**: M4 で 21 個の plugin を独立に PR で積み上げるための受け皿。これがないと plugin 1 個目の実装で engine 統合を余儀なくされ、スコープが膨張する。

**Independent Test**: 3 つのテスト用 plugin (1 つは正常、1 つは error 返却、1 つは panic) を `RegisterForTest` で登録し、`core.RunPlugins(ctx, parallel=3)` を呼んで全 plugin の終了とエラー集約を assert する。実 GitHub API も実 template も不要。

**Acceptance Scenarios**:

1. **Given** 2 つの plugin が同じ名前で `Register` を試みる状態で **When** 2 つ目を呼ぶ **Then** panic が発生し、二重登録が検出できる。
2. **Given** 3 つの test plugin (success / error / panic) を登録した状態で **When** `core.RunPlugins(ctx)` を errgroup 並列度 3 で実行する **Then** すべての plugin 完了後にコンテキストが返り、`data.Plugins["success"] == result`、`data.Plugins["error"] == err`、`data.Plugins["panic"] == recoveredErr` がセットされる。
3. **Given** `core` plugin が `config.timezone=Asia/Tokyo` を受けた状態で **When** `core.Run(ctx, *PluginContext)` を呼ぶ **Then** `data.Config.Timezone.Name == "Asia/Tokyo"` となり、`data.Computed` の zero value が初期化される。
4. **Given** template が `Register("classic", t)` で登録された状態で **When** `templates.Get("classic")` を呼ぶ **Then** 該当 `Template` が返り、未知の名前では `NotFoundError` が返る。
5. **Given** `errgroup.SetLimit(parallel)` で並列度 1 を指定した状態で **When** 同じ 3 plugin を実行する **Then** 順次実行され、それでもエラー集約結果は並列実行時と同等。

---

### User Story 5 - エンドツーエンドでのデータ取得結線 (Priority: P3)

貢献者が `engine.Compute(ctx, req, deps)` を呼ぶと、mocked GitHub から `data.User` / `data.User.ContributionsCollection` / `data.User.Calendar` / `data.Computed.Repositories.*` が populate された internal `data` 構造体が返る。出力 (SVG/JSON) はまだ生成しない (M2 以降)。

**Why this priority**: US1〜US4 が独立に完了した後、骨格全体が「組み立てたら動く」ことの最終確認。M2 で classic template と JSON 出力が積まれた時点で初めて user-facing 価値になる。骨格段階では internal 状態が正しく populate されることだけを assert する。

**Independent Test**: mocked GraphQL/REST + base+core + 1 つの no-op test plugin で `engine.Compute` を呼び、戻り値の `data` を golden file (`tests/golden/foundation_data.json`) と比較する。SVG/PNG 生成は対象外。

**Acceptance Scenarios**:

1. **Given** mocked GitHub が `user` クエリで `octocat` を返す状態で **When** `engine.Compute(ctx, Request{Login: "octocat", Template: "noop"}, deps)` を呼ぶ **Then** `result.Data.User.Login == "octocat"` であり、`result.Errors == nil` となる。
2. **Given** mocked GitHub が 250 件 (3 ページ) のリポジトリを返す状態で **When** `base.Run` 内のページングが完了する **Then** `data.Computed.Repositories.Count == 250` となり、`Stargazers/Forks/Releases` などの集計が確定する。
3. **Given** account 種別が `organization` の状態で **When** `engine.Compute` を呼ぶ **Then** field 単位 fallback で `packages` / `sponsorshipsAsSponsor/Maintainer` / `membersWithRole` が個別取得され、bulk 失敗時にも data が埋まる。
4. **Given** bulk クエリ `user.x` が一部 field で 502 を返す状態で **When** `base.Run` が走る **Then** 失敗した field のみ unit query で再取得され、最終結果が他経路と同等になる。
5. **Given** `die=true` フラグの状態で **When** いずれかの plugin が error を返す **Then** `engine.Compute` は即座に該当 error を返し、後続 plugin は実行されない。

---

### Edge Cases

- **`settings.json` が存在しない場合**: ファイル未検出は欠損として扱い、`Settings{Port: 3000}` のみで起動を継続する (Web インスタンスは本 MVP では実装しないが、ローダの挙動として上流互換を維持)。
- **`metadata.yml` に未知 input が含まれる場合**: 解析時に panic / hard error にせず warn ログを残して該当 input をスキップする (上流互換維持と前方互換性のため)。
- **token が `NOT_NEEDED` の状態で GitHub API 呼び出しを試みる場合**: クライアント側で `InputError` を返し、HTTP 通信は発生させない。
- **embed.FS から要求した asset が見つからない場合**: 起動時に fail-fast (`os.Exit(2)`) し、assets の不足をビルド成果物の段階で検出する。
- **errgroup の並列度を `0` または負数で指定**: `parallel <= 0` の場合は `runtime.GOMAXPROCS(0)` を採用する。
- **HTTP リクエストが context timeout に到達**: リトライしない (timeout は dispatcher 上位の責務)。
- **`config.timezone` に不正な IANA 名を指定**: `time.LoadLocation` の失敗を `data.Config.Timezone.Error` フィールドに記録し、UTC にフォールバック。

## Requirements *(mandatory)*

### Functional Requirements

#### スケルトンとビルド

- **FR-001**: System MUST `go.mod` を `module github.com/mjun0812/github-metrics`、Go 1.23 以上で初期化し、`cmd/metrics-action/main.go` および `cmd/metrics-cli/main.go` を空 main としてビルド可能にする。
- **FR-002**: System MUST `Makefile` に `build` / `test` / `lint` / `bench` / `gen` / `docker` / `e2e` ターゲットを定義し、`make build` で両バイナリの artifacts を `bin/` に出力する。
- **FR-003**: System MUST `.github/workflows/go-ci.yml` で PR トリガにより `go test ./...` / `go vet ./...` / `golangci-lint run --timeout=10m` / `govulncheck ./...` を独立 step で実行し、全 step 緑を merge ゲートとする。
- **FR-004**: System MUST `internal/logger` を `log/slog` ベースで提供し、`debug` フラグでレベル切替・JSON/text 切替を可能にする。
- **FR-005**: System MUST `internal/errors` に `InputError` / `NotFoundError` / `ForbiddenError` / `UnsupportedFormatError` / `RetryableError` を定義し、`errors.Is` / `errors.As` で個別識別可能にする。
- **FR-006**: System MUST `internal/context` 相当のヘルパ (`WithLogin`, `LoginFromContext`) を提供し、ログに `login` 属性を自動付与可能にする。
- **FR-007**: System MUST `internal/format` に `Format`, `FormatBytes`, `FormatPercentage`, `FormatDate`, `Ellipsis`, `S`, `FormatError` を実装し、境界値 (0, 999, 1000, 999999, 1000000) のテーブルテストで検証する。

#### 設定 / メタデータ / 入力レイヤ (constitution 原則 I)

- **FR-008**: System MUST `settings.json` を上流互換でロードし、`//` プレフィックスのキーをコメントとして読み飛ばす。ファイル不在時は `Settings{Port: 3000}` を返し、`Sandbox=true` では `optimize/cached/plugins.default/extras.default/mocked` を強制設定する。
- **FR-009**: System MUST `assets/plugins/<name>/metadata.yml` (採用 21 + base + core)、`assets/templates/{classic,repository}/metadata.yml`、ルート `action.yml`、`assets/version.txt` を `embed.FS` 経由でロードし、すべての `inputs` 定義フィールド (`type`, `default`, `format`, `values`, `global`, `preset`, `extras`) を保持する。
- **FR-010**: System MUST `Inputs.ForAction(env, preset)` / `Inputs.ForWeb(query)` / `Inputs.ForData(data, q, account)` を提供し、`type` ごとの正規化 (string / number / boolean / array / token / json / array format = comma-separated / space-separated / newline-separated) を上流の `legacy-converter` と同等の規則で行う。
- **FR-011**: System MUST 動的プレースホルダ `.user.login`, `.repository.name` 等を `data` から解決し、`metadata.to.Action(key)` / `metadata.to.Web(key)` / `metadata.to.Query(key)` の変換関数で目的別キー名を取得できるようにする。
- **FR-012**: System MUST `type: token` の入力が `String()` 経由で出力されたとき `(provided)` を返し、ログ・エラー・JSON ダンプのいずれにも生トークンが漏れないこと。

#### GitHub API レイヤ

- **FR-013**: System MUST `internal/httpx.Client` に `Get` / `PostJSON` / `PostForm` / `Binary` を実装し、5xx / 429 / network エラーで指数バックオフリトライ、4xx は再試行なし、User-Agent は `metrics/<version> (+https://github.com/mjun0812/github-metrics)` とする。
- **FR-014**: System MUST `internal/githubapi/rest.go` に `NewREST(token, customBaseURL)` を実装し、mock 用 `*http.Client` 差し替えを許可する。`internal/githubapi/auth.go` で token 種別 (`gh[pousr]_` / `github_pat_` / `NOT_NEEDED` / `MOCKED_TOKEN`) を判定し、`github_pat_` は早期拒否する。
- **FR-015**: System MUST `internal/githubapi/graphql.go` に `NewGraphQL(token, customBaseURL)` を実装し、`Khan/genqlient` で `assets/plugins/base/queries/*.graphql` から型付き Go 関数を生成する。
- **FR-016**: System MUST `internal/githubapi/rate.go` に `Resources{REST, GraphQL, Search}` 構造体 (`Limit/Used/Remaining/Reset`) を実装し、`Refresh(ctx)` で `GET /rate_limit` から更新可能にする。並行アクセスは race detector で clean。
- **FR-017**: System MUST `MOCKED_TOKEN` 環境下で実 GitHub URL への HTTP リクエストが発生したとき即座に panic することで、テスト網羅性の取りこぼしを検出する。

#### Embed / Asset パイプライン

- **FR-018**: System MUST `assets/plugins/*` (採用 21 + base + core: queries / metadata / examples) と `assets/templates/{classic,repository}/*` を `//go:embed assets/*` でバンドルし、ビルド時に外部 fetch を不要にする。
- **FR-019**: System MUST `internal/tools/gen-octicons/main.go` と `internal/tools/gen-twemoji/main.go` を提供し、`go generate ./...` または `make gen` で `assets/octicons/data.json` および `assets/twemoji/index.json` を再生成可能にする (固定 commit を参照)。

#### Plugin / Template レイヤ

- **FR-020**: System MUST `internal/plugins/plugin.go` に `Plugin interface { Name() string; Metadata() PluginMetadata; Run(ctx context.Context, pc *PluginContext) (any, error) }` と `Register(p Plugin)` / `Get(name) (Plugin, bool)` / `Each(fn)` / `RegisterForTest(p)` を実装する。二重登録は panic。
- **FR-021**: System MUST `internal/templates/template.go` に `Template` インタフェースと `Register/Get` / `PartialFunc` / `PartialContext` を実装し、`Check(q, account, format)` で format / account / repository 必須を検証する。
- **FR-022**: System MUST `internal/plugins/base/base.go` に user / organization の `Run` を実装し、bulk クエリ (`user.x.graphql`) を試行 → 失敗 field のみ unit query で fallback する処理を備える。
- **FR-023**: System MUST `internal/plugins/base/repositories.go` に repositories のページング (timeout 時に batch 半減リトライ含む) を実装し、`repositories_forks/affiliations/skipped` を適用したうえで `data.Computed.Repositories.{Stargazers, Forks, Releases, ...}` を集計する。
- **FR-024**: System MUST `internal/plugins/core/core.go` に `core` plugin を実装し、`config.timezone`, `config.animations`, `config.display`, `config.base64`, `debug.flags` の解釈と `data.Computed` の zero value 初期化を行う。タイムゾーン解決失敗時は `data.Config.Timezone.Error` に記録し UTC にフォールバック。
- **FR-025**: System MUST `internal/plugins/core/run_plugins.go` に `errgroup.SetLimit(parallel)` ベースの並列実行を実装し、各 plugin のエラーは `data.Plugins[name] = err`、panic は recover してエラー化する。`parallel <= 0` は `runtime.GOMAXPROCS(0)` を採用する。

#### エンジン結線

- **FR-026**: System MUST `internal/engine/engine.go` に `Compute(ctx, req, deps) (Result, error)` を実装し、以下の順序を保証する: テンプレート存在検証 → Convert 決定 → partial 順マージ (template 側委譲) → Imports 構築 → base plugin → template.Run (M1 段階では noop 可) → goroutine 群完了待ち → エラー集約。`die=true` で即時 fail、`die=false` で集約継続。

#### 共通

- **FR-027**: System MUST 採用していない (`docs/design/15-selection-answer.md` §7 の) 機能 — Web インスタンス、Markdown / PDF 出力、Insights、community plugins、ソーシャル/外部 API plugins — に属するコードを新規追加しないこと (constitution 原則 III)。
- **FR-028**: System MUST `./org_repo` を参照のみとし、git 履歴に含めないこと。コードのコピー&ペーストは禁止し、Go で書き起こすこと (constitution 原則 V および Development Workflow)。
- **FR-029**: System MUST すべてのソースコードのコメント / docstring / identifier を英語で記述し、ユーザードキュメント (本 spec を含む) を日本語で記述すること。
- **FR-030**: System MUST すべての公開関数および exported 型に対しテーブルテストを用意し、出力構造を持つ機能 (formatters / metadata loader / engine.Compute) には golden file テスト (XML 正規化 + ハッシュ比較、`-update` フラグで明示更新) を用意すること。

### Key Entities *(include if feature involves data)*

- **Settings**: 上流 `settings.json` と互換のグローバル設定。Web インスタンスを実装しなくてもキー集合は維持する (原則 I)。`Sandbox`, `Token`, `Modes`, `Plugins`, `Templates`, `Optimize`, `Mocked` 等。
- **PluginMetadata / TemplateMetadata / ActionMetadata / PackageMetadata**: `metadata.yml` の構造写像。`inputs` の `type`, `default`, `format`, `values`, `global`, `preset`, `extras` を保持。
- **Inputs**: action env / web query / data context のいずれかから組み立てる正規化済み入力マップ。`type: token` は表示時に `(provided)` を返す不透明値として扱う。
- **PluginContext**: plugin が `Run` 時に受け取るコンテキスト。`Settings`, `Inputs`, `Logger`, `HTTPClient`, `REST`, `GraphQL`, `Data` への参照を含む。
- **Data**: engine が組み立てる内部状態。`User`, `Account` (`user` | `organization` | `repository`), `Config`, `Computed.Repositories.*`, `Plugins[name]`, `Errors[]`。M1 では SVG/JSON 化対象としては未使用 (M2 以降)。
- **Resources (rate)**: `REST`, `GraphQL`, `Search` の `Limit/Used/Remaining/Reset`。
- **Error types**: `InputError`, `NotFoundError`, `ForbiddenError`, `UnsupportedFormatError`, `RetryableError`。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: クリーンなクローン環境で `make build && make test` がローカル PC で 3 分以内に完了し、CI workflow (`go test ./...` + `go vet ./...` + `golangci-lint run` + `govulncheck ./...`) が PR 上で 7 分以内に緑になる。
- **SC-002**: 上流 `lowlighter/metrics` の `action.yml` および採用範囲の `metadata.yml` を **キー差分ゼロ** で読み込める。互換性 diff ツール (`make check-compat` 等) を実装し、上流と本実装のキー集合差を空にする。
- **SC-003**: 採用 21 plugin + base + core + classic/repository テンプレートの metadata がすべて `embed.FS` 経由で起動時に 200 ms 以内にロード完了する。
- **SC-004**: `MOCKED_TOKEN` 経路で実 GitHub URL への HTTP リクエストが発生したケース 0 件 (CI ジョブで panic 検出されない状態を維持)。
- **SC-005**: 土台 19 タスク (T-001..005, T-007..010, T-012..018, T-020..022) のうち、Acceptance criteria を満たす自動テストが各タスクにつき 1 件以上存在し、unit test coverage が `internal/logger` / `internal/errors` / `internal/format` / `internal/config` で 80% 以上。
- **SC-006**: mocked GitHub backend を用いた integration test (US5) で `engine.Compute` が 1 ユーザー分のデータを 2 秒以内に組み立てる (chromedp 抜き、`testing.B` で `BenchmarkCompute_Foundation_Mocked` を追加)。
- **SC-007**: `golangci-lint run` の重大度 `error` 件数 0 件。`govulncheck ./...` の既知脆弱性報告 0 件。両者は CI で merge ブロックする。
- **SC-008**: `./org_repo` のファイルが `git status` で常に ignored 表示であり、`git log -- org_repo/` が空であること (constitution 強制)。

## Assumptions

- Go バージョンは 1.23 系を採用する。constitution 「Go (latest stable)」運用に基づき、`go.mod` の `go` directive で固定する。1.22 以下サポートは MUST NOT。
- リポジトリ名 (module path) は `github.com/mjun0812/github-metrics` で確定 (Clarifications 2026-05-15 参照)。作者 user id は `mjun0812`。
- `assets/plugins/*` および `assets/templates/{classic,repository}/*` の **ソース** は `./org_repo/source/plugins/*` および `./org_repo/source/app/web/statics/.../templates/*` から **手動コピーではなく** ライセンス踏襲のうえで `make sync-assets` 相当のスクリプトで取得する (constitution Development Workflow)。スクリプトの実装は `T-012` に内包する。
- `metadata.yml` の互換性は **キー名と型** の単位で判定する。上流が将来追加するキーは前方互換 (warn してスキップ) で扱う。
- `engine.Compute` の M1 段階での `template.Run` は no-op (空 SVG / 空 string) で構わない。実描画は M2 (T-023 classic) で satisfaction する。
- 本 spec は CI 環境として GitHub Actions runner (`ubuntu-latest`, `macos-latest` のサブセット) を前提とする。Docker 同梱は M10 で扱い、M1 段階では `docker` ターゲットの placeholder のみ提供する。
- `internal/testutil/mocks` (REST / GraphQL モック) の実装本体は M9 (T-118 / T-119) で行うが、M1 段階でも US3 / US5 を成立させるための **最小スタブ** を `internal/githubapi` 内に local test helper として置く。M9 で full 実装に置き換える。
- 上流の `golangci-lint` ルール集合は新規に組成する。`staticcheck` / `gosec` / `revive` / `gofumpt` を有効化し、誤検知が多いルールは個別 disable する (記録は `.golangci.yml` のコメント)。
- 本 feature の完了基準は「土台 19 タスク全部の Acceptance criteria が緑」とする。一部だけ完了した中間状態でも US1〜US4 は独立に着地可能なので、PR は段階的に分割可。
