# Feature Specification: Wire `repositories.Starred` to `/users/{login}/starred`

**Feature Branch**: `012-rest-data-fetch`

**Created**: 2026-05-22

**Status**: Scope-narrowed (see History below)

**Input**: 011 で 19 plugin の partial-parity 完了後、コードベース調査の結果として `repositories.Starred` のみが placeholder 実装 (Featured のコピー) で残っており本来の挙動 (`/users/{login}/starred` から取得) に置き換える必要がある。`traffic` / `repositories.Random` は M4 で既に完全配線済、`contributors` user/org mode は contract §6 で「meaningful base/head commit range が無いため Skipped」と意図的に documented されているため本 PR スコープ外。

## History

- **Initial scope**: 4 plugin (traffic / contributors user-org / repositories.Starred / repositories.Random) を REST 経由で配線する想定で spec 策定
- **2026-05-22 scope reduction**: 実コードベース調査により以下が判明
  - `traffic.go::Run`: 既に完全配線済 (M4 baseline、6 tests passing)
  - `contributors.go::Run`: user/org mode は contract §6 で意図的に Skipped (commit range 不在のため)
  - `repositories.go::Run::randomSubset`: 既に完全配線済 (Fisher-Yates + seed)
  - `repositories.go::Run::Starred`: line 127 で `res.Starred = featured` の **placeholder** のみ
- → 本 feature は **Starred の real fetch 配線のみ**に縮小

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Starred repositories list reflects user's actual stars (Priority: P1) 🎯 MVP

A user enables `plugin_repositories: yes` + `plugin_repositories_starred: yes`. `Result.Starred` には、Featured のコピーではなく、本人が実際に star した repositories の最新 N 件が反映される。

**Why this priority**: M4 baseline で唯一残った「データ取得の placeholder 実装」を解消する。1 plugin だけのため scope が明確、変更影響範囲も小さい。

**Independent Test**: docker render against `mjun0812` で `plugin_repositories_starred: yes`、`plugin_repositories_limit: 4` 指定 → `data.plugins.repositories.starred` JSON に Featured とは異なる、最近 star した repo 4 件が並ぶ (Featured と `nameWithOwner` が一致しないことで確認可能)。

**Acceptance Scenarios**:

1. **Given** ある user が starred repositories を持つ、**When** action を `plugin_repositories_starred: yes` + `plugin_repositories_limit: 4` で実行、**Then** `Result.Starred` は `/users/{login}/starred?per_page=100&sort=created&direction=desc` の先頭 4 件を `plugins.Repository` shape にマップした list。
2. **Given** `pc.REST` が nil (例: token 不在パス)、**When** Starred が enable されている、**Then** placeholder 経路 (Featured のコピー) に fallback し、`*RetryableError` を `pc.Data.Errors` には追加しない (token 経路の問題は engine 境界で別途扱う)。
3. **Given** `/users/{login}/starred` が 5xx を返す、**When** Starred fetch が失敗、**Then** `Result.Starred = nil` (omitempty で JSON から消える)、`pc.Data.Errors` に `*RetryableError` が 1 件追加、他フィールド (Featured / Pinned / Random) は影響なし。
4. **Given** `pc.Data.User.Login == ""` (engine が user を解決できなかった)、**When** Starred が enable、**Then** ネットワーク呼び出しを行わず、`Result.Starred = []` (空 slice) を返す。
5. **Given** `plugin_repositories_starred: no` (default)、**When** action 実行、**Then** Starred fetch は呼ばれない (M4 baseline 動作維持)。

### Edge Cases

- **Pagination**: 1 ページ (`per_page=100`) のみ取得。`plugin_repositories_limit` は 1..100 範囲想定 (upstream metadata で同制限)。limit > 100 のケースは「100 件まで」で truncate。
- **Repository マッピングの不完全性**: REST `repository` shape のうち、`language` は string、`languages.edges` は無いため `Languages` フィールド (`[]LanguageStat`) は空。`Language` フィールドのみ `{Name: lang, Color: ""}` で populate (color は GraphQL 限定情報のため空)。
- **Rate limit (429 / X-RateLimit-Remaining=0)**: 一般的な HTTP error path に集約。`*RetryableError` を Data.Errors に追加、`Result.Starred = nil` で fallback。
- **Network timeout**: 30s per-request timeout を適用。タイムアウト時は 5xx と同等の handling。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `plugin_repositories_starred` が truthy かつ `pc.REST != nil` の場合、システムは `/users/{login}/starred?per_page=100&sort=created&direction=desc` を取得 MUST。
- **FR-002**: REST レスポンスの各 repository オブジェクトを既存 `plugins.Repository` shape にマップ MUST。マッピング表は data-model.md §E-012-C 参照。
- **FR-003**: 取得結果を `plugin_repositories_limit` (default 8) で truncate MUST。
- **FR-004**: HTTP failure 時は `pc.Data.AppendError` で `*RetryableError` を記録 MUST。`Result.Starred` は nil に set し、Featured / Pinned / Random は影響を受けない MUST。
- **FR-005**: `pc.Data.User.Login == ""` の時はネットワーク呼び出しを行わず、空 slice (`[]`) を返す MUST。
- **FR-006**: `pc.REST == nil` の時は M4 baseline の placeholder 動作 (Featured のコピー) を保持 MUST — 後方互換性のため。
- **FR-007**: per-request HTTP timeout は 30s MUST。overall context cancellation は honor MUST。
- **FR-008**: 既存の `traffic` / `repositories.Random` / `repositories.Featured` / `repositories.Pinned` の挙動は無変更 MUST (この PR では touch しない)。

### Key Entities

- **StarredRepo**: `/users/{login}/starred` の各エントリ。既存 `plugins.Repository` struct を再利用、新規 struct 不要。Language は string→`{Name, Color:""}`、Languages (`[]LanguageStat`) は空 slice。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `plugin_repositories_starred: yes` を設定した時、`data.plugins.repositories.starred` の各エントリの `nameWithOwner` が `data.plugins.repositories.featured` のいずれとも異なるケースが少なくとも 1 件存在する (Featured コピーではなく本物の starred であることの証明)。
- **SC-002**: `plugin_repositories_starred: no` (default) で実行した時、`data.plugins.repositories.starred` フィールドが JSON 出力に **存在しない** (omitempty 経由) — M4 baseline 動作維持。
- **SC-003**: REST 5xx を 100% 返す mock 環境下で、`Result.Starred = nil`、`pc.Data.Errors` に 1 件追加、他 plugin の出力は無変更 (golden diff = 0 outside `repositories.starred`)。
- **SC-004**: 既存 19 plugin の golden test が全 green を維持 (`go test ./...` exit 0)。

## Assumptions

- 既存 `internal/githubapi/rest.go` の `(*REST).Get(ctx, path, extra)` を使用 (rest.go:157)。新規ヘルパ追加なし。
- 既存 `xerrors.NewRetryableError` を error wrapping に使用。
- per_page=100 + limit ≤ 100 想定で、pagination ロジックは追加しない (上流 metadata.yml に `plugin_repositories_limit` の max=100 制約あり)。
- `plugin_repositories_starred` は既に `parseInputs` で読まれている (line 70 `starred bool`) ため、input parsing 追加不要。
- 既存 partial (`internal/plugins/repositories/partial.go`) は無変更。Featured と同じ Repository shape を render しているため、Starred の populate が直ちに反映される。
