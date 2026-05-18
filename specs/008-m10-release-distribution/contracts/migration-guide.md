# Contract: Migration guide

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-004

`docs/migration-to-go.md` — the upstream → go-port migration guide
shipped with v1.0.0.

## 1. Language and length

- **Language**: Japanese (constitution V).
- **Length budget**: ≤ 250 lines of Markdown.
- **Reading-time target**: ≤ 10 minutes end-to-end for a reader
  who is already running upstream `lowlighter/metrics` in
  production (SC-005).

## 2. Required sections (6)

### 2.1 概要 (Overview)

- 1 paragraph stating: this project is a Go port of
  `lowlighter/metrics`, scoped to the 21 adopted plugins + 2
  templates from `docs/design/15-selection-answer.md`.
- 1 paragraph stating the audience: existing upstream users who
  want a faster / lighter / Action-native runtime *for the adopted
  subset*. Not for users who depend on the unported plugins.
- 1 sentence linking to the constitution principles I (input
  compat) + II (DOM-equivalent output).

### 2.2 採用機能一覧 (Adopted feature matrix)

Table format:

| 種別 | 名前 | upstream slug | 注記 |
|------|------|---------------|------|
| Plugin | base | base | 必須 (core) |
| Plugin | core | core | 必須 (core) |
| Plugin | languages | languages | indepth サブモード対応 |
| ... | ... | ... | ... |
| Template | classic | classic | 21 partials filtered to adopted set |
| Template | repository | repository | M7 |
| Output format | svg | --output svg | M1 default |
| Output format | json | --output json | M2 |
| Output format | png | --output png | M3 chromedp |
| Output format | jpeg | --output jpeg | M3 chromedp |

The 21 plugins must be enumerated explicitly. The two templates
must be enumerated explicitly. Source links: each row references
the `docs/design/15-selection-answer.md` §3 row that justifies it.

### 2.3 未対応機能一覧 (Unported feature matrix)

Mirror table format:

| 種別 | 名前 | 不採用理由 | upstream への参照 |
|------|------|------------|-------------------|
| Runtime | Web インスタンス (M5) | 運用コスト過大 | `docs/design/15-selection-answer.md` §1 |
| Template | terminal | low 互換性価値 | `docs/design/15-selection-answer.md` §3 |
| Template | markdown | PDF/Markdown 出力と一体 | `docs/design/15-selection-answer.md` §3 |
| Template | community | community 拡張機構未採用 | `docs/design/15-selection-answer.md` §3 |
| Plugin | anilist | 外部 API 系 (M8) | `docs/design/15-selection-answer.md` §2 |
| Plugin | chess | 外部 API 系 (M8) | `docs/design/15-selection-answer.md` §2 |
| ... (19 social plugins total) | ... | ... | ... |
| Output | pdf | upstream は Puppeteer; chromedp 経由は複雑度割に合わず | `docs/design/15-selection-answer.md` §4 |
| Output | markdown | community plugin と一体 | `docs/design/15-selection-answer.md` §4 |
| Feature | insights HTML | M5 web 一体 | `docs/design/15-selection-answer.md` §1 |

All 19 M8 social plugins must be listed (`anilist`, `chess`, `chessdotcom`,
`code`, `codeforces`, `discussions`, `followup`, `gists`,
`introduction`, `leetcode`, `lichess`, `music`, `nightscout`,
`pagespeed`, `posts`, `screenshot`, `splatoon`, `stackoverflow`,
`steam`, `tweets`, `wakatime` — verify count against the
selection-answer doc).

### 2.4 入力互換性 (Input compatibility)

Per constitution principle I:

- All adopted `with:` inputs are **drop-in identical** — same key
  names, same default values, same legal value ranges as upstream.
- Inputs gating an unported plugin (e.g., `plugin_anilist: yes`,
  `plugin_anilist_medias: anime`) are **silently accepted and
  no-op**'d — the workflow does not fail, the input just produces
  no output. This is the design intent: users can leave their
  existing workflow file untouched on day 1 of migration and
  remove unported gates incrementally.

Provide a worked example:

```yaml
- uses: mjun0812/github-metrics@v1.0.0
  with:
    user: octocat
    plugin_languages: yes              # adopted → produces output
    plugin_languages_limit: '5'         # adopted → respected
    plugin_anilist: yes                 # unported → silently no-op
    plugin_anilist_medias: anime        # unported → silently no-op
```

Result: the rendered SVG contains the languages panel; no anilist
panel; no warning, no error.

### 2.5 移行手順 (Migration steps)

4 steps:

1. Pin: `uses: lowlighter/metrics@v3.34` → `uses: mjun0812/github-metrics@v1.0.0`.
2. (Optional) drop unported `plugin_*` gates from `with:` to
   reduce workflow file noise. Not required — silent no-op (§2.4)
   means an unedited workflow file still works.
3. Re-run the workflow.
4. Compare output: a) per constitution principle II, JSON output
   is byte-compat; b) SVG output is **DOM-equivalent**, not
   byte-identical — element/attribute/class structure matches but
   dynamic strings (version footer, generation timestamp) differ.
   Use the `xmllint --format` diff if needed.

### 2.6 ロールバック (Rollback)

One-line revert:

```diff
- uses: mjun0812/github-metrics@v1.0.0
+ uses: lowlighter/metrics@v3.34
```

No `with:` migration is required — adopted inputs are drop-in
compatible (§2.4) so the existing input set works against both
sides. Rollback is therefore zero-cost; commit the diff, re-run,
done.

## 3. What MUST NOT appear in the guide

- Promotional language ("faster than", "better than", "superior") —
  the spec frames this as a *port for a subset*, not a competitor.
- Performance benchmarks vs upstream — out of scope for v1.0; can
  be added in a follow-up doc once we have measurements.
- Per-plugin migration details — link to plugin docs / source
  rather than duplicate explanation.
- TODO / FIXME / placeholder text — guide is published, must be
  complete.

## 4. Acceptance review

- One project maintainer reviews + approves.
- A first-time reader (someone who has not seen this project
  before) is asked to answer 3 questions in ≤ 10 minutes:
  1. "Which upstream plugins are unavailable in this go port?"
  2. "Are my existing `with:` inputs still recognized?"
  3. "How do I roll back to lowlighter/metrics?"

If all 3 are answered correctly → SC-005 passes.

## 5. Source-of-truth references

| Topic | Source doc |
|-------|-----------|
| Adopted plugin / template set | `docs/design/15-selection-answer.md` (especially §2 + §3) |
| Input compatibility semantics | `.specify/memory/constitution.md` principle I |
| Output compatibility semantics | `.specify/memory/constitution.md` principle II |
| Adopted phase order | `CLAUDE.md` "Adopted phase order" + `docs/design/16-tasks-mvp.md` |
