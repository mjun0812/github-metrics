# Profile README — per-plugin SVG layout guide

This document shows how to wire `github-metrics` in per-plugin mode to
produce a rich GitHub profile README where each section is an independent
SVG file.

## How it works

When `combined: 'no'` (the default for the Action input), the Action runs
once and writes one SVG per enabled plugin into `output_dir` (default:
`./metrics-renders/`).  Each file is named `<plugin>.svg`, e.g.
`header.svg`, `languages.svg`, `stars.svg`.

Because every SVG is independent:

- Removing a plugin removes only that card — no other output is affected.
- Cards can be repositioned freely in the README markup.
- A failed plugin produces a warning in the workflow log but does not block
  the other files from being written.

## Workflow

```yaml
# .github/workflows/metrics.yml
name: Profile metrics
on:
  schedule: [{cron: '0 0 * * *'}]
  workflow_dispatch:

jobs:
  metrics:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: mjun0812/github-metrics@v1
        with:
          user: ${{ github.repository_owner }}
          token: ${{ secrets.METRICS_TOKEN }}
          # Per-plugin output directory (committed to the repository).
          output_dir: ./metrics/
          combined: 'no'
          # Enable the plugins you want.
          plugin_header: 'yes'
          plugin_languages: 'yes'
          plugin_stars: 'yes'
          plugin_activity: 'yes'
          output_action: commit
          committer_branch: main
```

## README embedding

```markdown
<!-- Full-width header card -->
<img src="metrics/header.svg" width="100%">

<!-- Side-by-side cards -->
<img src="metrics/languages.svg" align="left" width="48%">
<img src="metrics/stars.svg" align="right" width="48%">

<div style="clear: both;"></div>

<!-- Full-width activity timeline -->
<img src="metrics/activity.svg" width="100%">
```

## Plugin allowlist (`plugins` input)

To render only a subset of the enabled plugins — useful when you enable
many plugins but want different layout files — pass a comma-separated list:

```yaml
plugins: 'header, languages, stars'
```

## CLI equivalent

```bash
metrics-cli \
  --user octocat \
  --token-env GITHUB_TOKEN \
  --output-dir ./metrics/ \
  --plugin plugin_header=yes \
  --plugin plugin_languages=yes \
  --plugin plugin_stars=yes
```

The `--output-dir` flag defaults to the current directory when omitted.
Use `--combined` to fall back to the classic single-file output.
