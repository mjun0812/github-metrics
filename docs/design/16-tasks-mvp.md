# 16. MVP タスクリスト

[15-selection-answer.md](./15-selection-answer.md) の選定結果に基づく **採用済 77 タスク** を、実装順 (M1→M10) で再整理した issue 化用リストです。

各タスクはそのまま GitHub issue 本文に貼れる粒度を維持しています。
詳細な背景は [12-tasks.md](./12-tasks.md) (元の全 132 タスク版) を参照してください — 本ドキュメントは MVP 用に **採用タスクのみ抽出** し、依存関係を整理したものです。

## 目次

- [1. MVP スコープ要約](#1-mvp-スコープ要約)
- [2. 進行カレンダー (目安)](#2-進行カレンダー-目安)
- [3. クリティカルパス](#3-クリティカルパス)
- [Phase M1: 基盤 (12 タスク)](#phase-m1-基盤-12-タスク)
- [Phase M2: コアエンジン + classic (16 タスク)](#phase-m2-コアエンジン--classic-16-タスク)
- [Phase M3: レンダリングパイプライン (9 タスク)](#phase-m3-レンダリングパイプライン-9-タスク)
- [Phase M4: GitHub プラグイン (21 タスク)](#phase-m4-github-プラグイン-21-タスク)
- [Phase M6: GitHub Action / CLI (8 タスク)](#phase-m6-github-action--cli-8-タスク)
- [Phase M7: repository テンプレート (1 タスク)](#phase-m7-repository-テンプレート-1-タスク)
- [Phase M9: テスト基盤 (6 タスク)](#phase-m9-テスト基盤-6-タスク)
- [Phase M10: リリース / Docker (4 タスク)](#phase-m10-リリース--docker-4-タスク)
- [MVP 追加タスク (N-001〜N-002)](#mvp-追加タスク-n-001n-002)
- [不採用タスク (backlog)](#不採用タスク-backlog)

---

## 1. MVP スコープ要約

| 区分                 | 採用                                                                                                                                                                                                                        | 内訳                                                                      |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| 起動形態             | Action + CLI                                                                                                                                                                                                                | Web 不採用                                                                |
| 出力形式             | SVG / PNG / JPEG / JSON                                                                                                                                                                                                     | Markdown 系・PDF・Insights 不採用                                         |
| テンプレート         | classic / repository                                                                                                                                                                                                        | terminal, markdown, community 不採用                                      |
| プラグイン (採用 21) | languages (+recent+indepth), activity, achievements, repositories, isocalendar, calendar, habits, stars, topics, starlists, people, notable, contributors, reactions, projects, sponsors, sponsorships, stargazers, traffic | 8 個の GitHub 系不採用、10 個の社外 API 系不採用、9 個の community 不採用 |
| 出力アクション       | commit / pull-request / data-changed                                                                                                                                                                                        | gist, markdown cache, workflow cleanup 不採用                             |
| 最適化               | CSS purge / XML format / twemoji / gemoji / octicons                                                                                                                                                                        | SVGO 不採用                                                               |
| 認証                 | (なし)                                                                                                                                                                                                                      | OAuth / Control / Restricted / rate-limit すべて Web 不採用に伴い不要     |
| 入力支援             | config_presets, 動的プレースホルダ                                                                                                                                                                                          |                                                                           |
| **総タスク数**       | **77**                                                                                                                                                                                                                      | (12-tasks.md の 132 から 55 を不採用)                                     |

## 2. 進行カレンダー (目安)

1 名担当 (兼任、週 25 時間想定) の目安:

| Phase                  | タスク数 | 期間     | 累計   |
| ---------------------- | -------- | -------- | ------ |
| M1 基盤                | 12       | 3 週間   | 3      |
| M2 エンジン + classic  | 16       | 4 週間   | 7      |
| M3 レンダリング        | 9        | 2 週間   | 9      |
| M4 プラグイン          | 21       | 5 週間   | 14     |
| M6 Action / CLI        | 8        | 2 週間   | 16     |
| M7 repository テンプレ | 1        | 0.5 週間 | 16.5   |
| M9 テスト基盤          | 6        | 1.5 週間 | 18     |
| M10 Docker / Release   | 4        | 1 週間   | 19     |
| (バッファ + 統合)      | -        | 2 週間   | **21** |

→ **約 5 ヶ月**。並列担当 2 名なら 3 ヶ月程度に短縮可能。

## 3. クリティカルパス

```
T-001 (repo init)
   │
   ├── T-002 logger ──┬─→ T-007 HTTP ──→ T-008 REST ─┬→ T-010 rate
   │                   ├─→ T-013 format               └→ T-009 GraphQL
   │                   └─→ T-003 settings
   │                                                    │
   │                                                    ▼
   ├── T-004 metadata ──→ T-005 inputs ──→ T-006 presets
   │
   └── T-012 embed.FS
                            │
                            ▼
              ┌──→ T-014 Plugin I/F ────┐
              │                          ├──→ T-016 Compute ──→ T-029 JSON (出力可)
              ├──→ T-015 Template I/F ──┤        │
              │                          │        ▼
              │      ┌──→ T-017 base.user ┐      T-021 core ──→ T-022 並列
              │      ├──→ T-018 base.org   │            │
              │      ├──→ T-019 indepth    │            ▼
              │      └──→ T-020 repos      ┘            T-023 classic ─→ T-024-028 partials
              │
              └──→ T-031 chromedp ──→ T-032 svg.Resize ──→ T-039 PNG/JPEG ─→ MVP 出力動作
                                            │
                                            ▼
                                  T-033 hash / T-034 css / T-035 xml / T-036 twe / T-037 ge / T-038 octi
                                            │
                                            ▼
                                  T-041〜T-068 (採用 21 プラグイン)
                                            │
                                            ▼
                                  T-105〜T-117 (Action) + T-118-120 (mock) + T-126/128/130 (Docker/release)
```

**最初に動かしたい MVP の最小完走**: T-001 → T-008/T-009 → T-016/T-017 → T-023/T-029 = **JSON 出力までで 19 タスク**。
そこから T-031/T-032/T-039 を足すと SVG/PNG 出力 = **22 タスクで「画像が出る」状態**。

---

# Phase M1: 基盤 (12 タスク)

## T-001: リポジトリ初期化

**Labels:** `phase:M1` `area:infra` `kind:chore` `priority:high`

**背景:** Go 移植の最初のコミット。`cmd/`, `internal/`, `assets/`, `api/` の各ディレクトリスケルトンを作る。

**スコープ:**

- `go.mod` 作成 (`module github.com/<org>/<repo>`, Go 1.23)。
- ディレクトリスケルトン作成 ([01-architecture.md §2](./01-architecture.md#2-go-パッケージ構成))。
- `Makefile` (build / test / lint / bench / gen / docker / e2e)。
- `.github/workflows/go-ci.yml`: PR で `go test ./...`, `go vet ./...`, `golangci-lint run`, `govulncheck ./...` を実行。

**Acceptance criteria:**

- [ ] `make build` で `bin/metrics-action`, `bin/metrics-cli` の空 main がビルドできる。
- [ ] `make test` がパスする (空テストで OK)。
- [ ] `golangci-lint` が CI で実行され緑になる。

**Blocked by:** (なし)
**Related specs:** [01-architecture.md](./01-architecture.md)

---

## T-002: ロガー / エラー / context ヘルパ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**背景:** 全パッケージで一貫した log 出力 (`slog`) と error wrapping のために共通ヘルパを最初に揃える。

**スコープ:**

- `internal/logger`: `slog.Default` 設定 (JSON / text 切替、`debug` フラグで level 切替)。
- `internal/errors`: `InputError`, `NotFoundError`, `ForbiddenError`, `UnsupportedFormatError`, `RetryableError` を定義 ([01-architecture.md §6](./01-architecture.md#6-エラーモデル))。
- context ヘルパ: `WithLogin(ctx, login)`, `LoginFromContext(ctx)`。

**Acceptance criteria:**

- [ ] エラー型は `errors.As` / `errors.Is` で識別可能。
- [ ] テストカバレッジ > 80%。

**Blocked by:** T-001
**Related specs:** [01-architecture.md §6](./01-architecture.md#6-エラーモデル)

---

## T-003: settings.json ローダ

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:** `//` キー (コメント) を捨てるパーサが必要。

**スコープ:**

- `internal/config/settings.go`: `Settings` 構造体 ([09-configuration.md §1.2](./09-configuration.md#12-スキーマ))、`Load(path)`、`Sandbox` フラグの強制設定処理。
- `getter NoToken() bool`。

**Acceptance criteria:**

- [ ] `tests/fixtures/settings/*.json` のロードケース網羅。
- [ ] `Sandbox=true` で強制設定が適用される。

**Blocked by:** T-001, T-002
**Related specs:** [09-configuration.md §1](./09-configuration.md#1-settingsjson-web-インスタンス)

---

## T-004: metadata.yml ローダ

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:** `assets/plugins/<name>/metadata.yml`, `assets/templates/<name>/metadata.yml`, `action.yml`, `assets/version.txt` を一括ロードする。

**スコープ:**

- `internal/config/metadata.go`: `PluginMetadata`, `TemplateMetadata`, `ActionMetadata`, `PackageMetadata` 構造体 ([05-plugins.md §3.2](./05-plugins.md#32-go-表現))。
- `Load(fsys fs.FS) (*MetadataLoader, error)`。
- `metadata.to.Action/Web/Query(key)` 変換関数。
- `Extras(name, settings) bool`。

**Acceptance criteria:**

- [ ] 採用 21 プラグイン + base/core + classic/repository の metadata.yml をすべてロードできる。
- [ ] inputs 定義の全フィールド (`type`, `default`, `format`, `values`, `global`, `preset`, `extras`) を保持。

**Blocked by:** T-001
**Related specs:** [09-configuration.md §4](./09-configuration.md#4-metadatayml-ローダ)

---

## T-005: 入力パーサ (型変換 + 動的プレースホルダ)

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:high`

**背景:** `metadata.yml` の `type:` に従って値を正規化する。

**スコープ:**

- `internal/config/inputs.go`:
  - `NormalizeInput(def InputDef, raw any) (any, error)`。
  - `Inputs.ForAction(env, preset)`, `Inputs.ForWeb(query)`, `Inputs.ForData(data, q, account)`。
- 配列 format (`comma-separated` / `space-separated` / `newline-separated`) 対応。
- boolean キャスト規則は [13-appendix.md §F](./13-appendix.md#f-入力正規化ルール一覧-legacyconverter-互換)。

**Acceptance criteria:**

- [ ] 各 type のテーブルテストパス。
- [ ] `.user.login` placeholder が `data.User.Login` から解決される。
- [ ] `token` 型は `String()` で `(provided)` を返す。

**Blocked by:** T-004
**Related specs:** [09-configuration.md §3](./09-configuration.md#3-入力解決の優先順位)

---

## T-006: preset ローダ

**Labels:** `phase:M1` `area:config` `kind:feature` `priority:medium`

**背景:** `config_presets` で `@name` / URL / ローカルパスから YAML をロードして `q` にマージ。

**スコープ:**

- `internal/config/presets.go`: `LoadPresets(ctx, list, meta, fetch) (map[string]any, error)`。
- YAML スキーマ `schema: v1`, `with: {...}` ([13-appendix.md §K](./13-appendix.md#k-presets-yaml-スキーマ-v1-例))。
- `preset: no` / `type: token` 入力は除外。

**Acceptance criteria:**

- [ ] `@maximum-content` 等の組込みを `assets/presets/<name>.yml` から読める。
- [ ] URL / ローカル両方テスト。

**Blocked by:** T-004, T-007
**Related specs:** [09-configuration.md §5](./09-configuration.md#5-presets-ローダ)

---

## T-007: HTTP クライアントラッパ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**スコープ:**

- `internal/httpx/client.go`: `Client` 構造体、`Get/PostJSON/PostForm/Binary`。
- `hashicorp/go-retryablehttp` で 5xx・429・network エラー → 指数バックオフ。4xx は再試行なし。
- User-Agent: `metrics/<version> (+https://github.com/<org>/<repo>)`。
- `imgb64(url)` ヘルパ ([04-rendering.md §11](./04-rendering.md#11-補助ユーティリティ))。

**Acceptance criteria:**

- [ ] `httptest.Server` で retry / timeout / status code 別挙動をテスト。
- [ ] `imgb64` で `data:image/png;base64,...` 文字列を返す。

**Blocked by:** T-002
**Related specs:** [08-external-services.md §7](./08-external-services.md#7-共通-http-クライアント方針)

---

## T-008: GitHub REST クライアントラッパ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**スコープ:**

- `internal/githubapi/rest.go`: `NewREST(token, customBaseURL)`、mock 用 `*http.Client` 差し替え可能。
- `internal/githubapi/auth.go`: token 種別判定 (`gh[pousr]_` / `github_pat_` / `NOT_NEEDED` / `MOCKED_TOKEN`)。
- `github_pat_` 拒否ロジック。

**Acceptance criteria:**

- [ ] `GET /rate_limit` のテストで `RateLimitResponse` が取れる。
- [ ] mocked クライアントが期待 path で受信する。

**Blocked by:** T-007
**Related specs:** [08-external-services.md §1.1](./08-external-services.md#11-rest)

---

## T-009: GitHub GraphQL クライアント (genqlient)

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:high`

**スコープ:**

- `Khan/genqlient` 設定 (`genqlient.yaml`)。
- `assets/plugins/base/queries/*.graphql` を変数化 ([13-appendix.md §A](./13-appendix.md#a-base-プラグインの-graphql-クエリ全文))。
- `internal/githubapi/graphql.go`: `NewGraphQL(token, customBaseURL)`、mock 対応。

**Acceptance criteria:**

- [ ] `make gen` で base の `user.graphql`, `user.x.graphql` から Go 関数生成。
- [ ] mock backend へのテストパス。

**Blocked by:** T-008
**Related specs:** [08-external-services.md §1.2](./08-external-services.md#12-graphql)

---

## T-010: API レート状態トラッカ

**Labels:** `phase:M1` `area:infra` `kind:feature` `priority:medium`

**スコープ:**

- `internal/githubapi/rate.go`: `Resources{Rest, GraphQL, Search}` (`Limit/Used/Remaining/Reset`)、`Refresh(ctx)`。

**Acceptance criteria:**

- [ ] mocked rate-limit エンドポイントから値が反映される。
- [ ] race detector clean。

**Blocked by:** T-008
**Related specs:** [02-action.md §3.3](./02-action.md#33-token-validation)

---

## T-012: アセット埋め込みパイプライン

**Labels:** `phase:M1` `area:infra` `kind:chore` `priority:medium`

**スコープ:**

- `assets/plugins/*` (queries, metadata, examples) を新規リポジトリへコピー (`make sync-assets`)。
- `assets/templates/*` 同上 (classic + repository のみ)。
- `assets/octicons/data.json` を `@primer/octicons` から抽出する `internal/tools/gen-octicons/main.go`。
- `assets/twemoji/index.json` を Twemoji リポジトリから抽出する `internal/tools/gen-twemoji/main.go`。

**Acceptance criteria:**

- [ ] `make gen` で octicons / twemoji 再生成。
- [ ] `embed.FS` から各 plugin の queries が読める。

**Blocked by:** T-001
**Related specs:** [10-testing-deployment.md §4.3](./10-testing-deployment.md#43-embedfs)

---

## T-013: フォーマッタ群

**Labels:** `phase:M1` `area:render` `kind:feature` `priority:high` `good-first-issue`

**スコープ:**

- `internal/format/format.go`:
  - `Format(n, opts)`, `FormatBytes(n)`, `FormatPercentage(n, opts)`, `FormatDate(t, opts)`, `Ellipsis(s, n)`, `S(n, suffix)`, `FormatError(err, opts)`。

**Acceptance criteria:**

- [ ] 境界値 (`0, 999, 1000, 999999, 1000000`) のテーブルテスト。
- [ ] timezone 切替が正しい。

**Blocked by:** T-001
**Related specs:** [04-rendering.md §2.3](./04-rendering.md#23-共通-helper)

---

# Phase M2: コアエンジン + classic (16 タスク)

## T-014: Plugin インタフェース + Registry

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/plugin.go`:
  - `Plugin interface{ Name(); Metadata(); Run(ctx, *PluginContext) (any, error) }`。
  - `registry`, `Register(p)`, `Get(name)`, `Each(fn)`, `RegisterForTest(p)`。
- `PluginContext` 構造体 ([01-architecture.md §4](./01-architecture.md#4-主要データ型))。

**Acceptance criteria:**

- [ ] 二重登録で panic。

**Blocked by:** T-002, T-004
**Related specs:** [05-plugins.md §2](./05-plugins.md#2-プラグインインタフェース)

---

## T-015: Template インタフェース + Registry

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `internal/templates/template.go`: `Template` インタフェース、`Register/Get`、`PartialFunc`、`PartialContext`。

**Acceptance criteria:**

- [ ] `Check(q, account, format)` で format / account / repo 必須を検証。

**Blocked by:** T-014
**Related specs:** [07-templates.md §2](./07-templates.md#2-テンプレートインタフェース)

---

## T-016: engine.Compute オーケストレータ

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**スコープ:**

- `internal/engine/engine.go`: `Compute(ctx, req, deps) (Result, error)`。
- 順序: テンプレート存在検証 → Convert 決定 → partial 順マージ → Imports 構築 → base plugin → template.Run → goroutine 群完了待ち → エラー集約 → 出力変換。
- `die=true` での即時 fail / `die=false` で footer 出力。

**Acceptance criteria:**

- [ ] mock GitHub クライアントで classic の最小 SVG が生成される。
- [ ] `Convert=json` で JSON が返る (T-029 完了時)。

**Blocked by:** T-014, T-015, T-017, T-021, T-023
**Related specs:** [01-architecture.md §5](./01-architecture.md#5-実行フロー)

---

## T-017: base plugin — user アカウント

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/base/base.go` の user ケース。
- bulk クエリ `user.x.graphql` 実行、失敗時に field 単位 fallback ([13-appendix.md §B](./13-appendix.md#b-base-プラグインの取得アルゴリズム-擬似コード))。
- `data.User`, `data.User.ContributionsCollection`, `data.User.Calendar` をセット。

**Acceptance criteria:**

- [ ] mock GraphQL backend での integration テスト。
- [ ] bulk failure → unit query fallback のテスト。

**Blocked by:** T-009, T-014
**Related specs:** [05-plugins.md §5](./05-plugins.md#5-base-プラグインの特殊扱い)

---

## T-018: base plugin — organization アカウント

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/base/organization.go`。
- field 単位 fallback: `packages`, `sponsorshipsAsSponsor/Maintainer`, `membersWithRole`。

**Acceptance criteria:**

- [ ] mock backend テスト。

**Blocked by:** T-017
**Related specs:** [05-plugins.md §5](./05-plugins.md#5-base-プラグインの特殊扱い)

---

## T-019: base plugin — indepth contributions

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:medium`

**背景:** achievements の閾値判定で commits 全期間集計を使うため精度向上目的で採用。

**スコープ:**

- `internal/plugins/base/indepth.go`:
  - `data.User.CreatedAt` から現在まで 4 週間 windows でループ。
  - 各 window で `queries.base.contributions(login, field, range)` 実行し合算。
  - `search.commits(author:<login>)` で全期間 commits を補正。
  - `metadata.api.github.overuse` extras フラグでガード。

**Acceptance criteria:**

- [ ] mock backend テスト (3 年分のループ実行検証)。
- [ ] extras 無効時は skip。

**Blocked by:** T-017
**Related specs:** [05-plugins.md §5.2](./05-plugins.md#52-動作)

---

## T-020: base plugin — repositories ページング

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/base/repositories.go`:
  - `queries.base.repositories` を `pageInfo.hasNextPage` か `settings.repositories` 到達まで回す。
  - timeout 時は batch を半減してリトライ。
  - `repositories_forks/affiliations/skipped` 適用。
- `data.Computed.{Commits, Repositories.{Stargazers/Forks/Releases/...}}` の集計。

**Acceptance criteria:**

- [ ] 250 件 (3 ページ) のページング検証。
- [ ] `repositories_skipped` でフィルタ可能。

**Blocked by:** T-017
**Related specs:** [05-plugins.md §5.4](./05-plugins.md#54-リポジトリ取得)

---

## T-021: core plugin — グローバル設定注入

**Labels:** `phase:M2` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/core/core.go`:
  - inputs 解釈 (`config.timezone`, `config.animations`, `config.display`, `config.base64`, `debug.flags`)。
  - `time.LoadLocation` でタイムゾーン解決、失敗時 error フィールド。
  - `data.Computed` の zero value 初期化。

**Acceptance criteria:**

- [ ] Asia/Tokyo 指定で `data.Config.Timezone.Name == "Asia/Tokyo"`。
- [ ] 不正タイムゾーンで `error` フィールドセット。

**Blocked by:** T-014, T-013, T-005
**Related specs:** [05-plugins.md §6](./05-plugins.md#6-core-プラグインの役割)

---

## T-022: core plugin — プラグイン並列実行 + callback

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**スコープ:**

- `internal/plugins/core/run_plugins.go`:
  - `errgroup.SetLimit(parallel)` で並列度制御。
  - エラーは `data.Plugins[name] = err`。
  - `callbacks.Plugin(login, name, success, result)` の呼び出し。
- panic recover → error 化。

**Acceptance criteria:**

- [ ] 3 つの mock plugin で並列実行成功。
- [ ] 1 つ panic でも他は正常完了。

**Blocked by:** T-021
**Related specs:** [05-plugins.md §7](./05-plugins.md#7-並列実行とエラー集約)

---

## T-023: classic テンプレートスケルトン

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `internal/templates/classic/classic.go`: `Template` 実装、`//go:embed`。
- `image.svg` スケルトン: [13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン) と互換の DOM。
- `Run(ctx, p)` で `plugins.core` を呼ぶ。
- `Check` で account/format 判定。
- **MVP 注意**: `partials/_.json` は採用プラグイン分のみ含めるよう調整 (N-001 で MVP 用 \_.json 生成)。

**Acceptance criteria:**

- [ ] 空 SVG が生成される (DOM 構造は `<svg>` + `<foreignObject>` + `items-wrapper`)。

**Blocked by:** T-015, T-021
**Related specs:** [07-templates.md §3](./07-templates.md#3-共通レイアウト)

---

## T-024: classic partial — base.header

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `internal/templates/classic/partials/base_header.go` に `PartialFunc` を実装。
- DOM 構造は [13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン) と互換。
- 動的式は `internal/render/helpers.go` ヘルパで処理。

**Acceptance criteria:**

- [ ] mock data → goldenfile (`tests/golden/base_header.svg.frag`) と一致。
- [ ] 不在キーで panic せず空文字列出力。

**Blocked by:** T-023, T-017, T-020
**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

## T-025: classic partial — introduction

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**背景:** introduction プラグインは MVP 不採用だが、partial は `_.json` 既定順に含まれるため stub 実装が必要。

**スコープ:**

- `internal/templates/classic/partials/introduction.go`。
- `if data.Plugins["introduction"] != nil` で本体描画、それ以外は空文字列。
- 出典: `source/templates/classic/partials/introduction.ejs`。

**Acceptance criteria:**

- [ ] introduction 不在で何も出力しない。

**Blocked by:** T-023
**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

## T-026: classic partial — base.activity+community

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `internal/templates/classic/partials/base_activity_community.go`。
- 出典: `source/templates/classic/partials/base.activity+community.ejs`。

**Acceptance criteria:**

- [ ] base.activity / base.community 表示。

**Blocked by:** T-023, T-017
**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

## T-027: classic partial — base.repositories

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `internal/templates/classic/partials/base_repositories.go`。
- 出典: `source/templates/classic/partials/base.repositories.ejs`。

**Acceptance criteria:**

- [ ] base.repositories 表示。

**Blocked by:** T-023, T-020
**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

## T-028: classic — metadata footer

**Labels:** `phase:M2` `area:templates` `kind:feature` `priority:high`

**スコープ:**

- `image.svg` 末尾の `<% if (base.metadata) %>` 部 (timezone / version / generated)。

**Acceptance criteria:**

- [ ] base.metadata=true でフッタ表示。

**Blocked by:** T-023
**Related specs:** [13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン)

---

## T-029: JSON 出力モード

**Labels:** `phase:M2` `area:engine` `kind:feature` `priority:high`

**スコープ:**

- `internal/engine/json.go`: `Marshal(data *Data) ([]byte, error)`。
- 循環参照を `"[Circular]"` で潰す cycleDetector。
- `Set` → `[]T`、`Map` → `map[string]any`。

**Acceptance criteria:**

- [ ] `tests/cases/*.yml` の `config_output=json` ケースで JSON キー集合がリファレンスと一致。
- [ ] 循環参照で panic しない。

**Blocked by:** T-016
**Related specs:** [04-rendering.md §7.1](./04-rendering.md#71-json)

---

# Phase M3: レンダリングパイプライン (9 タスク)

## T-031: chromedp ラッパ

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**スコープ:**

- `internal/render/chrome.go`: `Browser{allocCtx, parentCtx}`、`New(opts)`, `NewTab(ctx)`, `Close()`。
- 起動オプション: `--no-sandbox`, `--disable-extensions`, `--disable-dev-shm-usage`, `--disable-gpu`, `--single-process`。
- `METRICS_CHROME_PATH` 環境変数を尊重。
- N 回ごとに再起動 (`SettingsBrowserRecycle`、既定 200)。
- debug flag マッピング (`--puppeteer-disable-headless` 等)。

**Acceptance criteria:**

- [ ] Docker 内で `chromium` を起動できる integration テスト。
- [ ] 200 回利用後に再起動が発生。

**Blocked by:** T-002
**Related specs:** [04-rendering.md §3](./04-rendering.md#3-svg-リサイズ-chromedp)

---

## T-032: svg.Resize

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**スコープ:**

- `internal/render/svg_resize.go`: `Resize(ctx, rendered, opts) (string, MIME, error)`。
- 評価 JS とアルゴリズムは [13-appendix.md §G](./13-appendix.md#g-svgresize-の-chromedp-評価スクリプト) 参照。
- viewport 980x980、`no-animations` 付与、2.4 秒 sleep、`#metrics-end` で計測。

**Acceptance criteria:**

- [ ] サンプル SVG のリサイズが期待 height 範囲。
- [ ] PNG 出力ができる (T-039 と統合)。

**Blocked by:** T-031
**Related specs:** [04-rendering.md §3](./04-rendering.md#3-svg-リサイズ-chromedp)

---

## T-033: svg.Hash

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**スコープ:**

- `internal/render/svg_hash.go`: `Hash(rendered string) (string, error)`。
- `goquery` で `<footer>` 除去 → `<svg>` outerHTML → MD5 hex。
- アルゴリズム: [13-appendix.md §H](./13-appendix.md#h-svghash-の正規化アルゴリズム)。

**Acceptance criteria:**

- [ ] footer 削除後の outer HTML が同じなら MD5 同一。

**Blocked by:** T-001
**Related specs:** [04-rendering.md §10](./04-rendering.md#10-svg-ハッシュ-差分判定)

---

## T-034: CSS optimizer

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**スコープ:**

- `internal/render/css.go`: `OptimizeCSS(rendered) (string, error)`。
- `<style data-optimizable="true">…</style>` を抽出 → 未使用セレクタ削除 → minify。
- `tdewolff/parse/v2/css` + `cascadia` + `tdewolff/minify/v2/css`。

**Acceptance criteria:**

- [ ] 未使用クラスを 50% 削減できる固定ケース。

**Blocked by:** T-001
**Related specs:** [04-rendering.md §4.1](./04-rendering.md#41-css-最適化-svgoptimizecss)

---

## T-035: XML 整形

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:low`

**スコープ:**

- `internal/render/xml_format.go`: `FormatXML(rendered) (string, error)`。
- `lineSeparator="\n"`, `collapseContent=true` で整形。

**Acceptance criteria:**

- [ ] 入れ子要素のインデントが期待通り。

**Blocked by:** T-001
**Related specs:** [04-rendering.md §4.2](./04-rendering.md#42-xml-整形-svgoptimizexml)

---

## T-036: twemoji 置換

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**スコープ:**

- `internal/render/twemoji.go`: Unicode emoji を抽出 → SVG fetch → `<svg class="twemoji">` 置換。
- in-memory LRU キャッシュ。

**Acceptance criteria:**

- [ ] `📊 hello 🚀` → emoji が `<svg class="twemoji">` に置換。

**Blocked by:** T-007, T-012
**Related specs:** [04-rendering.md §8.1](./04-rendering.md#81-twemoji)

---

## T-037: gemoji 置換

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:low`

**スコープ:**

- `internal/render/gemoji.go`: `GET /emojis` で name→url、`:name:` を `<img class="gemoji">` 置換。

**Acceptance criteria:**

- [ ] mock REST で `:octocat:` が `<img class="gemoji">` に置換。

**Blocked by:** T-007, T-008
**Related specs:** [04-rendering.md §8.2](./04-rendering.md#82-gemoji)

---

## T-038: octicon 置換

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:medium`

**スコープ:**

- `internal/render/octicon.go`: `assets/octicons/data.json` (T-012 生成) を `embed`。
- `:octicon-<name>-<size>:` を `<svg>` 置換、size 省略時は 16px。

**Acceptance criteria:**

- [ ] `:octicon-star-24:` が 24px SVG に。
- [ ] 未知 octicon は素通し。

**Blocked by:** T-012
**Related specs:** [04-rendering.md §8.3](./04-rendering.md#83-octicon)

---

## T-039: PNG / JPEG 出力統合

**Labels:** `phase:M3` `area:render` `kind:feature` `priority:high`

**スコープ:**

- `engine.Compute` の出力フェーズで `Convert ∈ {png, jpeg}` のとき `svg.Resize` の `convert` 引数を渡す。
- MIME: `image/png` / `image/jpeg`。

**Acceptance criteria:**

- [ ] SVG → PNG 出力が `image.Decode` で成功する。

**Blocked by:** T-032
**Related specs:** [04-rendering.md §3.1](./04-rendering.md#31-アルゴリズム)

---

# Phase M4: GitHub プラグイン (21 タスク)

> 各プラグインは `internal/plugins/<name>/<name>.go` で `Plugin` インタフェースを実装し、`init()` で `Register()` する。
> 各タスクは **本体実装 + partial 実装 + テスト** を含む。
> 共通 Acceptance criteria:
>
> - [ ] Run() が mocked dependency でエラーなく完了。
> - [ ] 期待する `data.Plugins[<name>]` 構造が出来上がる。
> - [ ] 該当する partial で SVG 表示できる。

## T-041: plugin languages (標準モード)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- `data.User.Repositories.Nodes` の `languages.edges.{size, node.{name, color}}` を集計。
- `plugin_languages_{limit, threshold, ignored, skipped, aliases, colors, other}` 反映。

**Blocked by:** T-014, T-020
**Related specs:** [06-plugins-detail.md §2.12](./06-plugins-detail.md#212-languages)

---

## T-042: plugin languages.recent

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- `plugin_languages_sections` に `recently-used` が含まれる場合の分岐。
- REST `/users/.../events` から PushEvent → commit diff (`/repos/.../commits/{sha}`) → 言語判定 (go-enry)。
- `plugin_languages_recent_{load, days, categories}` 反映。

**Blocked by:** T-041
**Related specs:** [06-plugins-detail.md §2.12](./06-plugins-detail.md#212-languages)

---

## T-043: plugin languages.indepth

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- `plugin_languages_indepth=true` で `go-git` clone + `go-enry` 解析。
- 一時ディレクトリ管理 + defer cleanup。
- 個別タイムアウト `plugin_languages_analysis_timeout_repositories` (7.5 分)、全体 15 分。
- extras `metrics.cpu.overuse` + `metrics.run.git` でガード。

**Blocked by:** T-041
**Related specs:** [08-external-services.md §5](./08-external-services.md#5-git--linguist)

---

## T-044: plugin activity

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- REST `/users/{login}/events` を `plugin_activity_load` (300) 件まで取得。
- イベントタイプフィルタ。
- `plugin_activity_{limit, days, visibility, timestamps, skipped, ignored}` 反映。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.2](./06-plugins-detail.md#22-activity)

---

## T-045: plugin achievements

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:high`

**スコープ:**

- ランク計算 (`rank(x, [c, b, a, s, m])` テーブル、[06-plugins-detail.md §2.1](./06-plugins-detail.md#21-achievements))。
- 各統計値 (commits, repositories, stars, followers, organizations, ...) をランクに照合。
- `plugin_achievements_{threshold, secrets, display, limit, only, ignored}` 反映。
- (option) Profile ページの chromedp scrape は後回しで OK。

**Blocked by:** T-017, T-020, T-019 (indepth で精度向上)
**Related specs:** [06-plugins-detail.md §2.1](./06-plugins-detail.md#21-achievements)

---

## T-046: plugin repositories

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- featured / pinned (`pinnedItems`) / starred (`user.repositories(orderBy:STARGAZERS)`) / random。
- `plugin_repositories_{order, pinned, starred, random, affiliations, forks, skipped, batch}`。

**Blocked by:** T-014, T-020
**Related specs:** [06-plugins-detail.md §2.19](./06-plugins-detail.md#219-repositories)

---

## T-049: plugin isocalendar

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- `plugin_isocalendar_duration` (`half-year`/`full-year`) で `contributionsCollection.contributionCalendar` 取得。
- streak (Max, Current), average, sum を計算。

**Blocked by:** T-014, T-017
**Related specs:** [06-plugins-detail.md §2.11](./06-plugins-detail.md#211-isocalendar)

---

## T-050: plugin calendar

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- 年単位の `contributionsCollection.contributionCalendar` を `plugin_calendar_limit` ループで取得。

**Blocked by:** T-014, T-017
**Related specs:** [06-plugins-detail.md §2.3](./06-plugins-detail.md#23-calendar)

---

## T-051: plugin habits

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- REST `/users/.../events` から PushEvent を `plugin_habits_from` (200) 件取得。
- 各コミットの diff から: 曜日別 / 時間帯別 / indent / 行平均文字数。
- linguist 統合 (`plugin_habits_languages.recent`)。
- `plugin_habits_{days, facts, charts, charts_type, trim}`。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.9](./06-plugins-detail.md#29-habits)

---

## T-052: plugin stars

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low` `good-first-issue`

**スコープ:**

- GraphQL `user.starredRepositories(orderBy:STARRED_AT)` を `plugin_stars_limit` (4) 件取得。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.25](./06-plugins-detail.md#225-stars)

---

## T-053: plugin topics

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- chromedp で `https://github.com/stars/<login>/topics` 巡回。
- DOM から `name, description, icon, url` 抽出。
- `plugin_topics_{mode, limit, sort}`。
- extras `metrics.run.puppeteer.scrapping` でガード。

**Blocked by:** T-014, T-031
**Related specs:** [06-plugins-detail.md §2.27](./06-plugins-detail.md#227-topics)

---

## T-054: plugin starlists

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- chromedp で GitHub Star Lists ページ巡回。
- `plugin_starlists_languages` (bool) で言語分析モード (T-041 のロジックを再利用)。

**Blocked by:** T-053, T-041
**Related specs:** [06-plugins-detail.md §2.24](./06-plugins-detail.md#224-starlists)

---

## T-055: plugin people

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL followers / following / sponsors / contributors / stargazers / watchers / members を `plugin_people_types` ごとに取得。
- `plugin_people_{limit, size, shuffle, identicons}`。

**Blocked by:** T-014, T-017
**Related specs:** [06-plugins-detail.md §2.16](./06-plugins-detail.md#216-people)

---

## T-056: plugin notable

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL repositoryOwner, REST commits / Issues, organizations。
- `plugin_notable_{filter, repositories, indepth, types, from, self}`。

**Blocked by:** T-014, T-017
**Related specs:** [06-plugins-detail.md §2.15](./06-plugins-detail.md#215-notable)

---

## T-059: plugin contributors

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- REST `/repos/.../stats/contributors`, GraphQL `repository.refs.commits` で base/head 範囲取得。
- `plugin_contributors_{base, head, contributions, sections, ignored}`。
- repository テンプレ向け。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.5](./06-plugins-detail.md#25-contributors)

---

## T-062: plugin reactions

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL issues/comments の reactions 集計。
- `plugin_reactions_{limit_*, days, details, ignored}`。

**Blocked by:** T-014, T-017
**Related specs:** [06-plugins-detail.md §2.18](./06-plugins-detail.md#218-reactions)

---

## T-063: plugin projects

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL `user.projects` / `repository.projects`。
- `plugin_projects_{limit, repositories, descriptions}`。
- `read:project` scope 必要。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.17](./06-plugins-detail.md#217-projects)

---

## T-064: plugin sponsors

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL `user.sponsorsListing`, `sponsorsForViewer`。
- `plugin_sponsors_{sections, size, title, past}`。
- `read:user`, `read:org` scope 必要。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.21](./06-plugins-detail.md#221-sponsors)

---

## T-065: plugin sponsorships

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- GraphQL `viewer.sponsorshipsAsSponsor(activeOnly:false)`。
- `plugin_sponsorships_sections`。

**Blocked by:** T-014
**Related specs:** [06-plugins-detail.md §2.22](./06-plugins-detail.md#222-sponsorships)

---

## T-066: plugin stargazers (worldmap オプション)

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:medium`

**スコープ:**

- GraphQL `repository.stargazers(orderBy:STARRED_AT)`。
- (option) Google Maps Geocoding で worldmap (location → 緯度経度)。
- `plugin_stargazers_{charts, charts_type, worldmap, worldmap_token, worldmap_sample}`。

**Blocked by:** T-014, T-007
**Related specs:** [06-plugins-detail.md §2.23](./06-plugins-detail.md#223-stargazers)

---

## T-068: plugin traffic

**Labels:** `phase:M4` `area:plugins` `kind:feature` `priority:low`

**スコープ:**

- REST `/repos/.../traffic/views` を repositories に並列。
- token に `repo` scope が無ければ warning。

**Blocked by:** T-014, T-020
**Related specs:** [06-plugins-detail.md §2.28](./06-plugins-detail.md#228-traffic)

---

# Phase M6: GitHub Action / CLI (8 タスク)

## T-105: action entrypoint (skip 判定 + setup)

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**

- `cmd/metrics-action/main.go`:
  - `[Skip GitHub Action]` / `Auto-generated metrics for run #N` commit message でスキップ。
  - `setup()` 呼び出し、Docker 環境補完 (`INPUT_OUTPUT_ACTION`, `INPUT_COMMITTER_TOKEN`, `GITHUB_REPOSITORY`)。
  - 起動バナー出力 ([13-appendix.md §E](./13-appendix.md#e-action-起動バナーの整形ルール-info-互換))。

**Acceptance criteria:**

- [ ] スキップケース 2 種で exit 0。
- [ ] バナーに version 表示。

**Blocked by:** T-003, T-004
**Related specs:** [02-action.md §3](./02-action.md#3-実行フェーズ)

---

## T-106: INPUT\_\* パーサ + presets 統合

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**

- `INPUTS` JSON を最優先、なければ `INPUT_<UPPER>` を読む。
- `config_presets` を `LoadPresets` で読み込み、`q` 展開。
- `metadata.plugins.core.Inputs.ForAction(env, preset)` で `config_*` を抽出。
- `_filename` のワイルドカード `*` を `convert` に応じた拡張子に置換。

**Acceptance criteria:**

- [ ] テスト用 inputs.yaml をロード、`q` map に正しいキーで展開。

**Blocked by:** T-005, T-006, T-105
**Related specs:** [02-action.md §2.2](./02-action.md#22-値の型変換)

---

## T-108: token validation + rate quota check

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**

- token 未指定 / `github_pat_` 拒否。
- `GET /rate_limit` で残量チェック (`quota_required_rest/graphql/search`)。
- `HEAD /` で `X-OAuth-Scopes` 確認。
- `notice_releases=true` のとき新バージョン通知 (`GET /repos/<org>/<repo>/releases`)。

**Acceptance criteria:**

- [ ] token 不足で exit 1。
- [ ] quota 不足で skipped 終了。

**Blocked by:** T-008, T-010, T-105
**Related specs:** [02-action.md §3.3](./02-action.md#33-token-validation)

---

## T-109: render + retry 統合

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**

- `internal/action/retry.go`: `Retry(ctx, fn, retries, delay)` 共通実装。
- `engine.Compute` を `retries`/`retries_delay` で繰り返し。
- `/renders/<filename>` 書き出し (mkdir -p)。
- `dryrun=true` ならファイル書き出しのみで output_action はスキップ。

**Acceptance criteria:**

- [ ] mocked Compute が 2 回失敗 → 3 回目成功で正常終了。

**Blocked by:** T-016, T-106
**Related specs:** [02-action.md §3.9](./02-action.md#39-render)

---

## T-110: committer — commit モード

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

## T-111: committer — pull-request

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:high`

**スコープ:**

- `pull-request*` モードで `metrics-run-${runId}` head ブランチ作成。
- `POST /repos/.../pulls` で PR 作成。
- `pull-request-{merge|squash|rebase}` で `PUT /repos/.../pulls/{n}/merge`。
- 失敗は warning ログで継続。

**Acceptance criteria:**

- [ ] PR 作成 → mergeable=true → 自動 merge ケーステスト。

**Blocked by:** T-110
**Related specs:** [02-action.md §4.4](./02-action.md#44-pr-作成--マージ)

---

## T-114: output_condition=data-changed

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:medium`

**スコープ:**

- 既存ファイル取得 (`GET /repos/.../contents/<filename>`)、`svg.Hash` で比較。
- 一致なら `committer.Commit=false`。

**Acceptance criteria:**

- [ ] 一致時に commit がスキップされる。

**Blocked by:** T-033, T-110
**Related specs:** [02-action.md §3.10](./02-action.md#310-output-condition)

---

## T-117: CLI 専用フラグ

**Labels:** `phase:M6` `area:action` `kind:feature` `priority:medium`

**スコープ:**

- `cobra` または `flag` で CLI フラグ:
  - `--config <path>`, `--user`, `--template`, `--token`, `--plugin key=val`, `--output`, `--filename`, `--dryrun`, `--token-env KEY=ENV`。
- YAML を `INPUTS` 相当に整形してから既存パイプラインへ流す。

**Acceptance criteria:**

- [ ] `metrics-action --user octocat --template classic --output svg --dryrun` で SVG が標準出力。

**Blocked by:** T-105, T-106
**Related specs:** [02-action.md §7](./02-action.md#7-cli-モード-actionyml-に依存しない直接利用)

---

# Phase M7: repository テンプレート (1 タスク)

## T-089: repository テンプレート

**Labels:** `phase:M7` `area:templates` `kind:feature` `priority:medium`

**スコープ:**

- `internal/templates/repository/repository.go`。
- repository 専用 partial (base.repository, introduction, base.community, base.activity)。
- `Check` で `account != "repository"` を 406。

**Acceptance criteria:**

- [ ] mock data で SVG 生成。

**Blocked by:** T-015, T-021
**Related specs:** [07-templates.md §6](./07-templates.md#6-repository-テンプレート)

---

# Phase M9: テスト基盤 (6 タスク)

## T-118: testutil/mocks — REST モック

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**

- `internal/testutil/mocks/rest.go`: `MockTransport` (`http.RoundTripper` 実装)。
- `tests/fixtures/github/rest/<endpoint>.json` から自動応答。

**Acceptance criteria:**

- [ ] `mock.GET("/rate_limit", "rate_limit.json")` で差し替え可能。

**Blocked by:** T-008
**Related specs:** [10-testing-deployment.md §2](./10-testing-deployment.md#2-mocks-の設計)

---

## T-119: testutil/mocks — GraphQL モック

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**

- `internal/testutil/mocks/graphql.go`: genqlient `Doer` 実装。
- クエリ名 (`operationName`) で fixture を選択。
- `tests/fixtures/github/graphql/<name>.json` を自動ロード。

**Acceptance criteria:**

- [ ] base.user / base.user.x を mock 経由で取得成功。

**Blocked by:** T-009
**Related specs:** [10-testing-deployment.md §2.1](./10-testing-deployment.md#21-mocked-api)

---

## T-120: golden file テストフレームワーク

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**

- `internal/testutil/golden`: SVG / JSON 結果を正規化して `tests/golden/<case>.{svg,json}` と比較。
- SVG は footer の動的部分 (timestamps, version) を mask。
- `-update` フラグで golden 更新。

**Acceptance criteria:**

- [ ] `make test-golden` の初期 case (`classic_octocat`) を 1 つ追加し緑。

**Blocked by:** T-118, T-119
**Related specs:** [10-testing-deployment.md §1.1](./10-testing-deployment.md#11-階層)

---

## T-121: e2e — classic SVG ラウンドトリップ

**Labels:** `phase:M9` `area:test` `kind:test` `priority:high`

**スコープ:**

- `tests/integration/compute_test.go`: mock backend + classic + 採用 21 プラグインの一部で `engine.Compute` 実行 → golden と比較。

**Blocked by:** T-016, T-041〜T-068 (採用分), T-120
**Related specs:** [10-testing-deployment.md §1.2](./10-testing-deployment.md#12-テスト構成)

---

## T-122: e2e — action dryrun

**Labels:** `phase:M9` `area:test` `kind:test` `priority:medium`

**スコープ:**

- `tests/integration/action_test.go`: `metrics-action --dryrun` を子プロセスで起動 (`os/exec`)、INPUTS 環境変数注入。
- `/renders/<filename>` が期待通り生成。

**Blocked by:** T-105〜T-117, T-118
**Related specs:** [10-testing-deployment.md §1.1](./10-testing-deployment.md#11-階層)

---

## T-125: linter / govulncheck の CI 統合

**Labels:** `phase:M9` `area:infra` `kind:chore` `priority:medium`

**スコープ:**

- `.github/workflows/go-ci.yml` に `golangci-lint run --timeout=10m` と `govulncheck ./...` 追加。
- staticcheck, gosec, revive, gofumpt をリンタ集合に。

**Blocked by:** T-001
**Related specs:** [10-testing-deployment.md §7.2](./10-testing-deployment.md#72-追加ジョブ)

---

# Phase M10: リリース / Docker (4 タスク)

## T-126: Dockerfile (Chrome 同梱)

**Labels:** `phase:M10` `area:infra` `kind:chore` `priority:high`

**スコープ:**

- `deploy/Dockerfile`: multi-stage (`golang:1.23-bookworm` → `debian:bookworm-slim` + chromium + fonts)。
- multi-arch (linux/amd64, linux/arm64) を `docker/buildx`。
- `METRICS_CHROME_PATH=/usr/bin/chromium` 環境変数。

**Acceptance criteria:**

- [ ] `docker run` で `metrics-action --help` が実行できる。
- [ ] イメージサイズ < 600 MB。

**Blocked by:** T-117
**Related specs:** [10-testing-deployment.md §5](./10-testing-deployment.md#5-docker)

---

## T-128: action.yml の Go バイナリ呼び出し化

**Labels:** `phase:M10` `area:action` `kind:chore` `priority:high`

**スコープ:**

- `action.yml` の `runs.using: composite` を Go バイナリ呼び出しに更新。
- リリース後に `<org>/<repo>@v1` で参照可能にする。

**Acceptance criteria:**

- [ ] サンプルワークフローで成功。

**Blocked by:** T-126
**Related specs:** [02-action.md §1](./02-action.md#1-概要)

---

## T-130: リリーススクリプト + GitHub Release 公開

**Labels:** `phase:M10` `area:infra` `kind:chore` `priority:high`

**スコープ:**

- `scripts/release.sh`: クロスコンパイル (linux/amd64, linux/arm64, darwin/arm64, windows/amd64)、checksum、署名。
- `.github/workflows/release.yml`: タグ `v*` で起動、Docker push + GitHub Release 作成。

**Acceptance criteria:**

- [ ] dry-run で release-notes が生成される。

**Blocked by:** T-126
**Related specs:** [10-testing-deployment.md §6](./10-testing-deployment.md#6-リリース)

---

## T-131: マイグレーションガイド

**Labels:** `phase:M10` `area:docs` `kind:docs` `priority:high`

**スコープ:**

- `docs/migration-to-go.md` を作成。
- 既知の差分 (community templates 未対応、Markdown 系・PDF・Insights 未対応、社外 API 系プラグイン未対応など)。
- ロールバック方法。

**Acceptance criteria:**

- [ ] レビュー OK。

**Blocked by:** T-128
**Related specs:** [11-go-migration.md §4.4](./11-go-migration.md#44-マイグレーションガイド-公開)

---

# MVP 追加タスク (N-001〜N-002)

選定結果に伴って **元の T-XXX には無い** 追加タスクを下記の通り定義する。

## N-001: classic partials/\_.json の MVP 用フィルタ

**Labels:** `phase:M2` `area:templates` `kind:chore` `priority:high`

**背景:** classic のデフォルト `_.json` は 46 partial を含む。採用していないプラグインの partial を載せると EJS include が失敗するため、MVP 用に絞った `_.json` を用意する必要がある。

**スコープ:**

- `assets/templates/classic/partials/_.json` (MVP 用) を以下に変更:
  ```json
  [
    "base.header",
    "introduction",
    "base.activity+community",
    "base.repositories",
    "languages",
    "repositories",
    "habits",
    "topics",
    "isocalendar",
    "calendar",
    "stars",
    "starlists",
    "stargazers",
    "people",
    "activity",
    "reactions",
    "achievements",
    "sponsors",
    "sponsorships",
    "notable",
    "contributors",
    "projects",
    "traffic"
  ]
  ```
- 後から追加採用したい場合は `_.json` に追記するだけで extends 可能な構造を維持。
- T-023 (classic skeleton) と同時 or 直後に着手。

**Acceptance criteria:**

- [ ] 採用プラグインの partial のみが含まれる。
- [ ] partial loop で missing file エラーが発生しない。
- [ ] `_.json` の並び順が表示順を表す。

**Blocked by:** T-023
**Related specs:** [07-templates.md §4](./07-templates.md#4-partial-機構)

---

## N-002: chromedp 依存プラグインの動作確認 (topics / starlists)

**Labels:** `phase:M4` `area:test` `kind:test` `priority:medium`

**背景:** `topics` (T-053) と `starlists` (T-054) は chromedp scrape を伴う。MVP では確実に動かしたい採用機能なので、scraping が GitHub の UI 変更で壊れるリスクを早期検出する e2e を 1 本入れる。

**スコープ:**

- `tests/integration/scrape_test.go`: 実際の GitHub ページに対する smoke test (CI では `-tags=online` でオプト)。
- 失敗時は warning ログを残し、テスト全体は failure としない (UI 変更検知のため)。

**Acceptance criteria:**

- [ ] online タグ付きで `make test-online` 実行可能。
- [ ] DOM 構造変化を検知する assert (要素の存在チェック)。

**Blocked by:** T-053, T-054, T-031
**Related specs:** [08-external-services.md §6](./08-external-services.md#6-ブラウザスクレイピング)

---

# 不採用タスク (backlog)

参考のため、不採用となった 55 タスクを記録する。後から必要になったら拾い直す。

## 出力形式

- T-040 (Markdown PDF)

## テンプレート

- T-090 (terminal)
- T-091 (markdown)
- T-092 (markdown-pdf)
- T-093 (community loader)

## プラグイン (GitHub 系不採用 8 個)

- T-047 (lines)
- T-048 (gists)
- T-057 (followup)
- T-058 (discussions)
- T-060 (code) ← プライベートコード漏洩リスクで恒久不採用候補
- T-061 (introduction)
- T-067 (skyline)
- T-069 (support) ← deprecated

## プラグイン (社外 API 系全 10 個)

- T-070 (wakatime)
- T-071 (pagespeed)
- T-072 (posts)
- T-073 (rss)
- T-074 (stackoverflow)
- T-075 (leetcode)
- T-076 (anilist)
- T-077 (music)
- T-078 (steam)
- T-079 (tweets) ← deprecated

## プラグイン (community 系全 9 個)

- T-080〜T-088 (crypto / nightscout / stock / chess / splatoon / fortune / poopmap / screenshot / 16personalities)

## Web 系 (全 10 個)

- T-094, T-095, T-096, T-097, T-098, T-099, T-100, T-101, T-102, T-103

## Action 系

- T-107 (skip detection は T-105 に内包)
- T-112 (gist)
- T-113 (markdown キャッシュ commit)
- T-115 (workflow cleanup)
- T-116 (insights webserver)

## テスト/インフラ

- T-011 (cache) — Web 不採用
- T-123 (e2e web) — Web 不採用
- T-124 (bench) — 後回し
- T-127 (CLI image) — 軽量化はリリース後
- T-129 (action.yml 自動生成) — 手書きで運用
- T-132 (JSON Schema 公開) — 初版で省略

---

# Issue 作成チェックリスト

このドキュメントから issue を起こす際の運用:

- [ ] 各タスクの section 全文を issue 本文にコピー。
- [ ] タイトルは `T-XXX: <title>` 形式。
- [ ] Labels をそのまま付与。
- [ ] Blocked by の T-XXX 番号を GitHub 標準の "linked issues" で連結。
- [ ] GitHub Projects (Beta) に `Backlog → Ready → In Progress → Review → Done` のカラムで配置。
- [ ] M1 から順に Ready 化し、Blocked by が解消された順に着手。
