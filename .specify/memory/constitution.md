<!--
SYNC IMPACT REPORT
==================
Version change: (initial template) → 1.0.0
Bump rationale: First ratified constitution. Placeholders replaced with concrete principles
                derived from docs/design/* and the user-supplied scope brief.

Modified principles (template slot → final name):
- [PRINCIPLE_1_NAME] → I. 入力互換性 (NON-NEGOTIABLE)
- [PRINCIPLE_2_NAME] → II. 出力契約 (DOM/JSON 単位)
- [PRINCIPLE_3_NAME] → III. スコープ規律 (採用機能のみ実装)
- [PRINCIPLE_4_NAME] → IV. テーブルテスト + Golden File
- [PRINCIPLE_5_NAME] → V. Go 規約と言語ポリシー

Added sections:
- "Technical Constraints" (former [SECTION_2_NAME])
- "Development Workflow" (former [SECTION_3_NAME])
- Governance

Removed sections: none (all template slots filled).

Templates requiring updates:
- ✅ .specify/templates/plan-template.md — "Constitution Check" gate already present;
       new principles must be enumerated by /speckit-plan when this file is consumed.
- ⚠ .specify/templates/spec-template.md — review at next /speckit-specify to ensure
       compatibility section captures input/output互換性 requirements.
- ⚠ .specify/templates/tasks-template.md — review at next /speckit-tasks to ensure
       テーブルテスト/golden file task category is reflected.
- ✅ .specify/templates/commands/* — no agent-specific (CLAUDE-only) references found
       in this repo; nothing to rewrite at this time.

Follow-up TODOs: none.
-->

# github-metrics Constitution

## Core Principles

### I. 入力互換性 (NON-NEGOTIABLE)

`action.yml` の input 名・既定値、および `settings.json` のスキーマキーは、上流である
`lowlighter/metrics` と **完全互換** でなければならない (`docs/design/09-configuration.md`)。
キーのリネーム、既定値の変更、未知キー混入時のハードエラー化は MUST NOT。未対応の機能に
属するキーは、解釈せず黙って素通りさせる (no-op) MUST。`metadata.yml` 由来の input 解釈と
`presets` の合成順序は上流と同一の優先度を維持 MUST。

**Rationale**: 既存ユーザーのワークフロー (`uses: lowlighter/metrics@...`) を 1 行の差し替えで
移行できることが、本移植の存在意義である。入力レイヤで互換性が崩れると移行コストが
発生し、移植価値そのものが失われる。

### II. 出力契約 (DOM/JSON 単位)

出力フォーマットには 2 段階の契約レベルを設ける (`docs/design/00-overview.md §3.4 / §4.3`)。

- **JSON 出力**: 上流と **完全互換** とする。フィールド名・型・null 表現・キー順序のうち、
  上流が保証する範囲は MUST 保持。Schema を破壊する追加フィールドは MUST NOT。
- **SVG / Markdown 出力**: バイト一致は目指さない。**DOM (要素・属性・クラス) 構造単位**で
  上流と同等であること MUST。文字列リテラル (バージョン文字列、生成時刻等) の差分は許容。

DOM 構造差分は golden file テスト (原則 IV) で検出 MUST。

**Rationale**: 既存ユーザーの README へ貼られた `<img>` および JSON を消費する下流ツール
(ダッシュボード、Bot 等) を破壊しないため。一方で SVG をバイト同一に保つことは Go 移植の
利点 (型安全な再構築、最適化) を捨てることになり非現実的である。

### III. スコープ規律 (採用機能のみ実装)

実装対象は `docs/design/15-selection-answer.md` の §6 (採用タスク集合) に列挙された機能の
**みに限定** する。§7 (不採用タスク) に該当する機能 — Web インスタンス、Markdown / PDF
出力、community plugins、ソーシャル/外部 API 系プラグイン、Insights HTML 等 — の
新規コード追加は MUST NOT。スコープ拡張は constitution 改定または採用回答書の改訂を
経て行う MUST。

**Rationale**: 上流は 40+ プラグインと複数モードを抱え保守負荷が高い。MVP の成功条件は
「自分の README で動く採用機能集合が再現できる」ことであり、未採用機能を半端に取り込むと
テスト・互換性検証・依存ライブラリ管理のコストが指数的に増える。

### IV. テーブルテスト + Golden File

すべてのレンダリングおよび整形ロジック (plugin の `Run`、template の組み立て、SVG/JSON
整形、CSS purge、XML format) は以下の 2 種の自動テストでカバー MUST
(`docs/design/10-testing-deployment.md §1`)。

- **テーブルテスト**: 入力 (mocked GraphQL/REST レスポンス, INPUTS) と期待出力の組を
  `[]struct` で列挙し、`t.Run(tc.name, ...)` で網羅。新規ケース追加は edit のみで完結 MUST。
- **Golden file**: 生成 SVG / JSON / Markdown を `testdata/golden/**` に固定し、XML 正規化
  + ハッシュ比較で diff 検出。golden 更新は `-update` フラグ経由で明示的に MUST。

外部 API は `internal/testutil/mocks` の RESTMux / GraphQLMux で差し替え MUST。
`MOCKED_TOKEN` 経路を通らない外部呼び出しがテスト中に発生した場合は panic MUST。

**Rationale**: 出力契約 (原則 II) の DOM 同等性は人間の目視では維持できない。テーブル
テストはケースの追加コストを最小化し、golden file は構造差分を機械的に保証する。両者の
組合せが互換性を持続可能にする唯一の手段である。

### V. Go 規約と言語ポリシー

- **言語**: Go (latest stable; `go.mod` の `go` directive で固定)。サポート対象は最新マイナー
  バージョンのみ。
- **プロジェクト構成**: `cmd/<binary>/main.go` を実行エントリ、再利用不可なロジックは
  `internal/` 配下。公開ライブラリ提供を目的としないため `pkg/` は **作らない** MUST。
- **ドキュメント言語**: 設計書・ユーザードキュメント (`docs/**`, `README.md`) は **日本語**。
- **コードコメント**: ソースコード内のコメント・docstring・identifier は **英語** MUST。
- **ロギング**: 標準 `log/slog` を使用。レベルは `debug/info/warn/error`。
- **依存追加**: 新規外部依存は採用根拠 (代替検討と棄却理由) を PR 説明に MUST 記述。

**Rationale**: 日本語ドキュメントは本プロジェクトの主要読者 (起票者および国内開発者) に
最適化するが、コードは将来 OSS として国際的に読まれる前提で英語に統一する。`cmd/` +
`internal/` の二層は Go コミュニティの de facto standard であり、外部公開 API の表面積を
最小化する。

## Technical Constraints

- **ライセンス**: MIT (上流と同一)。
- **ビルド成果物**: `cmd/metrics-action` (GitHub Action / CLI) を主バイナリとする。Web
  インスタンス用バイナリは原則 III により本構成では生成しない。
- **性能目標** (`docs/design/10-testing-deployment.md §3`): `engine.Compute` (classic + 主要
  10 plugin) < 5 秒 (chromedp 抜き)、`svg.Resize` (chromedp 経由) < 5 秒/レンダリング。
- **配布**: Docker イメージ + クロスコンパイル済みバイナリ (linux/amd64, linux/arm64,
  darwin/arm64, windows/amd64)。`go build -trimpath -ldflags="-s -w -X main.version=..."` 必須。
- **アセット**: `assets/` は `//go:embed` でバイナリに同梱。`git lfs` 依存は MUST NOT。
- **外部依存の選定基準**: 上流が Node 固有ライブラリを使う箇所 (例: `puppeteer` → `chromedp`,
  `linguist-js` → 自前実装または分析サービス委譲) は移植先で機能等価が確認できたものに
  限り採用 MUST。

## Development Workflow

### org_repo の取り扱い

- `./org_repo` は上流 `lowlighter/metrics` の参照コピーであり、**git 履歴に含めない** MUST。
  リポジトリルートの `.gitignore` に `org_repo/` を記載し、`git status` で常に ignored で
  あること MUST。
- `org_repo` 配下のコードを **コピー & ペーストで持ち込まない** MUST。仕様確認や挙動再現
  目的の参照に限定し、移植コードは上記 §V の規約に沿って Go で書き起こす MUST。
- ライセンス遵守: 上流の MIT ライセンス表記は本プロジェクトの `LICENSE` および
  ドキュメント先頭で踏襲 MUST。

### コミット規約

- Conventional Commits 形式を MUST。曖昧な動詞 (「修正」「更新」「対応」のみ) の本文は
  MUST NOT。何を・なぜ変更したかを 1 行目で示し、必要に応じ 3 行目以降で詳述。
- Semantic Versioning 2.0.0 を MUST。Git tag は `vX.Y.Z` 形式。

### Pull Request ゲート

すべての PR は以下を満たさなければ merge MUST NOT。

1. `gofmt` / `go vet` / リンタ (`golangci-lint`) パス。
2. 影響範囲の table test + golden file が green。
3. 入力互換性 (原則 I) を変更する変更は、`action.yml` / `settings.json` / `metadata.yml`
   のスキーマ差分を PR 説明に明記。
4. スコープ外機能 (原則 III) の追加でないこと。例外は constitution 改定 PR が先行する場合のみ。
5. 出力 DOM 構造を変える変更は、golden 更新の意図と影響範囲を PR 説明に記述。

## Governance

- 本 constitution は本プロジェクト内のすべての他の規約・慣行に優先する。CLAUDE.md や
  個別ドキュメントの記述と矛盾する場合は本書を優先 MUST。
- 改定は PR 経由とし、本書冒頭の Sync Impact Report を更新 MUST。改定種別ごとの
  バージョン規則:
  - **MAJOR**: 原則の削除・後方互換を破る再定義。
  - **MINOR**: 原則の追加、または既存原則の運用範囲を実質的に拡張する改訂。
  - **PATCH**: 表現の明確化、誤字、参照ファイル名の追従など非意味的変更。
- 採用機能の範囲を変更する場合は、constitution 改定と併せて
  `docs/design/15-selection-answer.md` を更新 MUST。
- ランタイム開発ガイダンス (技術スタック詳細、shell コマンド、初期化手順) は
  `CLAUDE.md` および `docs/design/**` を参照 MUST。

**Version**: 1.0.0 | **Ratified**: 2026-05-15 | **Last Amended**: 2026-05-15
