# 12. Issue 化用タスク分解

> ⚠️ **重要 — このドキュメントは upstream の全機能を網羅したリストです (採用外を含む)**
>
> 本プロジェクトでは upstream `lowlighter/metrics` の **subset** のみを実装しています。
> Phase 順序やタスクの採用判断には **このファイルを直接使わないでください**。代わりに:
>
> - **採用機能定義**: [15-selection-answer.md](./15-selection-answer.md)
> - **MVP タスク順序**: [16-tasks-mvp.md](./16-tasks-mvp.md)
>
> 採用 phase 順序: `M1 → M2 → M3 → M4 → **M6** → M7 → M9 → M10`
>
> **非ゴール (実装禁止)**:
>
> - **M5** (Web インスタンス: chi server / OAuth / insights 等の HTTP 公開機能)
> - **M8** (ソーシャル / 外部 API plugin: anilist / leetcode / chess / steam / music / pagespeed / tweets / stackoverflow / wakatime 等)
>
> 本ファイル内の M5 / M8 セクションのタスクは upstream 設計の完全性のため残してあるだけで、
> 本プロジェクトでは着手しません。

本ドキュメントは Go 移植を **GitHub Issue 駆動** で進めるためのタスクリストです。
各タスクはそのまま issue 本文に貼れる粒度で記述されています。

- ID (`T-001` 形式) を issue 内の参照キーとして使ってください (`Closes T-XXX` 等)。
- ラベル例:
  - `phase:M1` 〜 `phase:M10`
  - `area:engine` / `area:plugins` / `area:render` / `area:server` / `area:action` / `area:templates` / `area:config` / `area:infra` / `area:test`
  - `priority:high` / `priority:medium` / `priority:low`
  - `kind:feature` / `kind:chore` / `kind:test` / `kind:docs`
  - `good-first-issue` (T-001〜T-013 の素直な実装系から候補)
- `Blocks` / `Blocked by` で依存を明示しています。issue 作成時に GitHub の「linked issue」で連結してください。

## 目次

- [Phase M1: 基盤](#phase-m1-基盤)
- [Phase M2: コアエンジン + classic 最小](#phase-m2-コアエンジン--classic-最小)
- [Phase M3: レンダリングパイプライン](#phase-m3-レンダリングパイプライン)
- [Phase M4: 主要 GitHub プラグイン](#phase-m4-主要-github-プラグイン)
- [Phase M5: Web インスタンス](#phase-m5-web-インスタンス)
- [Phase M6: GitHub Action / CLI](#phase-m6-github-action--cli)
- [Phase M7: Markdown / 派生テンプレート](#phase-m7-markdown--派生テンプレート)
- [Phase M8: ソーシャル / 外部 API プラグイン](#phase-m8-ソーシャル--外部-api-プラグイン)
- [Phase M9: community プラグイン + テスト整備](#phase-m9-community-プラグイン--テスト整備)
- [Phase M10: リリース / DevOps / ドキュメント](#phase-m10-リリース--devops--ドキュメント)

---

## Phase M1: 基盤

### T-001: リポジトリ初期化 (go.mod, ディレクトリ構成, Makefile, CI スケルトン)

**Labels:** `phase:M1` `area:infra` `kind:chore` `priority:high`

**背景:**
Go 移植の最初のコミット。既存 Node 実装と並走するため、Go コードは `cmd/`, `internal/`, `assets/`, `api/` の各ディレクトリに新規追加する。Node 側ファイル(`source/`, `package.json` 等)はこのフェーズでは削除しない。

**スコープ:**
- `go.mod` 作成 (`module github.com/lowlighter/metrics`, Go 1.23)。
- ディレクトリスケルトン作成 ([01-architecture.md §2](./01-architecture.md#2-go-パッケージ構成) の構成に従う)。
- `Makefile` (build / test / lint / bench / gen / docker / e2e)。
- `.github/workflows/go-ci.yml`: PR で `go test ./...`, `go vet ./...`, `golangci-lint run`, `govulncheck ./...` を実行。
- README に Go 移植中である旨を追記 (要相談、別 issue にしても可)。

**Acceptance criteria:**
- [ ] `make build` で `bin/metrics-action`, `bin/metrics-server` の空 main がビルドできる。
- [ ] `make test` がパスする (空テストで OK)。
- [ ] `golangci-lint` が CI で実行され緑になる。
- [ ] CODEOWNERS / dependabot 設定が `internal/`, `cmd/` をカバーする (任意)。

**Related specs:** [01-architecture.md](./01-architecture.md)

---

### T-002: ロガー / エラー / context ヘルパ実装

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**背景:**
全パッケージで一貫した log 出力 (`slog`) と error wrapping を行うため、共通ヘルパを最初に揃える。

**スコープ:**
- `internal/logger`: `slog.Default` 設定 (JSON / text 切替、`debug` フラグで level 切替)。
- `internal/errors`:
  - `errors.New(code, msg, wrap)` ラッパ。
  - `engine.InputError`, `engine.NotFoundError`, `engine.ForbiddenError`, `engine.UnsupportedFormatError`, `engine.RetryableError` 等を定義 ([01-architecture.md §6](./01-architecture.md#6-エラーモデル))。
- `context` ヘルパ: `WithLogin(ctx, login)`, `LoginFromContext(ctx)` (ログのスコープ自動付与)。

**Acceptance criteria:**
- [ ] エラー型は `errors.As` / `errors.Is` で識別可能。
- [ ] `slog.Default()` の出力に `login`, `stage` などのスコープが入る。
- [ ] テストカバレッジ > 80%。

**Blocks:** T-003〜T-013, T-014, T-016

**Related specs:** [01-architecture.md §6](./01-architecture.md#6-エラーモデル), [01-architecture.md §7](./01-architecture.md#7-ロギングトレース)

---

### T-003: settings.json ローダ (// コメントキー対応)

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:**
Web インスタンスの起点となる設定読み込み。Node 版は `JSON.parse` が二重キーを後勝ち解釈していたため、Go 側で `//` キーを捨てるパーサが必要。

**スコープ:**
- `internal/config/settings.go`:
  - `Settings` 構造体 ([09-configuration.md §1.2](./09-configuration.md#12-スキーマ))。
  - `Load(path string) (*Settings, error)`:
    - 存在しなければ `&Settings{Port:3000}` を返す。
    - `tidwall/gjson` または `json.RawMessage` で `//` キーを枝刈り → `json.Unmarshal`。
- `Sandbox` フラグで読み込みをスキップ、`optimize=true / cached=0 / plugins.default=true / extras.default=true / sandbox=true / mocked=true` を強制する処理。
- `getter NoToken() bool` (`token == "NOT_NEEDED"`)。

**Acceptance criteria:**
- [ ] `tests/fixtures/settings/*.json` を用意し、`Load` 成功 / 失敗ケースを網羅。
- [ ] サンプル `settings.example.json` (既存) をロードしてもエラーにならない。
- [ ] `Sandbox=true` で強制設定が適用される。

**Blocked by:** T-001, T-002

**Related specs:** [09-configuration.md §1](./09-configuration.md#1-settingsjson-web-インスタンス)

---

### T-004: metadata.yml ローダ (plugins / templates / action / package)

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:**
すべての入力解釈・出力検証の元になるメタデータローダ。`assets/plugins/<name>/metadata.yml`, `assets/templates/<name>/metadata.yml`, `action.yml`, `assets/version.txt` を読み込む。

**スコープ:**
- `internal/config/metadata.go`:
  - `PluginMetadata`, `TemplateMetadata`, `ActionMetadata`, `PackageMetadata` 構造体 ([05-plugins.md §3.2](./05-plugins.md#32-go-表現))。
  - `Load(fsys fs.FS) (*MetadataLoader, error)`。
  - `embed.FS` で `assets/plugins/**`, `assets/templates/**` をバンドル。
- `metadata.to.Action(key)`, `metadata.to.Web(key)`, `metadata.to.Query(key)` 相当の変換関数。
- `PluginMetadata.Extras(name, settings) bool` 実装。

**Acceptance criteria:**
- [ ] 既存 `source/plugins/*/metadata.yml` を全部読み込みできる。
- [ ] `Inputs` セクションのうち `type`, `default`, `format`, `values`, `global`, `preset`, `extras` をすべて保持。
- [ ] テンプレート metadata の `formats`, `supports`, `extends` を保持。

**Blocked by:** T-001

**Related specs:** [09-configuration.md §4](./09-configuration.md#4-metadatayml-ローダ)

---

### T-005: 入力パーサ (型変換 + 動的プレースホルダ)

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:**
`metadata.yml` の `type: boolean/number/string/array/json/token` に従って値を正規化する。`.user.login` 等の動的プレースホルダ解決も担う。

**スコープ:**
- `internal/config/inputs.go`:
  - `NormalizeInput(def InputDef, raw any) (any, error)`。
  - `Inputs.ForAction(env, preset)`, `Inputs.ForWeb(query)`, `Inputs.ForData(data, q, account)`。
- `array` の format(`comma-separated` / `space-separated` / `newline-separated` の組合せ)に対応。
- `values:` ホワイトリスト適用。
- `min`, `max` clamp。

**Acceptance criteria:**
- [ ] `tests/unit/config/inputs_test.go` で各 type のテーブルテストパス。
- [ ] `.user.login` のような placeholder が `data.User.Login` から解決される。
- [ ] `token` 型はマスク出力 (`String()` で `(provided)` を返す helper)。

**Blocked by:** T-004

**Related specs:** [09-configuration.md §3](./09-configuration.md#3-入力解決の優先順位)

---

### T-006: preset ローダ (`@<name>`, URL, ローカル)

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:medium`

**背景:**
`config_presets` は YAML ファイルを取り込んで `q` にマージする仕組み。`@languages` のような組込み名 / URL / ローカルパス (Action 環境のみ) の 3 形式に対応。

**スコープ:**
- `internal/config/presets.go`:
  - `LoadPresets(ctx, list string, meta *MetadataLoader, fetch HTTPGetter) (map[string]any, error)`。
  - YAML スキーマは `schema: v1`, `with: {...}` ([09-configuration.md §5.2](./09-configuration.md#52-スキーマ))。
- `preset: no` の入力は除外。
- `token` 型は preset 経由で受け取らない。

**Acceptance criteria:**
- [ ] `@maximum-content` 等の組込みプリセットを `assets/presets/<name>.yml` として埋込み、ロード成功。
- [ ] URL / ローカル両方のロードに対するテスト。
- [ ] 不正スキーマで error。

**Blocked by:** T-004, T-007

**Related specs:** [09-configuration.md §5](./09-configuration.md#5-presets-ローダ)

---

### T-007: HTTP クライアントラッパ (axios 相当)

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**背景:**
すべての外部 API 呼び出しを集約する共通クライアント。retry / timeout / User-Agent / body size limit を一箇所で設定する。

**スコープ:**
- `internal/httpx/client.go`:
  - `Client struct{...}`、`Get/PostJSON/PostForm/Binary` メソッド。
  - `hashicorp/go-retryablehttp` で 4xx 非再試行 / 5xx・429・network エラーで指数バックオフ retry。
  - 既定 30 秒タイムアウト、body 50 MB 上限。
  - User-Agent: `metrics/<version> (+https://github.com/lowlighter/metrics)`。
- `imgb64(url)` ヘルパ ([04-rendering.md §11](./04-rendering.md#11-補助ユーティリティ))。

**Acceptance criteria:**
- [ ] `httptest.Server` を立て、retry / timeout / status code 別挙動をテスト。
- [ ] `imgb64` で PNG → `data:image/png;base64,...` 文字列を返す。

**Blocked by:** T-002

**Related specs:** [08-external-services.md §7](./08-external-services.md#7-共通-http-クライアント方針)

---

### T-008: GitHub REST クライアントラッパ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**背景:**
`google/go-github` をラップし、カスタム base URL / mocking 対応を入れる。

**スコープ:**
- `internal/githubapi/rest.go`:
  - `NewREST(token, customBaseURL)`。
  - `MockTransport` injection 用に `*http.Client` を差し替え可能に。
- `internal/githubapi/auth.go`: token 種別判定 (`gh[pousr]_` classic / `github_pat_` fine-grained / legacy / NOT_NEEDED / MOCKED_TOKEN)。

**Acceptance criteria:**
- [ ] `GET /rate_limit` を呼ぶテストで `RateLimitResponse` が取れる。
- [ ] mock 化したクライアントが期待 path で受信する。
- [ ] `github_pat_` トークンで明示的に「未対応」エラーを返す。

**Blocked by:** T-007

**Related specs:** [08-external-services.md §1.1](./08-external-services.md#11-rest)

---

### T-009: GitHub GraphQL クライアント (genqlient 採用)

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**背景:**
40+ プラグインで GraphQL を使う。`Khan/genqlient` で型生成し、`.graphql` ファイルから Go 関数を作る。

**スコープ:**
- `Khan/genqlient` 設定 (`genqlient.yaml`)。
- `assets/plugins/base/queries/*.graphql` を変数化 (`$login: String!` 等)。
- `internal/githubapi/graphql.go`:
  - `NewGraphQL(token, customBaseURL)` で `Doer` 実装を返す。
  - mocking 対応。
- 既存の `$login` 文字列置換クエリは genqlient 形式へ書き換えるか、暫定で文字列 replace 互換実装を提供。

**Acceptance criteria:**
- [ ] `make gen` で base の `user.graphql`, `user.x.graphql` から Go 関数が生成される。
- [ ] mock backend に対するテストパス。

**Blocked by:** T-008

**Related specs:** [08-external-services.md §1.2](./08-external-services.md#12-graphql), [08-external-services.md §2](./08-external-services.md#2-github-graphql-クエリの管理)

---

### T-010: API レート状態トラッカ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:medium`

**背景:**
`/.requests` エンドポイントや Action 起動時の quota check で利用するレート情報を一元管理する。

**スコープ:**
- `internal/githubapi/rate.go`:
  - `Resources{Rest, GraphQL, Search}` 構造体 (`Limit, Used, Remaining, Reset`)。
  - `Refresh(ctx)` で `GET /rate_limit` を取得し更新。
  - 15 分タイマー + 「リクエスト消費後」のフラグ refresh。

**Acceptance criteria:**
- [ ] テストで mocked rate-limit エンドポイントから値が反映される。
- [ ] 並行 refresh が安全 (race detector clean)。

**Blocked by:** T-008

**Related specs:** [03-web-server.md §1.2](./03-web-server.md#12-構築フロー), [02-action.md §3.3](./02-action.md#33-token-validation)

---

### T-011: in-memory cache ラッパ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:medium`

**背景:**
`memory-cache` 相当 (TTL 付き)。embed mode の `:login` キャッシュと insights のプラグイン別キャッシュで使う。

**スコープ:**
- `internal/cache/cache.go`: `patrickmn/go-cache` をラップ。
- `Put(key, value, ttl)`, `Get(key)`, `Delete(key)`, `ItemCount()`。
- TTL=0 で無期限。

**Acceptance criteria:**
- [ ] TTL テスト。
- [ ] race detector clean。

**Blocked by:** T-001

**Related specs:** [03-web-server.md §8](./03-web-server.md#8-キャッシュとペンディング管理)

---

### T-012: アセット埋め込みパイプライン (embed.FS)

**Labels:** `phase:M1` `area:infra` `kind:chore` `priority:medium`

**背景:**
plugin queries / templates / static / octicons / twemoji を `//go:embed` でバイナリに同梱する。

**スコープ:**
- `assets/plugins/*` (queries, metadata, examples) を Node 側からコピー (`make sync-assets`)。
- `assets/templates/*` 同上。
- `assets/web/statics/*` 同上。
- `assets/octicons/data.json` を `@primer/octicons` から抽出する `internal/tools/gen-octicons/main.go`。
- `assets/twemoji/index.json` を Twemoji リポジトリから抽出する `internal/tools/gen-twemoji/main.go` (固定 sha)。

**Acceptance criteria:**
- [ ] `make gen` で octicons / twemoji が再生成される。
- [ ] `embed.FS` から各 plugin の queries が読める。

**Blocked by:** T-001

**Related specs:** [10-testing-deployment.md §4.3](./10-testing-deployment.md#43-embedfs), [04-rendering.md §8](./04-rendering.md#8-絵文字アイコン置換)

---

### T-013: フォーマッタ群 (format / s / date / bytes / ellipsis)

**Labels:** `phase:M1` `area:render` `kind:feature` `priority:high` `good-first-issue`

**背景:**
全プラグイン / partial で使う数値・日付フォーマッタ。仕様は [04-rendering.md §2.3](./04-rendering.md#23-共通-helper) を参照。

**スコープ:**
- `internal/format/format.go`:
  - `Format(n, opts)`: `1.2k`, `5M`, `2b` 等の単位付きフォーマット。
  - `FormatBytes(n)`: `5.3 MB`。
  - `FormatPercentage(n, opts)`: `45.2%`、`rescale` オプション。
  - `FormatDate(t, opts)`: timezone 尊重、`15 Nov 2023` 等。
  - `Ellipsis(s, n)`: 省略。
  - `S(n, suffix)`: 単複サフィックス。
  - `FormatError(err, opts)`: エラー正規化 (`{Type, Message}`)。

**Acceptance criteria:**
- [ ] Node 版とのテーブルテスト一致。境界値 (`0`, `999`, `1000`, `999999`, `1000000`) を網羅。
- [ ] timezone 切替が正しい。

**Blocked by:** T-001

**Related specs:** [04-rendering.md §2.3](./04-rendering.md#23-共通-helper)

---

## Phase M2: コアエンジン + classic 最小

### T-014: Plugin インタフェースと Registry

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**背景:**
全プラグインが従う共通インタフェース。`init()` で `Register()` する流儀。

**スコープ:**
- `internal/plugins/plugin.go`:
  - `Plugin interface{ Name(); Metadata(); Run(ctx, *PluginContext) (any, error) }`。
  - `var registry = map[string]Plugin{}`、`Register(p)`, `Get(name)`, `Each(fn)`。
- `PluginContext` 構造体 ([01-architecture.md §4](./01-architecture.md#4-主要データ型))。

**Acceptance criteria:**
- [ ] 二重登録で panic する。
- [ ] テスト用に `RegisterForTest(p)` (登録後 cleanup) を提供。

**Blocked by:** T-002, T-004

**Related specs:** [05-plugins.md §2](./05-plugins.md#2-プラグインインタフェース), [05-plugins.md §4](./05-plugins.md#4-プラグイン登録-registry)

---

### T-015: Template インタフェースと Registry

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**背景:**
Plugin と同様に Template 側のレジストリを定義する。

**スコープ:**
- `internal/templates/template.go`:
  - `Template interface{ Name(); Metadata(); Image() []byte; Style() []byte; Fonts() []byte; Partials() []string; PartialBody(name) PartialFunc; Run(ctx, *PluginContext) error; Check(q, account, format) error }`。
  - `Register(t)` / `Get(name)`.
- `PartialFunc func(w io.Writer, ctx PartialContext) error`、`PartialContext` 構造体。

**Acceptance criteria:**
- [ ] 互換性チェック (`Check`) が format / account / repo 必須を検証。

**Blocked by:** T-014

**Related specs:** [07-templates.md §2](./07-templates.md#2-テンプレートインタフェース), [07-templates.md §10](./07-templates.md#10-テンプレート互換性チェック)

---

### T-016: engine.Compute オーケストレータ

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**背景:**
中核オーケストレータを実装する。base plugin → template.Run → 出力変換のパイプラインを組む。仕様は [01-architecture.md §5.1](./01-architecture.md#51-compute-シーケンス) 参照。

**スコープ:**
- `internal/engine/engine.go`:
  - `Compute(ctx, req ComputeRequest, deps Deps) (Result, error)`。
  - 順序: テンプレート存在検証 → Convert 決定 → partial 順マージ → Imports 構築 → base plugin → template.Run → goroutine 群完了待ち → エラー集約 → 出力変換。
- `Result{Rendered []byte, MIME string, Errors []PluginError}`.
- `die=true` での即時 fail / `die=false` で footer 出力。

**Acceptance criteria:**
- [ ] mocked GitHub クライアントで `classic` テンプレートの SVG が生成される。
- [ ] `Convert=json` で JSON が返る (T-029 完了時)。

**Blocked by:** T-014, T-015, T-017, T-021, T-023

**Related specs:** [01-architecture.md §5](./01-architecture.md#5-実行フロー), [04-rendering.md §1](./04-rendering.md#1-パイプライン概要)

---

### T-017: base plugin — user アカウント GraphQL

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**背景:**
共通データの取得 (`user.graphql` + `user.x.graphql`) を実装する。

**スコープ:**
- `internal/plugins/base/base.go`:
  - `Run` 内で `account="user"` ケースを処理。
  - `queries.base.user(login)`、続けて `queries.base.user.x(login, affiliations, calendar.from, calendar.to)` を実行。
  - 失敗時の field 単位フォールバックロジック (`packages`, `starredRepositories`, `watching`, `sponsorshipsAsSponsor/Maintainer`, `followers`, `following`, `issueComments`, `organizations`, `repositoriesContributedTo`)。
  - calendar 取得失敗時は空 weeks。
- `data.User`, `data.User.ContributionsCollection`, `data.User.Calendar` をセット。

**Acceptance criteria:**
- [ ] mock GraphQL backend での integration テスト。
- [ ] bulk failure → unit query fallback の挙動テスト。

**Blocked by:** T-009, T-014

**Related specs:** [05-plugins.md §5](./05-plugins.md#5-base-プラグインの特殊扱い)

---

### T-018: base plugin — organization アカウント

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**背景:**
組織アカウント用クエリ (`organization.graphql`, `organization.x.graphql`) の処理。

**スコープ:**
- `internal/plugins/base/organization.go`:
  - `Run` 内で `account="organization"` ケース。
  - field 単位 fallback: `packages`, `sponsorshipsAsSponsor/Maintainer`, `membersWithRole`。

**Acceptance criteria:**
- [ ] mock backend テスト。

**Blocked by:** T-017

**Related specs:** [05-plugins.md §5](./05-plugins.md#5-base-プラグインの特殊扱い)

---

### T-019: base plugin — indepth contributions

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:medium`

**背景:**
`base_indepth=true` で過去のアカウントライフタイム全体に対する `contributionsCollection.*` を年単位で集計する。

**スコープ:**
- `internal/plugins/base/indepth.go`:
  - `data.User.CreatedAt` から現在まで年 (or 1 年 windows) ごとにループ。
  - 各 window で `queries.base.contributions(login, field, range)` を実行し合算。
  - `metadata.api.github.overuse` extras フラグでガード。

**Acceptance criteria:**
- [ ] mock backend テスト (3 年分のループ実行を検証)。
- [ ] extras 無効時は skip。

**Blocked by:** T-017

**Related specs:** [05-plugins.md §5.2](./05-plugins.md#52-動作), [06-plugins-detail.md §1.1](./06-plugins-detail.md#11-base)

---

### T-020: base plugin — repositories ページング取得

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**背景:**
リポジトリ詳細を `repositories_batch` 件ずつページング取得し、`data.User.Repositories.Nodes` に格納する。

**スコープ:**
- `internal/plugins/base/repositories.go`:
  - GraphQL `queries.base.repositories(login, after, batch, affiliations)` を `pageInfo.hasNextPage` が `false` か `settings.repositories` 到達まで回す。
  - `repositories_forks=false` → `isFork: false` 条件。
  - `repositories_affiliations` → `ownerAffiliations` / `affiliations` (認証ユーザーの場合) 適用。
  - `repositories_skipped` 配列で owner/name フィルタ。
- 取得結果から `data.Computed.Commits`, `data.Computed.Repositories.{Stargazers/Forks/Releases/...}` 等の集計値を計算 ([13-appendix.md §B](./13-appendix.md#b-base-プラグインの取得アルゴリズム-擬似コード) `Postprocess` 参照)。

**Acceptance criteria:**
- [ ] mock backend で 250 件 (3 ページ) のページングを検証。
- [ ] `repositories_skipped` でフィルタ可能。

**Blocked by:** T-017

**Related specs:** [05-plugins.md §5.4](./05-plugins.md#54-リポジトリ取得)

---

### T-021: core plugin — グローバル設定注入

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**背景:**
`config.animations`, `config.display`, `config.timezone`, `config.base64`, `debug.flags` を読み取り、`data.Animated`, `data.Large`, `data.Columns`, `data.Config.Timezone` などをセットする。

**スコープ:**
- `internal/plugins/core/core.go`:
  - Inputs 解釈 (`metadata.plugins.core.Inputs.ForData`)。
  - `time.LoadLocation` でタイムゾーン解決。失敗時 `data.Config.Timezone.Error`。
  - `config.base64=false` で `imgb64` をパススルー化する flag を `Imports` に伝える。
  - `data.Computed` の初期化 (commits/sponsorships/licenses/repositories の zero value)。

**Acceptance criteria:**
- [ ] Asia/Tokyo を指定して `data.Config.Timezone.Name == "Asia/Tokyo"` が反映される。
- [ ] 不正タイムゾーンで `error` フィールドがセットされる。

**Blocked by:** T-014, T-013, T-005

**Related specs:** [05-plugins.md §6](./05-plugins.md#6-core-プラグインの役割)

---

### T-022: core plugin — プラグイン並列実行 + callback

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**背景:**
プラグインを `errgroup` で並列起動し、結果を `data.Plugins[name]` に集約する。

**スコープ:**
- `internal/plugins/core/run_plugins.go`:
  - `golang.org/x/sync/errgroup` で `q[name]=true` のものを並列実行。
  - `errgroup.SetLimit(parallel)` で並列度制御 (`settings.parallel`、未指定なら `GOMAXPROCS`)。
  - エラーは `data.Plugins[name] = err` (Node 版互換)。
  - `callbacks.Plugin(login, name, success, result)` の呼び出し。
- panic recover → error 化。

**Acceptance criteria:**
- [ ] 3 つの mock plugin で同時実行し、結果順が `q` 順に依存しないことを確認。
- [ ] 1 つだけ panic を投げる plugin で他が正常完了。

**Blocked by:** T-021

**Related specs:** [05-plugins.md §7](./05-plugins.md#7-並列実行とエラー集約)

---

### T-023: classic テンプレートスケルトン (image.svg embed + Run)

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**背景:**
classic の最小テンプレートを Go で実装する。最初は partials なしの空 SVG で OK。

**スコープ:**
- `internal/templates/classic/classic.go`:
  - `Template` 実装。
  - `//go:embed image.svg style.css fonts.css partials/_.json` でアセット同梱。
  - `Run(ctx, p)` で `plugins.core` を呼ぶだけ。
  - `Check` で account/format を判定。
- `image.svg` の `<%= %>` 部分を Go template に置換した最小スケルトン (style/fonts は埋め込み、partials ループは空)。

**Acceptance criteria:**
- [ ] `engine.Compute` で空 SVG が生成される (DOM 構造は Node 版と同一の `<svg>` ルート + `<foreignObject>` + `items-wrapper`)。

**Blocked by:** T-015, T-021

**Related specs:** [07-templates.md §3](./07-templates.md#3-共通レイアウト), [07-templates.md §5](./07-templates.md#5-classic-テンプレート)

---

### T-024〜T-028: classic partial 実装 5 件 (header / introduction / activity+community / repositories / metadata footer)

各 1 タスクずつ起票する。共通項を以下に示す:

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ (共通):**
- `internal/templates/classic/partials/<name>.go` に `PartialFunc` を実装。
- DOM 構造はオリジナル実装と同一に保つ ([13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン) と互換)。
- 動的式は `internal/render/helpers.go` のヘルパで処理。

**Acceptance criteria (共通):**
- [ ] mock data に対し goldenfile (`tests/golden/<name>.svg.frag`) と一致。
- [ ] 不在キー(`plugins.languages` 等)でも panic せず空文字列出力。

| ID | Partial | 出典 (参考、原実装) |
|----|---------|--------------------|
| T-024 | `base.header` | `source/templates/classic/partials/base.header.ejs` |
| T-025 | `introduction` | `source/templates/classic/partials/introduction.ejs` |
| T-026 | `base.activity+community` | `source/templates/classic/partials/base.activity+community.ejs` |
| T-027 | `base.repositories` | `source/templates/classic/partials/base.repositories.ejs` |
| T-028 | metadata footer (image.svg 内 `<% if (base.metadata) %>` 部) | `source/templates/classic/image.svg` 末尾 |

**Blocked by:** T-023, T-017, T-020

**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

### T-029: JSON 出力モード

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**背景:**
`config_output=json` のとき `data` を JSON にシリアライズして返す。Node 版は循環参照を `[Circular]` で潰す動作なので、Go でも同等のサイクル検出を入れる。

**スコープ:**
- `internal/engine/json.go`:
  - `Marshal(data *Data) ([]byte, error)`。
  - `cycleDetector` でループを検出し `"[Circular]"` 文字列へ。
  - `Set` 型は `[]T`、`Map` は `map[string]any` 表現。
- MIME = `application/json`。

**Acceptance criteria:**
- [ ] 既存 `tests/cases/*.yml` のうち `config_output=json` ケースで Node 版と JSON が同一キー集合。
- [ ] 循環参照混入時に panic せず `"[Circular]"` を出力。

**Blocked by:** T-016

**Related specs:** [04-rendering.md §7.1](./04-rendering.md#71-json)

---

### T-030: Insights JSON 出力モード

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:medium`

**背景:**
Insights モードの内部実装(`convert="json"`) を呼び出すヘルパ。固定 q / plugins セット ([04-rendering.md §6](./04-rendering.md#6-insights-html-出力))。

**スコープ:**
- `internal/engine/insights.go`:
  - `ComputeInsights(ctx, login, deps) (Result, error)`。
  - 固定 q (classic + achievements/isocalendar/languages/activity/notable/followup/introduction/topics/stars/reactions/repositories/sponsors/calendar)。
  - 固定 plugins enable map。

**Acceptance criteria:**
- [ ] mocked GitHub backend で完走する。
- [ ] JSON 内に必要なプラグインキーが存在。

**Blocked by:** T-029, T-022

**Related specs:** [04-rendering.md §6](./04-rendering.md#6-insights-html-出力)

---

## Phase M3: レンダリングパイプライン

### T-031: chromedp ラッパ (長寿命ブラウザ)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**背景:**
SVG resize / hash / PNG screenshot / Markdown PDF / Insights HTML すべてで使う chromedp の共通ラッパ。

**スコープ:**
- `internal/render/chrome.go`:
  - `Browser` 構造体: `allocCtx`, `parentCtx` を保持。`New(opts)` / `NewTab(ctx)` / `Close()`。
  - 起動オプション: `--no-sandbox`, `--disable-extensions`, `--disable-dev-shm-usage`, `--disable-gpu`, `--single-process` 等。
  - `METRICS_CHROME_PATH` 環境変数を尊重。
  - N 回ごとに再起動 (`SettingsBrowserRecycle`、既定 200)。
- debug flags (`--puppeteer-disable-headless`, `--puppeteer-debug`, `--puppeteer-wait-<event>`) のマッピング。

**Acceptance criteria:**
- [ ] Docker 内で `chromium` を起動できる integration テスト。
- [ ] 200 回利用後に再起動が発生。

**Blocked by:** T-002

**Related specs:** [04-rendering.md §3](./04-rendering.md#3-svg-リサイズ-chromedp), [08-external-services.md §6](./08-external-services.md#6-ブラウザスクレイピング)

---

### T-032: svg.Resize 実装

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**背景:**
SVG の最終高さを chromedp で計測し、`<svg height>` を書き換える。

**スコープ:**
- `internal/render/svg_resize.go`:
  - `Resize(ctx, rendered string, opts ResizeOptions) (string, MIME, error)`。
  - paddings (`"<absolute> + <relative>%"`) パース。
  - viewport `980x980`、`body { margin:0; padding:0; }` 追加。
  - ユーザー JS (`scripts []string`) 実行。
  - `no-animations` クラス付与 + `Sleep(2.4s)` 待機。
  - `getBoundingClientRect()` で `svg #metrics-end` を取得。
  - `convert=png/jpeg` でスクリーンショット。

**Acceptance criteria:**
- [ ] サンプル SVG (`tests/fixtures/svg/sample.svg`) のリサイズ結果が期待 height 範囲に収まる。
- [ ] PNG 出力ができる。

**Blocked by:** T-031

**Related specs:** [04-rendering.md §3](./04-rendering.md#3-svg-リサイズ-chromedp)

---

### T-033: svg.Hash (chromedp + goquery)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**背景:**
`output_condition=data-changed` の差分判定に使う MD5 計算。アルゴリズムは [13-appendix.md §H](./13-appendix.md#h-svghash-の正規化アルゴリズム)。`goquery` で実装するとブラウザ起動が不要で軽量。

**スコープ:**
- `internal/render/svg_hash.go`:
  - `Hash(rendered string) (string, error)`。
  - `goquery` で `<footer>` を除去 → `<svg>` outerHTML → MD5 hex。
- 空入力で `("", nil)` を返す。

**Acceptance criteria:**
- [ ] Node 版とのバイト一致 (footer 削除後の outer HTML が同じであれば MD5 同一)。

**Blocked by:** T-001

**Related specs:** [04-rendering.md §10](./04-rendering.md#10-svg-ハッシュ-差分判定)

---

### T-034: CSS optimizer (purge + minify)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**背景:**
`<style data-optimizable="true">…</style>` ブロックの未使用セレクタ削除と minify。

**スコープ:**
- `internal/render/css.go`:
  - `OptimizeCSS(rendered string) (string, error)`。
  - `<style data-optimizable="true">…</style>` を抽出 → HTML 本体に出現するセレクタのみ残す → minify。
  - 実装: `tdewolff/parse/v2/css` で AST、`golang.org/x/net/html` + `cascadia` で selector match、`tdewolff/minify/v2/css` で minify。

**Acceptance criteria:**
- [ ] 未使用クラスを 50% 削減できる固定ケースをテスト。
- [ ] 出現セレクタの保護リスト (`!purge`) をサポート。

**Blocked by:** T-001

**Related specs:** [04-rendering.md §4.1](./04-rendering.md#41-css-最適化-svgoptimizecss)

---

### T-035: XML 整形 (xml-formatter 相当)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:low`

**背景:**
`svg.optimize.xml` の置換。仕様: `lineSeparator="\n"`, `collapseContent=true` で整形。

**スコープ:**
- `internal/render/xml_format.go`:
  - `FormatXML(rendered string) (string, error)`。
  - `encoding/xml` 標準パーサで walk → indent。
- `raw=true` モードではパススルー。

**Acceptance criteria:**
- [ ] 入れ子要素のインデントが期待通り。

**Blocked by:** T-001

**Related specs:** [04-rendering.md §4.2](./04-rendering.md#42-xml-整形-svgoptimizexml)

---

### T-036: twemoji 置換

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**背景:**
SVG 内の絵文字を Twemoji の SVG に置換する。

**スコープ:**
- `internal/render/twemoji.go`:
  - Unicode emoji を正規表現で抽出 → `https://twemoji.maxcdn.com/v/<sha>/svg/<code>.svg` を fetch (or `assets/twemoji/` から取得)。
  - `<svg class="twemoji" ...>` に置換。
  - `<metrics …>emoji</metrics>` カスタムタグは属性付き置換。
  - in-memory cache (LRU)。

**Acceptance criteria:**
- [ ] `📊 hello 🚀` を入力 → emoji 部分が `<svg class="twemoji">` に置換。

**Blocked by:** T-007, T-012

**Related specs:** [04-rendering.md §8.1](./04-rendering.md#81-twemoji)

---

### T-037: gemoji 置換 (GitHub emoji)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:low`

**背景:**
`:smile:` 等のショート記法を `<img>` (data URI) に置換する。

**スコープ:**
- `internal/render/gemoji.go`:
  - `GET /emojis` で `name → url` map を取得。
  - 出現する `:name:` を抽出 → `imgb64(url)` で base64 化 → `<img class="gemoji" src="data:image/...;base64,...">` に置換。

**Acceptance criteria:**
- [ ] mock REST で `:octocat:` が `<img class="gemoji">` に置換される。

**Blocked by:** T-007, T-008

**Related specs:** [04-rendering.md §8.2](./04-rendering.md#82-gemoji)

---

### T-038: octicon 置換

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**背景:**
`:octicon-<name>-<size>:` プレースホルダを Primer Octicons SVG に置換する。

**スコープ:**
- `internal/render/octicon.go`:
  - `assets/octicons/data.json` を `embed` (T-012 生成済)。
  - 正規表現で抽出し SVG に置換。
  - `:octicon-<name>:` (size 省略) は 16px を採用。

**Acceptance criteria:**
- [ ] `:octicon-star-24:` が 24px SVG に。
- [ ] 未知 octicon は素通し。

**Blocked by:** T-012

**Related specs:** [04-rendering.md §8.3](./04-rendering.md#83-octicon)

---

### T-039: PNG / JPEG 出力統合

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**背景:**
T-032 の chromedp ベースで PNG / JPEG スクリーンショットを返す。

**スコープ:**
- `engine.Compute` の出力フェーズで `Convert ∈ {png, jpeg}` のときに `svg.Resize` の `convert` 引数を渡す。
- MIME は `image/png` / `image/jpeg`。

**Acceptance criteria:**
- [ ] SVG → PNG 出力が画像として有効 (`image.Decode` で成功)。

**Blocked by:** T-032

**Related specs:** [04-rendering.md §3.1](./04-rendering.md#31-アルゴリズム)

---

### T-040: PDF 出力 (markdown-pdf)

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**背景:**
markdown HTML を chromedp で PDF 化する `svg.pdf` 相当。

**スコープ:**
- `internal/render/pdf.go`:
  - `RenderPDF(ctx, html string, opts PDFOptions) ([]byte, error)`。
  - `<main class="markdown-body">…</main>` をラップ。
  - `@primer/css/dist/markdown.css` + ユーザー `style` を `addStyleTag`。
  - `paddings` を `main { margin: ... }` に。
  - `page.PrintToPDF`.

**Acceptance criteria:**
- [ ] 1 ページ PDF が生成される。

**Blocked by:** T-031, T-036, T-037, T-038

**Related specs:** [04-rendering.md §5](./04-rendering.md#5-pdf-出力)

---

## Phase M4: 主要 GitHub プラグイン

> 各プラグインは個別に issue 化。共通テンプレ:
>
> **Labels:** `phase:M4` `area:plugins` `kind:feature`
>
> **共通スコープ:**
> - `internal/plugins/<name>/<name>.go` を新規作成し、`Plugin` インタフェースを実装。
> - `init()` で `plugins.Register(&Plugin{})`。
> - GraphQL/REST 呼び出しは mocked テストでカバー。
> - 出力構造体に `json:` タグを付け、Node 版と JSON キーが一致するよう調整。
>
> **共通 Acceptance criteria:**
> - [ ] Run() が mocked dependency でエラーなく完了。
> - [ ] 期待する `data.Plugins[<name>]` 構造が出来上がる。
> - [ ] 該当する partial (T-024〜T-028 で未実装のものは別途) で SVG 表示できる。

### T-041: plugin languages (標準モード)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**
- `internal/plugins/languages/languages.go`。
- `data.User.Repositories.Nodes` の `languages.edges.{size, node.{name, color}}` を集計。
- `plugin_languages_limit`, `plugin_languages_threshold`, `plugin_languages_ignored`, `plugin_languages_skipped`, `plugin_languages_aliases`, `plugin_languages_colors`, `plugin_languages_other` を反映。

**Blocked by:** T-014, T-020

**Related specs:** [06-plugins-detail.md §2.12](./06-plugins-detail.md#212-languages)

---

### T-042: plugin languages.recent

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- `plugin_languages_sections` に `recently-used` が含まれる場合の分岐。
- REST `/users/.../events` から PushEvent を取得 → commit diff (`/repos/.../commits/{sha}`) で各ファイルの言語を判定 (go-enry)。
- `plugin_languages_recent_load` (300), `plugin_languages_recent_days` (14), `plugin_languages_recent_categories` を反映。
- `stats.recent.{stats, lines, colors}` を `data.Plugins.languages` に追加。

**Blocked by:** T-041

**Related specs:** [06-plugins-detail.md §2.12](./06-plugins-detail.md#212-languages)

---

### T-043: plugin languages.indepth (go-enry + git clone)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- `plugin_languages_indepth=true` で `data.User.Repositories.Nodes` のうち対象を `go-git` で clone → `go-enry` で言語解析。
- 一時ディレクトリ管理 (`os.MkdirTemp` / defer cleanup)。
- 個別タイムアウト `plugin_languages_analysis_timeout_repositories` (7.5 分)。
- 全体タイムアウト `plugin_languages_analysis_timeout` (15 分)。
- extras `metrics.cpu.overuse` + `metrics.run.git` でガード。

**Blocked by:** T-041

**Related specs:** [08-external-services.md §5](./08-external-services.md#5-git--linguist)

---

### T-044: plugin activity

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**
- REST `/users/{login}/events` (or `/orgs/{org}`) を `plugin_activity_load` (300) 件まで取得。
- イベントタイプフィルタ (`push/issue/pr/review/ref/release/comment/wiki/fork/star/member/public`)。
- `plugin_activity_limit` (5), `plugin_activity_days` (14), `plugin_activity_visibility` (public/all), `plugin_activity_timestamps`, `plugin_activity_skipped`, `plugin_activity_ignored`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.2](./06-plugins-detail.md#22-activity)

---

### T-045: plugin achievements

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**
- ランク計算ロジック ([06-plugins-detail.md §2.1](./06-plugins-detail.md#21-achievements) の閾値テーブル: `x>=s → S`, `x>=a → A`, …)。
- 各統計値 (commits, repositories, stars, followers, organizations, …) をランクテーブルに照合。
- `plugin_achievements_threshold` (S/A/B/C/X) でフィルタ。
- `plugin_achievements_secrets`, `plugin_achievements_display` (detailed/compact), `plugin_achievements_limit`, `plugin_achievements_only`, `plugin_achievements_ignored`。
- (option) GitHub Profile ページの chromedp scrape はあとで T-053 と類似の対応で実装。

**Blocked by:** T-017, T-020, T-031 (optional)

**Related specs:** [06-plugins-detail.md §2.1](./06-plugins-detail.md#21-achievements)

---

### T-046: plugin repositories

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- featured (`plugin_repositories_featured`) / pinned (`pinnedItems`) / starred (`user.repositories(orderBy: STARGAZERS)`) / random (上記からサンプル)。
- ソート: `plugin_repositories_order` (featured/pinned/starred/random)。
- `plugin_repositories_pinned/starred/random/affiliations/forks/skipped/batch`。

**Blocked by:** T-014, T-020

**Related specs:** [06-plugins-detail.md §2.19](./06-plugins-detail.md#219-repositories)

---

### T-047: plugin lines

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- REST `/repos/.../stats/contributors` を `data.User.Repositories.Nodes` に対して並列実行 (rate-limit 注意)。
- `weeks[].{a,d,c}` を user 単位で集計。
- `plugin_lines_sections` (`base`/`repositories`/`history`), `plugin_lines_repositories_limit` (4), `plugin_lines_history_limit` (1), `plugin_lines_skipped`。

**Blocked by:** T-014, T-020

**Related specs:** [06-plugins-detail.md §2.14](./06-plugins-detail.md#214-lines)

---

### T-048: plugin gists

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low` `good-first-issue`

**スコープ:**
- GraphQL `user.gists` cursor ループ。
- 集計 (totalCount, forks, stargazers, comments, files)。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.8](./06-plugins-detail.md#28-gists)

---

### T-049: plugin isocalendar

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- `plugin_isocalendar_duration` (`half-year`/`full-year`) に応じて `data.User.ContributionsCollection.ContributionCalendar` を取得 (base で 14 日分は取得済みなので追加クエリ)。
- 3D isometric SVG 用に week/day グリッドを返す。
- streak (Max, Current), average, sum を計算。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.11](./06-plugins-detail.md#211-isocalendar)

---

### T-050: plugin calendar

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- 年単位の `contributionsCollection.contributionCalendar` を `plugin_calendar_limit` (年数, 0=全期間) ループで取得。
- `Years: [{Year, Weeks: [...]}]` 構造で返す。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.3](./06-plugins-detail.md#23-calendar)

---

### T-051: plugin habits

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- REST `/users/.../events` から PushEvent を `plugin_habits_from` (200) 件取得。
- 各コミットの diff から: 曜日別 / 時間帯別カウント、indent (spaces vs tabs)、行平均文字数。
- linguist 統合 (`plugin_habits_languages.recent`)。
- `plugin_habits_days` (14), `plugin_habits_facts`, `plugin_habits_charts`, `plugin_habits_charts_type` (classic/graph), `plugin_habits_trim`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.9](./06-plugins-detail.md#29-habits)

---

### T-052: plugin stars (recently starred)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low` `good-first-issue`

**スコープ:**
- GraphQL `user.starredRepositories(orderBy: STARRED_AT)` を `plugin_stars_limit` (4) 件取得。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.25](./06-plugins-detail.md#225-stars)

---

### T-053: plugin topics (chromedp scrape)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- chromedp で `https://github.com/stars/<login>/topics?direction=desc&page=<n>&sort=<sort>` を巡回。
- DOM から `name`, `description`, `icon`, `url` を抽出。
- `plugin_topics_mode` (icons/labels/mastered), `plugin_topics_limit` (15), `plugin_topics_sort`。
- extras `metrics.run.puppeteer.scrapping` でガード。

**Blocked by:** T-014, T-031

**Related specs:** [06-plugins-detail.md §2.27](./06-plugins-detail.md#227-topics)

---

### T-054: plugin starlists (chromedp scrape)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- chromedp で GitHub Star Lists ページを巡回し、リスト名・リポジトリを抽出。
- `plugin_starlists_languages` (bool) で言語分析モード (languages plugin と同じロジックを再利用)。

**Blocked by:** T-053, T-041

**Related specs:** [06-plugins-detail.md §2.24](./06-plugins-detail.md#224-starlists)

---

### T-055: plugin people

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL followers / following / sponsors / contributors / stargazers / watchers / members を `plugin_people_types` ごとに取得。
- `plugin_people_limit` (24), `plugin_people_size` (28), `plugin_people_shuffle`, `plugin_people_identicons` 等。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.16](./06-plugins-detail.md#216-people)

---

### T-056: plugin notable

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL repositoryOwner, REST commits / issues。
- `plugin_notable_filter`, `plugin_notable_repositories`, `plugin_notable_indepth`, `plugin_notable_types`, `plugin_notable_from`, `plugin_notable_self`。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.15](./06-plugins-detail.md#215-notable)

---

### T-057: plugin followup

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL `search(type:ISSUE, query:'is:pr is:open author:<login>')` 等。
- repositories/user ごとに集計。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.7](./06-plugins-detail.md#27-followup)

---

### T-058: plugin discussions

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL `user.repositoryDiscussions`, `user.repositoryDiscussionComments`。
- 集計 (`started`, `answered`, `comments`, `upvotes`, `categories`)。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.6](./06-plugins-detail.md#26-discussions)

---

### T-059: plugin contributors

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- REST `/repos/.../stats/contributors`, GraphQL `repository.refs.commits` で base/head 範囲を取得。
- `plugin_contributors_base/head/contributions/sections/ignored`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.5](./06-plugins-detail.md#25-contributors)

---

### T-060: plugin code (random code snippet)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- REST events / commits からランダム diff を抽出。
- `plugin_code_lines/load/days/visibility/skipped/languages`。
- セキュリティ警告コメントを README に明記。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.4](./06-plugins-detail.md#24-code)

---

### T-061: plugin introduction

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low` `good-first-issue`

**スコープ:**
- bio (user) / description (org/repo) を Markdown → HTML。
- `plugin_introduction_title`。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.10](./06-plugins-detail.md#210-introduction)

---

### T-062: plugin reactions

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL issues/comments の reactions 集計。
- `plugin_reactions_limit*/days/details/ignored`。

**Blocked by:** T-014, T-017

**Related specs:** [06-plugins-detail.md §2.18](./06-plugins-detail.md#218-reactions)

---

### T-063: plugin projects

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL `user.projects` / `repository.projects`。
- `plugin_projects_limit/repositories/descriptions`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.17](./06-plugins-detail.md#217-projects)

---

### T-064: plugin sponsors

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL `user.sponsorsListing`, `sponsorsForViewer`。
- `plugin_sponsors_sections/size/title/past`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.21](./06-plugins-detail.md#221-sponsors)

---

### T-065: plugin sponsorships

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- GraphQL `viewer.sponsorshipsAsSponsor(activeOnly:false)`。
- `plugin_sponsorships_sections`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §2.22](./06-plugins-detail.md#222-sponsorships)

---

### T-066: plugin stargazers (with worldmap)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- GraphQL `repository.stargazers(orderBy:STARRED_AT)`。
- (option) Google Maps Geocoding で worldmap (location → 緯度経度) 計算。
- `plugin_stargazers_charts/charts_type/worldmap/worldmap_token/worldmap_sample`。

**Blocked by:** T-014, T-007

**Related specs:** [06-plugins-detail.md §2.23](./06-plugins-detail.md#223-stargazers)

---

### T-067: plugin skyline (3D city)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `https://skyline.github.com/<login>/<year>.json` のメッシュ取得。
- 3D ビューを SVG/frame に変換 (chromedp で skyline.github.com を開き frame をスクショ → アニメーション GIF)。
- `plugin_skyline_year/frames/quality/compatibility/settings`。

**Blocked by:** T-031

**Related specs:** [06-plugins-detail.md §2.20](./06-plugins-detail.md#220-skyline)

---

### T-068: plugin traffic

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- REST `/repos/.../traffic/views` を repositories に対して並列。
- token に `repo` scope が無ければ warning。

**Blocked by:** T-014, T-020

**Related specs:** [06-plugins-detail.md §2.28](./06-plugins-detail.md#228-traffic)

---

### T-069: plugin support (deprecated)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- chromedp で `github.community` プロファイルをスクレイプ (上流終了済のため、historical な値を最小限取る or 動作不可フォールバック)。
- `kind:deprecated` ラベル付き。

**Blocked by:** T-031

**Related specs:** [06-plugins-detail.md §2.26](./06-plugins-detail.md#226-support-deprecated)

---

## Phase M5: Web インスタンス

### T-094: server skeleton (chi ルーティング)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:high`

**スコープ:**
- `internal/server/server.go`: `go-chi/chi/v5` で router 生成。
- `New(opts) (*Server, error)` で `Settings`, `Plugins`, `Templates`, GitHub client, cache を組み立て。
- `ListenAndServe()`, `Shutdown(ctx)`。

**Acceptance criteria:**
- [ ] `GET /.version` で `package.version` が返る。

**Blocked by:** T-003, T-008, T-011

**Related specs:** [03-web-server.md §1](./03-web-server.md#1-起動とライフサイクル)

---

### T-095: middleware (compression / rate-limit / cache header / RealIP)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:high`

**スコープ:**
- compression: `middleware.Compress(5)`。
- rate-limit: `settings.ratelimiter` (`max=0` で無効) を `httprate.LimitByIP` に変換。skip 条件: cache hit ユーザー。
- cache header: `?cache=<ms>` か `settings.cached` で `Cache-Control` 設定。
- `middleware.RealIP` で X-Forwarded-For。
- グローバル limiter (60 req/min) を `/.*` 静的・メタに適用。

**Acceptance criteria:**
- [ ] テストで 429 が返る境界を確認。

**Blocked by:** T-094

**Related specs:** [03-web-server.md §3](./03-web-server.md#3-ミドルウェア)

---

### T-096: 静的アセット配信 (embed.FS)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:medium`

**スコープ:**
- `assets/web/statics/` 全体を `//go:embed` で取り込み、`http.FS` 経由で配信。
- `/.css/*`, `/.js/*`, `/`, `/index.html`, `/favicon.ico`, `/.opengraph.png` をマップ。
- `node_modules` 由来 (vue, prismjs, axios, clipboard, faker) は `assets/web/vendor/` に格納。

**Acceptance criteria:**
- [ ] `GET /` で `index.html` が返る。
- [ ] `GET /.js/app.js` が返る。

**Blocked by:** T-094, T-012

**Related specs:** [03-web-server.md §10](./03-web-server.md#10-静的アセットの配信)

---

### T-097: メタエンドポイント (.plugins / .templates / .modes / .extras / .requests 等)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:medium`

**スコープ:**
- `/.plugins`, `/.plugins.base`, `/.plugins.metadata`。
- `/.templates`, `/.templates/{template}`, `/.templates/{template}/partials/*`。
- `/.modes`, `/.extras`, `/.extras.logged`。
- `/.version`, `/.hosted`, `/.requests`, `/.uncache`。
- `/.opengraph.png` (`settings.web.opengraph` で 302、なければ static)。

**Acceptance criteria:**
- [ ] Node 版と JSON キー一致。

**Blocked by:** T-094, T-097 系列の各 GET を含む

**Related specs:** [03-web-server.md §2](./03-web-server.md#2-ルーティング一覧)

---

### T-098: embed エンドポイント (`GET /:login/:repository?`)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:high`

**スコープ:**
- login validation (`^[-\w]+$`)、`.` 始まり / `/` 含むものは next() 相当でスキップ。
- restricted リストチェック → 403。
- cache hit → そのまま返却。
- pending dedup (同一 login)。
- maxusers 超 → 503。
- repository alias (`q.template=repository`, `q.repo=repository`)。
- session header → user octokit に切替。
- `q["config.presets"]` を `LoadPresets` で展開。
- `engine.Compute` 呼び出し → cache.Put。
- 各エラー → 適切な HTTP code。

**Acceptance criteria:**
- [ ] mock GitHub backend で `GET /octocat` が SVG を返す。
- [ ] エラーマトリクスのテスト ([03-web-server.md §9](./03-web-server.md#9-エラーレスポンス規約))。

**Blocked by:** T-016, T-094, T-011, T-006

**Related specs:** [03-web-server.md §4](./03-web-server.md#4-embed-モード)

---

### T-099: insights エンドポイント (`GET /insights/...`)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:medium`

**スコープ:**
- 静的: `/insights/`, `/insights/.statics/*`, `/insights/{login}`。
- `/insights/query/{login}/` で 202 + 非同期 ComputeInsights。
- `/insights/query/{login}/{plugin}/` で plugin 結果取得 (キャッシュヒットのみ 200、無ければ 204)。
- pending dedup + callback で plugin 別キャッシュ書き込み。
- `/about/*` → `/insights/*` 301 redirect。

**Acceptance criteria:**
- [ ] mock backend での 202 → 後続 GET で plugin 結果が引ける。

**Blocked by:** T-030, T-094

**Related specs:** [03-web-server.md §5](./03-web-server.md#5-insights-モード)

---

### T-100: OAuth フロー

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:medium`

**スコープ:**
- `/.oauth/` 静的、`/.oauth/authenticate`, `/.oauth/authorize`, `/.oauth/revoke/:session`, `/.oauth/redirect`, `/.oauth/enabled`。
- state / session を `crypto/rand` で発行。
- `POST https://github.com/login/oauth/access_token`、`GET https://api.github.com/user` でログイン取得。
- `DELETE /applications/:id/grant` で revoke。
- session → octokit (user PAT) に切替するヘルパ `userOctokit(session)`。

**Acceptance criteria:**
- [ ] httptest backed test で end-to-end フローがパス。
- [ ] CSRF (state 不一致) で 400。

**Blocked by:** T-094, T-007

**Related specs:** [03-web-server.md §6](./03-web-server.md#6-oauth)

---

### T-101: コントロールエンドポイント (`POST /.control/stop`)

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:low`

**スコープ:**
- `settings.control.token` 有効時のみ。
- `Authorization: <token>` 一致で 202 → 5 秒後に `server.Shutdown` → `os.Exit(0)`。

**Acceptance criteria:**
- [ ] 401 / 202 が期待通り。

**Blocked by:** T-094

**Related specs:** [03-web-server.md §7](./03-web-server.md#7-コントロールエンドポイント)

---

### T-102: graceful shutdown + pending dedup

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:medium`

**スコープ:**
- SIGINT/SIGTERM → `server.Shutdown(ctx)` (最大 30 秒)。
- `pending sync.Map` (login → chan struct{}) を併設し、重複リクエストは前の完了を待つ。`debug` / `mock` モードでは無効。

**Acceptance criteria:**
- [ ] 同時 5 リクエストで実 Compute は 1 回しか走らないテスト。

**Blocked by:** T-094

**Related specs:** [03-web-server.md §1.4](./03-web-server.md#14-graceful-shutdown), [03-web-server.md §8.1](./03-web-server.md#81-同時実行制御)

---

### T-103: requests レート自動更新

**Labels:** `phase:M5` `area:server` `kind:feature` `priority:low`

**スコープ:**
- 15 分間隔の定期 refresh + リクエスト完了後の `_requests_refresh=true` フラグ → 15 秒間隔でフラグを見て refresh。

**Blocked by:** T-010, T-094

**Related specs:** [03-web-server.md §1.2](./03-web-server.md#12-構築フロー)

---

## Phase M6: GitHub Action / CLI

### T-105: action entrypoint (skip 判定 + setup)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- `cmd/metrics-action/main.go`:
  - `[Skip GitHub Action]` / `Auto-generated metrics for run #N` commit message でスキップ。
  - `setup()` 呼び出し、Docker 環境補完 (`INPUT_OUTPUT_ACTION`, `INPUT_COMMITTER_TOKEN`, `GITHUB_REPOSITORY`)。
  - 起動バナー出力 ([13-appendix.md §E](./13-appendix.md#e-action-起動バナーの整形ルール-info-互換) 参照)。

**Acceptance criteria:**
- [ ] スキップケース 2 種で exit 0。
- [ ] バナーに version 表示。

**Blocked by:** T-003, T-004

**Related specs:** [02-action.md §3](./02-action.md#3-実行フェーズ)

---

### T-106: INPUT_* パーサ + presets 統合

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- `INPUTS` (`toJson(inputs)`) JSON を最優先、なければ `INPUT_<UPPER>` を読む。
- `config_presets` を `LoadPresets` で読み込み、`q` に展開。
- `metadata.plugins.core.Inputs.ForAction(env, preset)` で `config_*` を抽出。
- `_filename` のワイルドカード `*` を `convert` に応じた拡張子に置換。

**Acceptance criteria:**
- [ ] テスト用 inputs.yaml をロードし、`q` map に正しいキーで展開される。

**Blocked by:** T-005, T-006, T-105

**Related specs:** [02-action.md §2.2](./02-action.md#22-値の型変換), [09-configuration.md §3](./09-configuration.md#3-入力解決の優先順位)

---

### T-108: token validation + rate quota check

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- token 未指定 / `github_pat_` 拒否。
- `GET /rate_limit` で残量チェック (`quota_required_rest/graphql/search` 比較)。
- `HEAD /` で `X-OAuth-Scopes` 確認。
- `notice_releases=true` のとき新バージョン通知。

**Acceptance criteria:**
- [ ] token 不足で exit 1。
- [ ] quota 不足で skipped 終了。

**Blocked by:** T-008, T-010, T-105

**Related specs:** [02-action.md §3.3](./02-action.md#33-token-validation), [02-action.md §3.4](./02-action.md#34-新バージョン通知)

---

### T-109: render + retry 統合

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- `internal/action/retry.go`:
  - `Retry(ctx, fn, retries, delay)` の共通実装。
- `engine.Compute` を `retries`/`retries_delay` で繰り返し。
- 完了後 `/renders/<filename>` に書き出し (mkdir -p)。
- `dryrun=true` ならファイル書き出しのみで output_action はスキップ。

**Acceptance criteria:**
- [ ] mocked Compute が 2 回失敗 → 3 回目成功で正常終了。

**Blocked by:** T-016, T-106

**Related specs:** [02-action.md §3.9](./02-action.md#39-render), [02-action.md §5](./02-action.md#5-リトライとレート制御)

---

### T-110: committer — commit モード

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- committer 構造体組み立て ([02-action.md §4.1](./02-action.md#41-committer-構造体))。
- head ブランチが無ければ作成 (`PUT /git/refs`)。
- 前回 oid を GraphQL から取得。
- `PUT /repos/.../contents/<filename>` で commit。

**Acceptance criteria:**
- [ ] mock GitHub に対し commit が走る。

**Blocked by:** T-109, T-008, T-009

**Related specs:** [02-action.md §4](./02-action.md#4-committer-commit--pr--gist-ロジック)

---

### T-111: committer — pull-request (+ merge / squash / rebase)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**
- `pull-request*` モードで `metrics-run-${runId}` head ブランチを作成。
- `POST /repos/.../pulls` で PR 作成。
- `pull-request-{merge|squash|rebase}` モードで `PUT /repos/.../pulls/{n}/merge`。
- 失敗は warning ログで継続。

**Acceptance criteria:**
- [ ] PR 作成 → mergeable=true → 自動 merge ケースをテスト。

**Blocked by:** T-110

**Related specs:** [02-action.md §4.4](./02-action.md#44-pr-作成--マージ)

---

### T-112: committer — Gist 出力

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:medium`

**スコープ:**
- `output_action=gist` 時に `PATCH /gists/{id}` で更新。
- `png`/`jpeg`/`markdown-pdf` 出力は拒否。

**Acceptance criteria:**
- [ ] mock テストパス。

**Blocked by:** T-109

**Related specs:** [02-action.md §4.5](./02-action.md#45-gist-出力)

---

### T-113: markdown キャッシュ committer

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:medium`

**スコープ:**
- 生成 HTML 中の `<img class="metrics-cacheable" data-name=... src="data:image/...;base64,...">` を抽出。
- `markdown_cache/<name>.<format>` にコミットし URL を書き換え。
- `retries_output_action`/`retries_delay_output_action` で再試行。

**Acceptance criteria:**
- [ ] mock テストで 2 枚の画像が cache パスにコミットされ、HTML が書き換わる。

**Blocked by:** T-091, T-110

**Related specs:** [02-action.md §4.3](./02-action.md#43-markdown-キャッシュ)

---

### T-114: output_condition=data-changed 判定

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:medium`

**スコープ:**
- 既存ファイル(`GET /repos/.../contents/<filename>`) を取得し、`svg.Hash` で比較。
- 一致したら `committer.Commit=false` に設定。

**Acceptance criteria:**
- [ ] 一致時に commit がスキップされる。

**Blocked by:** T-033, T-110

**Related specs:** [02-action.md §3.10](./02-action.md#310-output-condition)

---

### T-115: workflow cleanup (`clean_workflows`)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:low`

**スコープ:**
- `GET /repos/.../actions/runs` をページングし、metrics が生成した古い run を削除。
- `retention_days` パラメータも考慮。

**Acceptance criteria:**
- [ ] mock テストで対象 run が削除される。

**Blocked by:** T-008

**Related specs:** [02-action.md §3.12](./02-action.md#312-workflow-cleanup)

---

### T-116: insights webserver bootstrap (`output=insights`)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:low`

**スコープ:**
- `output=insights` のとき、`metrics-server` サブプロセスをバックグラウンド起動。
- `net.Dial("localhost:port")` で `Server ready` を 5 分待機。
- chromedp で `/insights/<login>?embed=1&localstorage=1` を取得 → HTML を保存。

**Acceptance criteria:**
- [ ] `output=insights` で HTML ファイルが `/renders/` に書き出される。

**Blocked by:** T-094, T-099, T-031

**Related specs:** [04-rendering.md §6](./04-rendering.md#6-insights-html-出力)

---

### T-117: CLI 専用フラグ (`metrics-action --config inputs.yaml`)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:low`

**スコープ:**
- `cobra` または `flag` で CLI フラグを実装:
  - `--config <path>`, `--user`, `--template`, `--token`, `--plugin key=val`, `--output`, `--filename`, `--dryrun`, `--token-env KEY=ENV`。
- YAML を `INPUTS` 相当に整形してから既存パイプラインへ流す。

**Acceptance criteria:**
- [ ] `metrics-action --user octocat --template classic --output svg --dryrun` で SVG が標準出力に出る。

**Blocked by:** T-105, T-106

**Related specs:** [02-action.md §7](./02-action.md#7-cli-モード-actionyml-に依存しない直接利用)

---

## Phase M7: Markdown / 派生テンプレート

### T-089: repository テンプレート

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:medium`

**スコープ:**
- `internal/templates/repository/repository.go`。
- partials: `base.repository`, `introduction`, `base.community`, `base.activity` (repository 版), 他 plugins。
- `Check` で `account != "repository"` を 406。
- `image.svg` のスケルトンは [13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン) の DOM 構造を維持して実装。

**Acceptance criteria:**
- [ ] mock data で SVG が生成される。

**Blocked by:** T-015, T-021

**Related specs:** [07-templates.md §6](./07-templates.md#6-repository-テンプレート)

---

### T-090: terminal テンプレート

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:low`

**スコープ:**
- `internal/templates/terminal/terminal.go`。
- SSH セッション風の `image.svg` + style.css + fonts.css。
- partials は classic を流用可。

**Acceptance criteria:**
- [ ] SVG 生成成功。

**Blocked by:** T-015, T-021

**Related specs:** [07-templates.md §7](./07-templates.md#7-terminal-テンプレート)

---

### T-091: markdown テンプレート (alias + embed())

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:high`

**スコープ:**
- `internal/templates/markdown/markdown.go`。
- alias data の準備 (NAME / LOGIN / COMMITS / ... の全 32 個)。
- `q.markdown` URL/path をロード (`raw.githubusercontent.com` に展開)。
- 2 パスの EJS 評価 (`<%`/`{`)。
- `embed(name, q)` helper: `engine.Compute` を再帰呼び出しし base64 化。
- markdown HTML 出力 (`MIME=text/html`)。

**Acceptance criteria:**
- [ ] `tests/fixtures/markdown/example.md` を入力で期待 HTML を生成。

**Blocked by:** T-015, T-016, T-021

**Related specs:** [07-templates.md §8](./07-templates.md#8-markdown--markdown-pdf-テンプレート)

---

### T-092: markdown-pdf パイプライン

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:medium`

**スコープ:**
- markdown 出力結果を T-040 の PDF レンダラに渡す。
- `paddings` / `style` / `twemojis` / `gemojis` / `octicons` を適用。
- `application/pdf` MIME。

**Acceptance criteria:**
- [ ] PDF バイト列が `%PDF-` で始まる。

**Blocked by:** T-091, T-040

**Related specs:** [04-rendering.md §5](./04-rendering.md#5-pdf-出力)

---

### T-093: community テンプレート loader (動的取得は無効化)

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:low`

**スコープ:**
- `settings.community.templates` の文字列パース (`repo@branch:name[+trust]`)。
- 初版では **動的 git clone は非対応**。warning ログを出して skip。
- インタフェースは整え、後継 (`v2`) で実装する。

**Acceptance criteria:**
- [ ] `community.templates` が指定されていても起動できる。

**Blocked by:** T-015

**Related specs:** [07-templates.md §9](./07-templates.md#9-community-テンプレート)

---

## Phase M8: ソーシャル / 外部 API プラグイン

### T-070: plugin wakatime

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**
- `https://wakatime.com/api/v1/users/<user>/stats/<range>?api_key=<token>`。
- self-hosted (`plugin_wakatime_url`) 対応。
- `plugin_wakatime_sections` (time/projects/projects-graphs/languages/languages-graphs/editors/os), `plugin_wakatime_days` (7), `plugin_wakatime_limit` (4), `plugin_wakatime_repositories_visibility`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.10](./06-plugins-detail.md#310-wakatime)

---

### T-071: plugin pagespeed

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `https://www.googleapis.com/pagespeedonline/v5/runPagespeed`。
- `plugin_pagespeed_token/url/detailed/screenshot/pwa`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.4](./06-plugins-detail.md#34-pagespeed)

---

### T-072: plugin posts (dev.to / hashnode / medium)

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `plugin_posts_source` 切替。
- `plugin_posts_user/limit/descriptions/covers`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.5](./06-plugins-detail.md#35-posts)

---

### T-073: plugin rss

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low` `good-first-issue`

**スコープ:**
- `mmcdole/gofeed` で RSS/Atom パース。
- `plugin_rss_source/limit`。

**Blocked by:** T-014

**Related specs:** [06-plugins-detail.md §3.6](./06-plugins-detail.md#36-rss)

---

### T-074: plugin stackoverflow

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `https://api.stackexchange.com/2.3/users/<id>`。
- `plugin_stackoverflow_user/sections/limit/lines_snippet`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.7](./06-plugins-detail.md#37-stackoverflow)

---

### T-075: plugin leetcode

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `https://leetcode.com/graphql` の `userPublicProfile`, `userSessionProgress` 等。
- `plugin_leetcode_user/sections/limit_skills/ignored_skills/limit_recent`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.2](./06-plugins-detail.md#32-leetcode)

---

### T-076: plugin anilist

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- `https://graphql.anilist.co`。
- `plugin_anilist_user/medias/sections/limit/limit_characters/limit_reviews/shuffle`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.1](./06-plugins-detail.md#31-anilist)

---

### T-077: plugin music (Spotify / LastFM / Apple Music / YouTube)

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- provider 切替 (`spotify`/`apple`/`lastfm`/`youtube`)。
- Spotify は OAuth refresh token、Apple Music は JWT、LastFM は API key。
- `plugin_music_provider/token/mode/playlist/limit/user/time_range`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.3](./06-plugins-detail.md#33-music)

---

### T-078: plugin steam

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- Steam Web API (`/ISteamUser/GetPlayerSummaries/v2/`, `/IPlayerService/GetRecentlyPlayedGames/v1/` 等)。
- `plugin_steam_token/user/sections/recent_games_limit/most_played_games_limit/playtime_threshold/achievements_limit`。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.8](./06-plugins-detail.md#38-steam)

---

### T-079: plugin tweets (deprecated)

**Labels:** `phase:M8` `area:plugins` `kind:feature` `priority:low`

**スコープ:**
- Twitter API v2 (`/2/users/by/username/<user>`, `/2/users/<id>/tweets`)。
- Bearer token (有料化済み)。
- `plugin_tweets_token/user/attachments/limit`。
- README で deprecation 明記。

**Blocked by:** T-007, T-014

**Related specs:** [06-plugins-detail.md §3.9](./06-plugins-detail.md#39-tweets-deprecated)

---

## Phase M9: community プラグイン + テスト整備

### T-080〜T-088: community プラグイン 9 件

各 1 タスクずつ起票。共通テンプレ:

**Labels:** `phase:M9` `area:plugins` `kind:feature` `priority:low`

**スコープ (共通):**
- `internal/plugins/community/<name>/<name>.go`。
- 外部 API or chromedp scrape。
- extras フラグでガード。

| ID | name | データソース |
|----|------|-------------|
| T-080 | crypto | CoinGecko `/coins/markets` |
| T-081 | nightscout | 任意 Nightscout サーバ |
| T-082 | stock | Alpha Vantage `/query?function=GLOBAL_QUOTE` |
| T-083 | chess | chess.com `/pub/player/<user>/stats` |
| T-084 | splatoon | splatoon3.ink 等 |
| T-085 | fortune | 内蔵 fortune コーパス |
| T-086 | poopmap | poopmap.com API |
| T-087 | screenshot | chromedp で任意 URL 撮影 |
| T-088 | 16personalities | 16personalities.com 結果ページ scrape |

**Blocked by:** T-031 (chromedp 系), T-007 (HTTP 系), T-014

**Related specs:** [06-plugins-detail.md §4](./06-plugins-detail.md#4-community-プラグイン)

---

### T-118: testutil/mocks — REST モック

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**
- `internal/testutil/mocks/rest.go`:
  - `MockTransport` (`http.RoundTripper` 実装)。
  - `routes map[string]func(*http.Request)*http.Response` または fixture file ロード。
  - `tests/fixtures/github/rest/<endpoint>.json` から自動応答。

**Acceptance criteria:**
- [ ] `mock.GET("/rate_limit", "rate_limit.json")` でレスポンスを差し替えできる。

**Blocked by:** T-008

**Related specs:** [10-testing-deployment.md §2](./10-testing-deployment.md#2-mocks-の設計)

---

### T-119: testutil/mocks — GraphQL モック

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**
- `internal/testutil/mocks/graphql.go`:
  - genqlient `Doer` 実装。
  - クエリ名 (`operationName`) で fixture を選択。
- `tests/fixtures/github/graphql/<name>.json` を自動ロード。

**Acceptance criteria:**
- [ ] base.user / base.user.x を mock 経由で取得テスト成功。

**Blocked by:** T-009

**Related specs:** [10-testing-deployment.md §2.1](./10-testing-deployment.md#21-mocked-api)

---

### T-120: golden file テストフレームワーク

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**
- `internal/testutil/golden`:
  - SVG / JSON 結果を正規化して `tests/golden/<case>.{svg,json}` と比較。
  - SVG は footer の動的部分 (timestamps, version) を mask して MD5 比較。
- `-update` フラグで golden を更新するヘルパ。

**Acceptance criteria:**
- [ ] `make test-golden` が緑になる初期 case (`classic_octocat`) を 1 つ追加。

**Blocked by:** T-118, T-119

**Related specs:** [10-testing-deployment.md §1.1](./10-testing-deployment.md#11-階層)

---

### T-121: e2e — classic SVG ラウンドトリップ

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**
- `tests/integration/compute_test.go`:
  - mock backend + classic template + 主要プラグイン 10 個有効化で `engine.Compute` を実行 → golden と比較。

**Blocked by:** T-016, T-041〜T-052, T-120

**Related specs:** [10-testing-deployment.md §1.2](./10-testing-deployment.md#12-テスト構成)

---

### T-122: e2e — action dryrun

**Labels:** `phase:M9` `area:test` `kind:test` `priority:medium`

**スコープ:**
- `tests/integration/action_test.go`:
  - `metrics-action --dryrun` を子プロセスで起動 (`os/exec`)、INPUTS 環境変数注入。
  - `/renders/<filename>` が期待通り生成される。

**Blocked by:** T-105〜T-117, T-118

**Related specs:** [10-testing-deployment.md §1.1](./10-testing-deployment.md#11-階層)

---

### T-123: e2e — web `/:login`

**Labels:** `phase:M9` `area:test` `kind:test` `priority:medium`

**スコープ:**
- `tests/integration/server_test.go`:
  - `httptest.NewServer` で `metrics-server` を起動。
  - `GET /octocat?config_output=json` で JSON が返り、期待キー集合と一致。
  - エラーマトリクスのテスト (400/403/404/406/429/503)。

**Blocked by:** T-094〜T-103, T-118

**Related specs:** [03-web-server.md §9](./03-web-server.md#9-エラーレスポンス規約)

---

### T-124: bench — Compute Classic

**Labels:** `phase:M9` `area:test` `kind:test` `priority:low`

**スコープ:**
- `tests/bench/compute_bench_test.go`:
  - `BenchmarkComputeClassic` (mock backend、chromedp なしモード)。
  - 目標: < 5 秒 / 1 ループ。

**Blocked by:** T-121

**Related specs:** [10-testing-deployment.md §3](./10-testing-deployment.md#3-ベンチマークとパフォーマンス目標)

---

### T-125: linter / govulncheck の CI 統合

**Labels:** `phase:M9` `area:infra` `kind:chore` `priority:medium`

**スコープ:**
- `.github/workflows/go-ci.yml` に `golangci-lint run --timeout=10m` と `govulncheck ./...` を追加。
- staticcheck, gosec, revive, gofumpt をリンタ集合に。

**Blocked by:** T-001

**Related specs:** [10-testing-deployment.md §7.2](./10-testing-deployment.md#72-追加ジョブ)

---

## Phase M10: リリース / DevOps / ドキュメント

### T-126: Dockerfile (multi-stage, multi-arch, Chrome 同梱)

**Labels:** `phase:M10` `area:infra` `kind:chore` `priority:high`

**スコープ:**
- `deploy/Dockerfile` を新規 (Node 版とは別)。
- multi-stage: `golang:1.23-bookworm` → `debian:bookworm-slim` + chromium + fonts。
- multi-arch (linux/amd64, linux/arm64) を `docker/buildx` でビルド。
- `METRICS_CHROME_PATH=/usr/bin/chromium` 環境変数。

**Acceptance criteria:**
- [ ] `docker run` で `metrics-action --help` が実行できる。
- [ ] イメージサイズ < 600 MB。

**Blocked by:** T-117

**Related specs:** [10-testing-deployment.md §5](./10-testing-deployment.md#5-docker)

---

### T-127: CLI 専用 (Chrome なし) イメージ

**Labels:** `phase:M10` `area:infra` `kind:chore` `priority:medium`

**スコープ:**
- `deploy/Dockerfile.cli` (`gcr.io/distroless/cc-debian12`)。
- chromedp 必須プラグイン (`topics`, `starlists`, `achievements`, `support`, render PDF/PNG/Insights) を起動時に warning。
- イメージサイズ < 50 MB 目標。

**Acceptance criteria:**
- [ ] 軽量イメージで `--output svg` が動く (chromedp 利用箇所はスキップ可能設計に)。

**Blocked by:** T-117

**Related specs:** [10-testing-deployment.md §5.2](./10-testing-deployment.md#52-軽量イメージ-cli-専用)

---

### T-128: action.yml の Go バイナリ呼び出しへの切替

**Labels:** `phase:M10` `area:action` `kind:chore` `priority:high`

**スコープ:**
- 既存 `action.yml` の `runs.using: composite` の中身を、Go バイナリを直接呼ぶように更新。
- 後方互換: Docker 経由のフローも残す。
- リリース後に `lowlighter/metrics@v4` で参照可能にする。

**Acceptance criteria:**
- [ ] サンプルワークフローで成功。

**Blocked by:** T-126

**Related specs:** [02-action.md §1](./02-action.md#1-概要)

---

### T-129: action.yml 自動生成ツール (`metrics-action gen action-yml`)

**Labels:** `phase:M10` `area:action` `kind:chore` `priority:low`

**スコープ:**
- `cmd/metrics-action/gen_action_yml.go`:
  - metadata.yml + core inputs から `action.yml` の `inputs:` セクションを生成。
  - 既存 Node 実装の `.github/scripts/build.mjs` 相当 (出典)。

**Acceptance criteria:**
- [ ] 既存 `action.yml` と diff が出ない (initially) ようジェネレートできる。

**Blocked by:** T-004

**Related specs:** [09-configuration.md §2.3](./09-configuration.md#23-自動生成)

---

### T-130: リリーススクリプト + GitHub Release publishing

**Labels:** `phase:M10` `area:infra` `kind:chore` `priority:high`

**スコープ:**
- `scripts/release.sh`: クロスコンパイル (linux/amd64, linux/arm64, darwin/arm64, windows/amd64)、checksum 計算、署名。
- `.github/workflows/release.yml`: タグ `v*` で起動し、Docker push + GitHub Release 作成 + Action ブランチ alias 更新。

**Acceptance criteria:**
- [ ] dry-run で release-notes が生成される。

**Blocked by:** T-126, T-127

**Related specs:** [10-testing-deployment.md §6](./10-testing-deployment.md#6-リリース)

---

### T-131: マイグレーションガイド (`docs/migration-to-go.md`)

**Labels:** `phase:M10` `area:docs` `kind:docs` `priority:high`

**スコープ:**
- Node 版から Go 版への移行手順。
- 既知の差分 (SVGO 無効、community templates 未対応、フォントレンダリングなど)。
- ロールバック方法 (`ghcr.io/lowlighter/metrics:legacy-vX.Y`)。

**Acceptance criteria:**
- [ ] 主要ユーザー (オーナー) のレビュー OK。

**Blocked by:** T-128

**Related specs:** [11-go-migration.md §4.4](./11-go-migration.md#44-マイグレーションガイド-公開)

---

### T-132: JSON Schema 公開 (settings / action / insights)

**Labels:** `phase:M10` `area:docs` `kind:docs` `priority:low`

**スコープ:**
- `api/settings.schema.json`, `api/action.schema.json`, `api/insights.schema.json` を生成・配信。
- `metrics-server` の `/.schema/<name>` でも配信。

**Acceptance criteria:**
- [ ] schema が JSON Schema Draft-07 を満たす。

**Blocked by:** T-004

**Related specs:** [09-configuration.md §7](./09-configuration.md#7-json-schema-提供)

---

## 進捗管理

進捗は GitHub Projects (Beta) で以下のカラムで可視化することを推奨:

- **Backlog** … 未着手
- **Ready** … blocker 解消、claim 待ち
- **In Progress** … 担当者 assign 済、PR 進行中
- **Review** … PR review 中
- **Done** … merge 済 / closed

各 issue には以下を必ず記入する:

- [ ] Definition of Done (Acceptance criteria) を満たした
- [ ] テストが追加され緑
- [ ] 関連仕様書 (`specs/*.md`) を更新した (実装で発見した差異)
- [ ] `Closes T-XXX` でリンクされた
