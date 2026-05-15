# Feature Specification: classic テンプレート + JSON 出力 (M2)

**Feature Branch**: `002-output-classic-json`

**Created**: 2026-05-15

**Status**: Draft

**Input**: User description: "M1 で出来た engine.Compute の戻り値を、ユーザーが実際に消費できる出力フォーマット (JSON および classic SVG) に変換する。T-023〜T-029 を 1 つの `output-classic-json` feature として spec 化。"

## User Scenarios & Testing *(mandatory)*

本機能の「ユーザー」は 2 種類存在する。

- **GitHub Action 利用者 (一次)**: README に貼る画像 URL もしくは JSON を出力結果として消費する。`uses: mjun0812/github-metrics@v1` 経由で classic SVG または `output: json` で JSON を得る。
- **データ消費 downstream (二次)**: JSON 出力を取り込む別ツール (CI / ダッシュボード / Bot)。スキーマ安定性が重要。

### User Story 1 - JSON 出力で full data structure を取得 (Priority: P1)

GitHub Action 利用者が `output: json` を指定すると、`engine.Compute` が組み立てた `Data` 構造体全体を JSON として出力する。スキーマは上流 `lowlighter/metrics` の `metrics.<login>.json` と **完全互換** (キー名、入れ子、型) でなければならない (constitution 原則 II)。循環参照は `"[Circular]"` 文字列で安全に潰す。

**Why this priority**: JSON は機械可読のため、Web UI を実装しない本 MVP においても下流ツールが直接消費できる唯一の output 形式。SVG (classic) と並ぶ最重要 deliverable。spec FR-026 で予約された Compute 出力フェーズの実体化でもある。

**Independent Test**: mocked GraphQL backend で `engine.Compute(Request{Login:"octocat", Format:"json"})` を呼び、戻り値の `Result.Output` を `tests/golden/json/octocat.json` と JSON 比較 (キー集合 + 値 deep-equal)。SVG パスは一切経由しない。

**Acceptance Scenarios**:

1. **Given** `data.User.Login == "octocat"` かつ `data.Computed.Repositories.Count == 250` が populate された状態で **When** `Format: "json"` で `engine.Compute` を呼ぶ **Then** `Result.Output` は `{"user": {"login": "octocat", ...}, "computed": {"repositories": {"count": 250, ...}}, ...}` を含む JSON byte slice、`Result.MIME == "application/json"`。
2. **Given** `data.Plugins["sample"]` が自分自身を参照する循環構造を持つ状態で **When** JSON 出力する **Then** 該当ノードは `"[Circular]"` に置換され、Marshal は panic / エラーを返さず完走する。
3. **Given** `data.Errors` に 2 件のエラーが詰まっている (Die=false 経路) 状態で **When** JSON 出力する **Then** `errors` 配列がトップレベルに含まれ、各エントリは `error` (string) フィールドを持つ。
4. **Given** 上流 `lowlighter/metrics` を `output_action=metrics.json` で実行した出力 (`tests/fixtures/upstream/octocat.json`) を持つ状態で **When** 同じ入力で本実装の JSON 出力をする **Then** 上流が出力するキー集合が本実装 JSON 出力に **すべて** 含まれる (キー差分 0 件、値は型・shape を比較)。

### User Story 2 - classic テンプレートで SVG ベース DOM を生成 (Priority: P1)

GitHub Action 利用者が `template: classic`, `output: svg` を指定すると、上流 `classic` テンプレートと **DOM 構造単位で同等** の SVG が生成される (constitution 原則 II)。バイト一致は目指さない (バージョン文字列 / タイムスタンプ等は差異を許容)。本 spec の範囲では 4 partial (`base.header` / `introduction` stub / `base.activity+community` / `base.repositories`) を実装する。

**Why this priority**: README バッジ運用の主要な出力 path。JSON と並んでこの spec の 2 つ目の deliverable。M1 では `template.Run` が no-op だった部分を実体化する。

**Independent Test**: mocked GraphQL + 既存 `engine.Compute` 経由で `Format: "svg"`, `Template: "classic"` を呼び、出力 SVG を XML 正規化 (footer の動的部分 mask) して `tests/golden/classic/octocat.svg` と MD5 比較。

**Acceptance Scenarios**:

1. **Given** classic テンプレートが registry に登録された状態で **When** `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼ぶ **Then** 戻り値は `<svg ...><foreignObject>...<div class="items-wrapper">...</div>...</foreignObject></svg>` という DOM を含む string、`Result.MIME == "image/svg+xml"`。
2. **Given** `data.User` が populate されている状態で **When** classic テンプレートが `base.header` partial を実行する **Then** 出力 fragment に `data-section="header"` 属性のあるブロックが含まれ、login / name / avatar URL がエスケープ済みで埋め込まれる。
3. **Given** `data.Plugins["introduction"]` が nil の状態 (M1 では未採用) で **When** introduction partial が実行される **Then** 空文字列が返り、panic / エラーは発生しない。
4. **Given** `data.Computed.Repositories` が populate された状態で **When** `base.repositories` partial が実行される **Then** 出力に `count`, `stargazers`, `forks` の数値が文字列化されたものが含まれ、`format.Format()` のk/m/b/t 短縮表記が使われる。
5. **Given** `base.metadata=true` の状態で **When** classic テンプレートを実行する **Then** SVG 末尾に `<g data-section="metadata">` が含まれ、timezone 名 / version 文字列 / generated 時刻が出力される。

### User Story 3 - Result.Output が呼び出し側に届く (Priority: P2)

`cmd/metrics-action` / `cmd/metrics-cli` (および将来の Web インスタンス) が `engine.Compute` の戻り値を 1 つの byte slice + MIME としてファイル書き出し or HTTP response にそのまま流せる。M1 では `Result.Output` フィールドが未使用だったが、本 spec で populate される。

**Why this priority**: P1 タスク群 (JSON / SVG) を実装すると自然に出来上がる出力経路の薄いラッパ。M6 の `T-109 render + retry` が消費する。

**Independent Test**: 既存 `tests/integration/foundation_test.go` を拡張し、`Format: "json"` で Compute を呼んだ後 `len(res.Output) > 0` と `res.MIME == "application/json"` を assert。

**Acceptance Scenarios**:

1. **Given** Format = "json" で Compute が完走した状態で **When** `Result.Output` を読む **Then** 非空の `[]byte` であり `json.Valid(res.Output)` が true、`Result.MIME == "application/json"`。
2. **Given** Format = "svg", Template = "classic" で完走した状態で **When** `Result.Output` を読む **Then** `[]byte` の先頭が `<svg` で始まる、`Result.MIME == "image/svg+xml"`。
3. **Given** Format = "" (空) の状態で **When** Compute を呼ぶ **Then** デフォルト ("svg" もしくは Template の最初の supported format) で出力する。

### Edge Cases

- **classic テンプレートが repository アカウントで呼ばれた場合**: classic は `supports: [user, organization]`。`Check(account="repository", ...)` は `*InputError` を返し、Compute は短絡する。
- **JSON 出力で巨大な map**: `data.Plugins` が 21 plugin × 数 KB の payload を持つケースでも 200 ms 以内に Marshal 完了 (パフォーマンス予算は SC-003 に記載)。
- **classic テンプレートで `data.User == nil`**: base plugin が失敗していた場合。template.Run はエラー返却せず、`<svg>` 内の partial は空文字列を返して整形済み empty SVG を出力する。
- **JSON 出力で `data.Config.Timezone.Error != nil`**: timezone 解決失敗をユーザーが知れるよう、エラー文字列がトップレベル `errors[]` に追加される。
- **SVG 出力で improper UTF-8** (絵文字を含む user name): XML エンティティ encode + UTF-8 のまま埋め込み、`<![CDATA[]]>` ではなく `&#xE2;` 形式の数値参照を使う (XML 1.0 互換)。
- **base.repositories で Count = 0**: partial は値 "0" を表示し、空ブロックは出さない (上流互換)。

## Requirements *(mandatory)*

### Functional Requirements

#### JSON 出力

- **FR-001**: System MUST `internal/engine/json.go` に `Marshal(data *plugins.Data) ([]byte, error)` を実装し、トップレベルの JSON object のキー集合を上流 `lowlighter/metrics` の `output_action=metrics.json` 出力と一致させる (constitution 原則 II)。
- **FR-002**: System MUST 循環参照を `"[Circular]"` 文字列に置換する cycleDetector を実装し、自己参照を含む任意の `data.Plugins[*]` 値で panic しないこと (T-029 acceptance)。
- **FR-003**: System MUST Go 内部の `set` / `map[K]V` 型を JSON では順序安定な `[]T` / `map[string]any` に正規化し、ハッシュ比較 (golden file) が決定論的に通ること。
- **FR-004**: System MUST `data.Errors` を JSON のトップレベル `errors` 配列として出力し、各エントリは `{"error": "<message>"}` の object とする。
- **FR-005**: System MUST `data.Config.Timezone.Error` が非 nil のとき該当エラーを `errors` に追加すること。

#### classic テンプレート

- **FR-006**: System MUST `internal/templates/classic/classic.go` に `Template` インタフェースの実装を置き、`init()` で registry に登録する。`Name() == "classic"`、`Metadata()` は embed.FS から `assets/templates/classic/metadata.yml` を返す。
- **FR-007**: System MUST `Check(q, account, format)` で `account ∈ {user, organization}` および `format ∈ {svg, png, jpeg, json}` のみ受け入れ、それ以外は適切な typed error (`*UnsupportedFormatError` / `*InputError`) を返す。
- **FR-008**: System MUST classic 用 `image.svg` スケルトン (`<svg>` + `<foreignObject>` + `<div class="items-wrapper">`) を assets/templates/classic/image.svg からロードし、partial 出力を所定の slot に流し込む。バイト同一不要、DOM 同等 (要素 + 属性 + 階層) であれば良い。
- **FR-009**: System MUST 4 partial 関数を `internal/templates/classic/partials/` に実装する: `base_header.go`, `introduction.go` (stub), `base_activity_community.go`, `base_repositories.go`。各 partial は `PartialFunc` 型で `idempotent`、不在キーで panic せず空文字列を返す。
- **FR-010**: System MUST `base.metadata=true` のとき classic テンプレート末尾に `<g data-section="metadata">...</g>` を出力し、`timezone.name` / package version / generated 時刻 (RFC 3339) を埋め込む。
- **FR-011**: System MUST partial の出力に含まれる動的文字列 (login / name / repo 名等) を XML エンティティでエスケープし、`<`/`>`/`&` を `&lt;`/`&gt;`/`&amp;` に変換する。`format.Format()` (k/m/b/t bucketing) を repository count 等で再利用する。

#### Result / engine 配管

- **FR-012**: System MUST `engine.Result` 構造体に `Output []byte` と `MIME string` フィールドを追加 (M1 で予約済みのスペース)。Compute は populate する。
- **FR-013**: System MUST `Request.Format` のディスパッチを engine.go に実装する: `json` なら `engine.json.Marshal` を呼び `Result.MIME = "application/json"`、`svg` なら `Template.Run` の結果を `[]byte` 化し `Result.MIME = "image/svg+xml"`。
- **FR-014**: System MUST `Request.Format` が空文字のとき、`Template.Metadata().Formats[0]` (または `Template == nil` のとき "json") をデフォルトとして使用する。
- **FR-015**: System MUST classic を `assets/templates/classic/_.json` の partial 順 (N-001 で MVP 用に絞った 4 partial) で実行する。`_.json` に列挙されていない partial は呼ばない。

#### テスト

- **FR-016**: System MUST `tests/golden/json/<case>.json` に JSON 出力の期待値を置き、XML 正規化不要 (JSON は `assert.JSONEq` で比較)。`-update` フラグで golden 更新可能。
- **FR-017**: System MUST `tests/golden/classic/<case>.svg` に SVG 出力の期待値を置き、XML 正規化 (whitespace 除去 + 属性 sort) + MD5 比較で diff 検出。footer の動的部分 (timestamps / version) は mask する。
- **FR-018**: System MUST 上流互換性 evidence として、`tests/fixtures/upstream/octocat.json` (上流 `lowlighter/metrics` の出力) と本実装 JSON 出力のキー集合差分が 0 件であることを確認するテストを置く (`tests/compatibility/json_test.go` 等)。

### Key Entities *(include if feature involves data)*

- **engine.Result (拡張)**: M1 で `Data` / `Errors` を持っていた。本 spec で `Output []byte` と `MIME string` を追加。
- **classic.Template**: `Template` interface の実装。registry に `"classic"` で登録。
- **classic partial group**: 4 つの `PartialFunc` (`base.header`, `introduction`, `base.activity+community`, `base.repositories`) + metadata footer fragment。
- **engine/json.Marshal**: トップレベル `Marshal(*plugins.Data) ([]byte, error)` 関数。cycleDetector を内包。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 上流 `lowlighter/metrics` の `output_action=metrics.json` 出力 (`tests/fixtures/upstream/octocat.json`) のキー集合と本実装 JSON 出力のキー集合差分が **0 件** (constitution 原則 II)。
- **SC-002**: classic + SVG 出力が `tests/golden/classic/octocat.svg` と XML 正規化後 MD5 一致 (constitution 原則 II / IV)。
- **SC-003**: `engine.Compute` が classic / json 経路ともに 1 ユーザー分のデータを **2 秒以内** に組み立て + Marshal 完了 (chromedp 抜き、SC-006 同等値)。
- **SC-004**: `internal/engine` および `internal/templates/classic` の unit test coverage が **80% 以上**。
- **SC-005**: 上流の `tests/cases/*.yml` のうち M1 採用機能で再現可能な 3 ケース以上が `config_output=json` で再現できる (キー集合一致)。
- **SC-006**: `Format` 未指定時のデフォルト挙動が `Template.Metadata().Formats[0]` と一致するテストが緑。
- **SC-007**: `lefthook run pre-commit --all-files` が緑、CI 全 7 job が一発で緑 (constitution principle IV)。

## Assumptions

- 上流 `lowlighter/metrics` の出力 JSON は MIT ライセンスの範囲で test fixture として vendor 可能と仮定。実際の `octocat.json` は `./org_repo` から `make sync-fixtures` 相当の手順 (本 spec の T58 系で実装) で取得する。
- classic の `image.svg` スケルトンは上流の `source/templates/classic/image.svg` を sync-assets で取り込んだものを `assets/templates/classic/image.svg` から読む。EJS のテンプレート構文は本実装の Go 側で再解釈 (string substitution + partial dispatch)。
- M1 で stub のまま残した `internal/githubapi/graphql.go` の Repository connection (totalCount のみ) は本 spec の範囲では拡張しない。`base.repositories` partial は **既に populate されている** `Data.Computed.Repositories` を読むだけで、追加 GraphQL query は発行しない。
- 上流の `partials/_.json` は採用していない plugin の partial を多数含むので、M1 で導入した N-001 (MVP 用 `_.json`) と整合させた classic の自前 `_.json` を `assets/templates/classic/partials/_.json` に置く。本 spec の T-023 範囲で `_.json` を新規 commit してもよい。
- 本 spec の SVG 出力は **静的 string concatenation** で組み立てる (`html/template` ではなく `text/template` 風の partial composition)。`html/template` を使うと自動エスケープが意図と合わない (SVG 内に XML 数値参照を埋めるため)。FR-011 で明示する。
- 完了基準は「上記 18 FR + 7 SC が緑」とする。
