<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:

- Active plan: [specs/008-m10-release-distribution/plan.md](specs/008-m10-release-distribution/plan.md)
- Feature spec: [specs/008-m10-release-distribution/spec.md](specs/008-m10-release-distribution/spec.md)
- M9 baseline (completed): [specs/007-m9-test-infrastructure/plan.md](specs/007-m9-test-infrastructure/plan.md)
- M7 baseline (completed): [specs/006-m7-repository-template/plan.md](specs/006-m7-repository-template/plan.md)
- M6 baseline (completed): [specs/005-m6-action-cli/plan.md](specs/005-m6-action-cli/plan.md)
- M4 baseline (completed): [specs/004-m4-github-plugins/plan.md](specs/004-m4-github-plugins/plan.md)
- M3 baseline (completed): [specs/003-chromedp-rendering-pipeline/plan.md](specs/003-chromedp-rendering-pipeline/plan.md)
- M2 baseline (completed): [specs/002-output-classic-json/plan.md](specs/002-output-classic-json/plan.md)
- M1 baseline (completed): [specs/001-project-foundation/plan.md](specs/001-project-foundation/plan.md)
- Project constitution: [.specify/memory/constitution.md](.specify/memory/constitution.md)
<!-- SPECKIT END -->

## Adopted MVP scope (DO NOT DEVIATE)

本プロジェクトは upstream `lowlighter/metrics` の **subset** のみを採用しています。
新しい Phase に着手する前に、必ず以下の source of truth で採用対象を確認してください。

### Source of truth

- **採用機能定義**: [docs/design/15-selection-answer.md](docs/design/15-selection-answer.md)
- **MVP タスク順序**: [docs/design/16-tasks-mvp.md](docs/design/16-tasks-mvp.md)
- ⚠️ **`docs/design/12-tasks.md` は upstream 全機能 (採用外含む)** — Phase 順序の判断には使わないこと

### Adopted phase order

`M1 → M2 → M3 → M4 → M6 → M7 → M9 → M10`

(M1-M4 + M6 + M7 + M9 完了済、現在は M10 spec/plan 段階)

### Skipped phases (実装禁止)

- **M5**: Web インスタンス (chi server / OAuth / insights 等の HTTP 公開機能) — 「運用コストが高く不要」と決定済 (15-selection-answer.md §1: "Action と CLI のみ採用します。Web インスタンスは運用コストが高く、当面は不要と判断しました。")
- **M8**: ソーシャル / 外部 API plugin (anilist / leetcode / chess / steam / music / pagespeed / tweets / stackoverflow / wakatime 等の 19 plugin) — 全数不採用

### Enforcement

- `tests/compliance/compliance_test.go::TestCompliance_M4_AdoptedPlugins` が採用 19 plugin dir を厳密一致で gating
- 同 `TestNoUnadoptedPluginReference` が不採用 plugin slug を production code から検出
- これらが落ちた場合 = 採用外機能が混入した = 即対処
