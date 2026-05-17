# Contract: action.yml inputs schema

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-001

本書はリポジトリルートに新規追加する `action.yml` の inputs 定義 schema を定める。upstream `lowlighter/metrics` の action.yml (1600 行) のうち、本プロジェクトの採用 21 plugin + core inputs に該当する部分のみを subset として生成する。

## 1. 生成方法 (constitution 原則 III + V)

**手書きしない**。`internal/tools/gen-action-yml/main.go` (新規) で `assets/plugins/<slug>/metadata.yml` + `assets/templates/<name>/metadata.yml` を読み取って action.yml を生成する。

```bash
make gen-action-yml   # = go run ./internal/tools/gen-action-yml --output ./action.yml
```

CI / lefthook で `git diff --quiet action.yml` を gate にし、metadata.yml と action.yml の drift を検出する。

## 2. action.yml の構造

```yaml
name: 'github-metrics'
description: 'Generate metrics for your GitHub profile / repository (drop-in replacement for lowlighter/metrics)'
author: 'mjun0812'
branding:
  icon: 'bar-chart-2'
  color: 'blue'

inputs:
  # ===== Core inputs =====
  user:
    description: 'GitHub user / org login. Defaults to the workflow trigger.'
    default: ''
    required: false
  template:
    description: 'Template name (currently only "classic" is shipped).'
    default: 'classic'
    required: false
  token:
    description: 'GitHub token (PAT). Required for real metrics; "MOCKED_TOKEN" for dryrun/mocked.'
    default: ''
    required: false
  # ... (output_action / output_condition / committer_* / config_* / dryrun / retries / etc.)

  # ===== Plugin enable gates (21 entries) =====
  plugin_languages:
    description: 'Display languages statistics'
    default: 'no'
  plugin_languages_indepth:
    description: 'Enable in-depth language analysis (heavy; clones repos)'
    default: 'no'
  # ... 残り plugin_<slug>* 入力

outputs:
  metrics_url:
    description: 'URL of the generated metrics file inside the repository (when output_action committed)'
  metrics_sha:
    description: 'SHA of the rendered SVG (post-render hash)'

runs:
  using: 'docker'
  image: 'docker://ghcr.io/mjun0812/github-metrics:latest'
```

## 3. Inputs 一覧 (採用範囲)

### 3.1 Core inputs (M1-M3 で確立)

| Input | Type | Default | Source |
|-------|------|---------|--------|
| `user` | string | '' (= workflow trigger user) | spec FR-001 |
| `template` | string | `classic` | M2 |
| `token` | token | '' | M1 |
| `output_action` | string | `commit` | spec FR-014 / FR-015 / FR-015b |
| `output_condition` | string | `always` | spec FR-012 / FR-013 |
| `dryrun` | boolean | `no` | spec FR-009 |
| `retries` | number | `3` | spec FR-007 |
| `retries_delay` | number | `300` (ms) | spec FR-007 |
| `committer_branch` | string | '' (= default branch) | spec FR-014 |
| `committer_message` | string | `Auto-generated metrics for run #${run}` | spec |
| `committer_author` | string | `metrics[bot]` | spec |
| `committer_email` | string | `metrics@noreply.localhost` | spec |
| `config_presets` | string | '' | spec FR-006 |
| `config_timezone` | string | `Etc/UTC` | M2 |
| `config_output` | string | `svg` (svg/png/jpeg/json) | M2 / M3 |
| `config_padding` | string | '' | M3 |
| `filename` | string | `github-metrics.*` | spec FR-008 |
| `notice_releases` | boolean | `yes` | spec FR-021 |
| `use_mocked_data` | boolean | `no` | spec FR-011 |
| `github_api_rest` | string | `https://api.github.com` | M1 |
| `github_api_graphql` | string | `https://api.github.com/graphql` | M1 |

### 3.2 Plugin enable gates + per-plugin options (21 entries × N options ≈ 200 inputs)

`assets/plugins/<slug>/metadata.yml` の `inputs:` セクションをすべて拾い、action.yml の `inputs:` にコピー。defaults は metadata.yml の `default:` を踏襲。description も同。

採用 plugin (21):

- P1 (5): `languages`, `activity`, `achievements`, `repositories`, `isocalendar`
- P2 (12): `calendar`, `habits`, `stars`, `people`, `notable`, `contributors`, `reactions`, `projects`, `sponsors`, `sponsorships`, `stargazers`, `traffic`
- P3 (4 slugs / 2 dirs): `topics`, `starlists`, `languages.recent` (= `languages.sections=recently-used`), `languages.indepth` (= `plugin_languages_indepth`)

不採用 19 plugin (FR-018 unadoptedPlugins) は metadata.yml ごと存在しないので自然に generation 対象外。

### 3.3 不採用機能の入力 (省略する)

upstream action.yml にあるが本プロジェクトで省略する入力:

- すべての `plugin_code_*`, `plugin_discussions_*`, `plugin_followup_*`, `plugin_gists_*`, `plugin_introduction_*`, `plugin_licenses_*`, `plugin_lines_*`, `plugin_skyline_*`, `plugin_support_*`, `plugin_anilist_*`, `plugin_leetcode_*`, `plugin_music_*`, `plugin_pagespeed_*`, `plugin_posts_*`, `plugin_rss_*`, `plugin_stackoverflow_*`, `plugin_steam_*`, `plugin_tweets_*`, `plugin_wakatime_*` (= 19 不採用)
- `markdown_*`, `extras_css`, `extras_js` (markdown / extras 採用見送り)
- `output_action='gist'` 周りの gist-specific input (`gist`, `gist_token`)
- `template='community'` 周りの community 専用 input
- `delay`, `clean_workflows`, `setup_community_templates`, `use_prebuilt_image` (MVP 簡略化)

これらの input が `with:` で渡されても **action.yml で受け入れない** → GitHub Actions は warning を出すが action 自体は起動する → `metrics-action` 内で `output_action` 等の値だけ別途検証 (FR-015b)。

## 4. outputs

| Output | Description |
|--------|-------------|
| `metrics_url` | repo 内の SVG/PNG/... のフルパス (`https://github.com/owner/repo/blob/branch/filename`)。dryrun 時は空 |
| `metrics_sha` | rendered output の SHA-256 ハッシュ (`render.Hash` の戻り値)。dryrun でも生成して set |

set 方法は `$GITHUB_OUTPUT` への `key=value\n` append (M6 R-003)。

## 5. runs

```yaml
runs:
  using: 'docker'
  image: 'docker://ghcr.io/mjun0812/github-metrics:latest'
```

- `using: docker` で Docker image を直接起動 (composite shell-script wrapper なし、shell オーバーヘッドゼロ)
- `image:` は `docker://ghcr.io/...` の prefix で GHCR を指す (clarification Q1)
- 同 release pipeline で `:vX.Y.Z` / `:latest` / `:sha-<short>` 3 タグ pushed (clarification Q1)
- ユーザは `uses: mjun0812/github-metrics@latest` 1 行で参照 (action.yml が GitHub に公開されていれば GH が pull する)

## 6. validation gates (CI)

| Gate | Check | Failure action |
|------|-------|----------------|
| action.yml drift | `make gen-action-yml && git diff --quiet action.yml` | metadata.yml を直すか、generation tool 側を修正 |
| 不採用 plugin slug の混入 | `grep -E "plugin_(code|discussions|...)" action.yml` | generation tool が誤って拾った場合の signal |
| inputs 行数 | upstream subset として ~600 行を想定 (1600 → 約 1/3) | 想定の倍以上なら不採用混入チェック |
