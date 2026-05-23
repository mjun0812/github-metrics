# Contract: Plugin doc page template

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-001

Codifies the structure of each `docs/plugins/<slug>.md` page. The
`gen-plugin-docs` tool emits this structure on first generation
and preserves the human-authored zones on re-runs.

## 1. Page skeleton

```markdown
<!-- AUTOGEN_START: title-and-description -->
# Plugin: <slug>

<one-paragraph description from metadata.yml::description>
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![<slug> sample](../examples/plugin-<slug>.svg)

> 上のサンプルは `--user mjun0812` のデータで `<slug>` プラグイン
> のみを有効化してレンダリングした例です。再生成は
> `make docs-examples`。

## このプラグインを使うべきケース

<!-- HUMAN-AUTHORED — kept verbatim across regenerations -->
<plugin-specific prose about when this plugin is useful>

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | required | 型 |
|-------|------|------------|----------|----|
| `plugin_<slug>` | Enable <slug> plugin | `no` | no | boolean |
| `plugin_<slug>_<...>` | ... | ... | no | ... |
| ... (all inputs from metadata.yml) | ... | ... | ... | ... |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_<slug>: yes
    # plugin_<slug>_<...>: ...
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --dryrun --filename - \
  --plugin_<slug> yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- HUMAN-AUTHORED -->
<plugin-specific pitfalls, scope requirements, sub-mode notes>

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`docs/design/15-selection-answer.md`](../design/15-selection-answer.md) — 採用根拠
- [`assets/plugins/<slug>/metadata.yml`](../../assets/plugins/<slug>/metadata.yml) — upstream metadata
```

## 2. AUTOGEN marker semantics

| Marker name | Source | Updated on |
|-------------|--------|------------|
| `title-and-description` | `metadata.yml::name` + `description` (first paragraph) | every `gen-plugin-docs` run |
| `config-table` | `metadata.yml::inputs` (all entries) | every `gen-plugin-docs` run |
| `usage-snippet` | template hardcoded; `<slug>` substituted | every `gen-plugin-docs` run |
| `plugins-gallery` (README only) | enumeration of all 19 plugins | every `gen-plugin-docs` run |

Human-authored zones (everything outside markers) are read-back
during regeneration and re-written byte-identical. The tool's
unit tests verify: (a) markers exist and pair correctly; (b)
re-running the tool on an already-generated file produces no
diff when metadata.yml is unchanged.

## 3. First-generation behavior

On the **first** invocation for a slug (no existing
`docs/plugins/<slug>.md`), the tool emits the full skeleton with
placeholders inside the human-authored zones:

```markdown
## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリ
で価値を持つか、どんな入力データに依存するか、を書いてください。 -->

...

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグイン
との相互作用などを書いてください。 -->
```

The maintainer fills in the `TODO` blocks before committing.
Subsequent re-runs preserve those edits.

## 4. Required sections enforced by FR-004

A doc-page parity test in `gen-plugin-docs/main_test.go` validates:

- The 3 AUTOGEN sections are present with correct names.
- A `## サンプル出力` heading exists.
- The sample image path matches `../examples/plugin-<slug>.svg`.
- An Action-mode usage snippet appears under the
  `usage-snippet` marker.

Missing sections fail the test with a clear pointer.

## 5. Output language

- Headings: 日本語 (project doc convention per constitution V).
- Code blocks: as-is (English keywords like `uses:`, `with:`).
- TODO placeholders: 日本語 instructions for the maintainer.

This mirrors `docs/migration-to-go.md` (M10 baseline) style.
