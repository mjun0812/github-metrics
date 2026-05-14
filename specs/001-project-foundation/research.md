# Phase 0 Research: プロジェクト土台 (M1 19 タスク)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は spec の Assumptions と Technical Context に含まれる未決定項目を解決する。各決定は **Decision / Rationale / Alternatives considered** の三項目で記述する。

## R-001: Module path

- **Decision**: `github.com/mjun0812/github-metrics` を採用する (Clarifications 2026-05-15 で作者 `mjun0812` から直接確定)。`go.mod` の `module` directive にそのまま書き、すべての import path のルートとする。
- **Rationale**: 作者 user id がユーザー本人から直接指示されたため、placeholder 運用は不要。確定値で初日から CI / レビューを回せる。Action 利用者向けの参照 (`uses: mjun0812/github-metrics@v1`) もこの module path と一致するため、配布 URL とコード URL がぶれない。
- **Alternatives considered**:
  - `github.com/USER/github-metrics` を `sed` 置換用 placeholder として残す → 確定値が判明したため不要。却下。
  - `gh.example/github-metrics` のような non-resolving 名前を使う → 公開予定なので不適切。却下。
  - 上流と同じ `github.com/lowlighter/metrics` を踏襲 → リポジトリ実体と異なり、`pkg.go.dev` 等の参照が壊れる。却下。

## R-002: HTTP リトライライブラリ

- **Decision**: `github.com/hashicorp/go-retryablehttp` を採用する。
- **Rationale**: 5xx / 429 / network エラーの指数バックオフ + `Retry-After` ヘッダ尊重 + drop-in `*http.Client` 互換という要件が揃っている。本プロジェクト要件 (FR-013) を満たし、コミュニティで十分メンテされている。User-Agent の差し替えも `Logger` interface 越しに容易。
- **Alternatives considered**:
  - 自作リトライラッパ → リトライポリシーは枯れたコードなので新規実装は損。テストコストが見合わない。却下。
  - `cenkalti/backoff/v4` + 手動ラッパ → 低層のバックオフライブラリ。`http.Client` 統合が手作業で、結局 retryablehttp 相当のラッパが必要。却下。

## R-003: GraphQL クライアント

- **Decision**: `github.com/Khan/genqlient` を採用し、クエリは `assets/plugins/base/queries/*.graphql` を `genqlient.yaml` 経由でビルド時生成する。
- **Rationale**: 上流が `.graphql` ファイル群でクエリを管理しているため、genqlient の "query file → typed Go function" 変換が migration 形態にもっとも素直。Operation 名ごとに mock fixture を切り替える設計 (T-119) も親和性が高い。
- **Alternatives considered**:
  - `shurcooL/graphql` (struct タグ駆動) → 上流の `.graphql` ファイルを別表現に書き換える必要が出るため、メンテ二重化を招く。却下。
  - `machinebox/graphql` → メンテが停滞気味。型生成なし。却下。
  - 生 HTTP + 手書き request struct → 採用 21 plugin で計 30+ クエリを抱えるため非現実的。却下。

## R-004: YAML パーサ

- **Decision**: `gopkg.in/yaml.v3` を採用する。
- **Rationale**: 上流の `metadata.yml` および `presets/*.yml` が `^!ref` / multi-document 等の YAML 1.2 機能を使う可能性があり、`v3` の API が安全側に倒れている。`Decode(any)` と `Node` ベース両方で対応可能。
- **Alternatives considered**:
  - `gopkg.in/yaml.v2` → quotedString の扱いに既知バグあり。却下。
  - `goccy/go-yaml` → 性能優位だが、上流 yaml 機能との互換性に未検証エッジケースが多い。M1 段階の検証コストを許容できない。M9 で benchmark してから再検討する余地は残す。

## R-005: `settings.json` の `//` キー処理

- **Decision**: `encoding/json` の `json.RawMessage` を使い、トップレベルおよび任意のネストで `key` が `//` で始まるエントリを再帰的に枝刈りしてから `json.Unmarshal` する。`internal/config/settings.go` に `stripCommentKeys(raw []byte) ([]byte, error)` を実装する。
- **Rationale**: `tidwall/gjson` 系のサードパーティ依存を増やすより、stdlib のみで完結する方が保守性が高い。`//` キーはあくまでコメント目的なので削除のみで十分。値の中身は再パースせず、ネスト走査のみで完結する。
- **Alternatives considered**:
  - `tidwall/gjson` で `Iterator` を回す → 追加依存。却下。
  - JSON5 ライブラリ (`Tnze/jsonc`) で本格コメント処理 → 上流は `//` キーをコメントとして使っているのであって、JSON5 構文は使っていない。却下。
  - 上流踏襲の `json.Unmarshal` で二重キー後勝ち解釈 → Go の `encoding/json` は重複キーを最後の値で上書きするため、`//` プレフィックス特化ではなく前段 strip が望ましい。

## R-006: ロギング既定形式

- **Decision**: `internal/logger` は **既定 = JSON** (production)、`--log-format text` で text ハンドラへ切り替える。`debug` レベルは `INPUT_DEBUG=true` または `--debug` で有効化。
- **Rationale**: GitHub Actions のログは複数行 JSON でも視認できる + 後段で構造化ログとして CI 集計に流せる。テスト時は `slog.NewTextHandler(io.Discard, ...)` を `t.Cleanup` で差し替える運用にする。
- **Alternatives considered**:
  - 既定 = text → CI 集計時にパースが弱い。却下。
  - `zerolog` / `zap` → 標準 `log/slog` で十分。サードパーティ依存削減を優先。constitution 原則 V とも整合。

## R-007: テストモッキング

- **Decision**: stdlib + `testify/assert` のみを採用し、`pytest-mock` のような mocker は **使わない** (本プロジェクトは Python ではない)。`httptest.NewServer` + 手書き fake が基本。`internal/githubapi/testhelper.go` に M1 最小スタブを置き、M9 (T-118/T-119) で `internal/testutil/mocks/{rest,graphql}.go` に full 実装へ昇格する。
- **Rationale**: ユーザー global instructions では Python に対し pytest-mock を要求しているが、本プロジェクトは Go。Go コミュニティでは fake 手書き + httptest が de facto。`gomock` は interface ベースの大規模 mocking が必要になったタイミングで導入を再検討する。
- **Alternatives considered**:
  - `gomock` (`go.uber.org/mock`) → M1 段階では interface が少なく overkill。M4 (21 plugin) で再評価。
  - `dnaeon/go-vcr` (HTTP record/replay) → 上流の faker ランダムデータと相性が悪い。決定論を担保しづらい。却下。

## R-008: `golangci-lint` ルール集合

- **Decision**: `.golangci.yml` で以下を有効化する: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `revive`, `gofumpt`, `gosec`, `nilerr`, `prealloc`, `unparam`。`exhaustive` / `exhaustruct` / `wrapcheck` は誤検知が多いため M1 段階では disable し、M9 で再検討。
- **Rationale**: constitution 原則 V「依存追加は採用根拠 PR 説明必須」と整合させ、コアセットを固定。CI を 10 分以内に収めるため `timeout: 10m`。`gosec` はトークン取り扱いに照らして必須 (FR-012)。
- **Alternatives considered**:
  - `golangci-lint --preset bugs --preset performance` のみ → スタイル系が抜けて gofumpt 統一が崩れる。却下。
  - 全有効化 → CI 時間と false positive が膨らむ。却下。

## R-009: `embed.FS` の対象と更新フロー

- **Decision**: `//go:embed assets/*` で同梱。`assets/octicons/data.json` と `assets/twemoji/index.json` は **生成物** であり、`internal/tools/gen-octicons/main.go` と `internal/tools/gen-twemoji/main.go` を `go run ./internal/tools/...` 経由で再生成する (`make gen` ターゲット)。`assets/plugins/**` および `assets/templates/**` は `./org_repo/source/...` から `scripts/sync-assets.sh` で取得する。
- **Rationale**: 上流の構造を踏襲しつつ、Go 側でビルド時 fetch を不要にする。`go.mod` 配布物を完全自己完結に保つことで、Docker レイヤキャッシュも効きやすくなる。`sync-assets.sh` を介することで、上流から手動コピーした証跡を残し、constitution Development Workflow 「コピペ禁止」の運用を担保する (スクリプトは upstream version pin + checksum 検証を行う)。
- **Alternatives considered**:
  - `git submodule` で上流を参照 → 履歴を引き継いでしまい、constitution 「org_repo は git 履歴に含めない」と矛盾。却下。
  - 起動時 HTTP fetch → エアギャップ環境で動かない。却下。
  - 上流 npm tarball を CI で取得 → CI 障害耐性が下がる。却下。

## R-010: CI ランナー OS

- **Decision**: `ubuntu-latest` を primary、`macos-latest` を smoke matrix (build + unit test のみ) として併走する。`windows-latest` は M10 で release バイナリ生成時にのみ動かす。
- **Rationale**: 採用ユーザー (GitHub Action 利用者) は実質 Linux runner で実行する。macOS の smoke で cross-OS path 等の地雷を早期検出。Windows は配布バイナリのビルド検証のみで十分。
- **Alternatives considered**:
  - Linux 単一 → cross-OS リグレッションが release 時にしか検出されない。却下。
  - 3 OS フル matrix → CI 時間 3 倍化。投資対効果が悪い。却下。

## R-011: `errors` 型と wrap 戦略

- **Decision**: `internal/errors` で 5 種の sentinel-bearing 型 (`InputError`, `NotFoundError`, `ForbiddenError`, `UnsupportedFormatError`, `RetryableError`) を struct として定義し、`Unwrap()` で内側エラーを公開する。コンストラクタは `errors.NewInputError(field string, cause error)` 等の signature とする。
- **Rationale**: `errors.Is` / `errors.As` の両方を満たし、原因チェーンを保てる。Sentinel 値だけだとフィールドコンテキスト (例: どの input か) を失う。
- **Alternatives considered**:
  - `errors.New("input error")` の sentinel 群 → context を失う。却下。
  - `cockroachdb/errors` 等の高機能ライブラリ → 過剰。標準 `errors` で十分。

## R-012: Race detector の CI 適用

- **Decision**: `make test` は通常実行、別ジョブ `make test-race` で `go test -race ./...` を必須化する。`internal/githubapi/rate.go` (FR-016) と `internal/plugins/core/run_plugins.go` (FR-025) が並行アクセスの主候補。
- **Rationale**: race detector は重いので全テストに常時付与すると CI 時間が伸びる。並行コードを書いた直後の安全網として独立ジョブで実行する。
- **Alternatives considered**:
  - 全テストで `-race` 常時 → CI 7 分目標 (SC-001) に抵触するおそれ。却下。
  - 並行コードだけ build tag で囲う → maintenance burden 増。却下。

## R-013: 出力 (`/renders/`) 書き出しの段階

- **Decision**: M1 段階では出力書き出しのコードを追加しない。`engine.Compute` の戻り値型に `Output []byte, MIME string` フィールドだけ用意し、本 byte 列は M2 の T-029 (JSON marshaller) が populate する。
- **Rationale**: スコープ規律 (原則 III) と user story 5 の Independent Test 範囲 (internal data 構造体のみ) と整合。M1 PR で永続化処理が紛れ込むと テスト責務が肥大化する。
- **Alternatives considered**:
  - M1 で stdout に internal `data` を fmt.Print する → デバッグ用途と本番出力の境界が曖昧になる。却下。
