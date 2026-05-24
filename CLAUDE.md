## Adopted MVP scope (DO NOT DEVIATE)

本プロジェクトは upstream `lowlighter/metrics` の **subset** のみを採用しています。
新しい Phase に着手する前に、必ず以下の source of truth で採用対象を確認してください。

### Source of truth

- **採用機能定義**: [docs/design/15-selection-answer.md](docs/design/15-selection-answer.md)
- **MVP タスク順序**: [docs/design/16-tasks-mvp.md](docs/design/16-tasks-mvp.md)
- ⚠️ **`docs/design/12-tasks.md` は upstream 全機能 (採用外含む)** — Phase 順序の判断には使わないこと

### Adopted phase order

`M1 → M2 → M3 → M4 → M6 → M7 → M9 → M10`

(全フェーズ完了済 — v1.0.0 として 2026-05-18 にリリース公開)

### Skipped phases (実装禁止)

- **M5**: Web インスタンス (chi server / OAuth / insights 等の HTTP 公開機能) — 「運用コストが高く不要」と決定済 (15-selection-answer.md §1: "Action と CLI のみ採用します。Web インスタンスは運用コストが高く、当面は不要と判断しました。")
- **M8**: ソーシャル / 外部 API plugin (anilist / leetcode / chess / steam / music / pagespeed / tweets / stackoverflow / wakatime 等の 19 plugin) — 全数不採用

### Enforcement

- `tests/compliance/compliance_test.go::TestCompliance_M4_AdoptedPlugins` が採用 19 plugin dir を厳密一致で gating
- 同 `TestNoUnadoptedPluginReference` が不採用 plugin slug を production code から検出
- これらが落ちた場合 = 採用外機能が混入した = 即対処
