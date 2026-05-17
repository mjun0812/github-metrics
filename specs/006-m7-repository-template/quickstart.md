# Quickstart: M7 — repository template

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md)

End-to-end walkthrough for trying the new `repository` template
locally + wiring it into a GitHub Actions workflow.

## 1. Prerequisites

- Go 1.26+ installed (or use the published Docker image)
- A GitHub Personal Access Token with at least `repo` scope (set as
  `GITHUB_TOKEN` env var)
- The target repository identifier — `<owner>/<name>` — that you want
  to render metrics for. Public repos work without special permissions
  beyond the token scope above; private repos require the token's
  owner to have read access

## 2. Build from source

```sh
git clone https://github.com/mjun0812/github-metrics.git
cd github-metrics
git checkout 006-repository-template     # this M7 branch
make build
ls -lh bin/metrics-action               # confirm the binary is present
```

## 3. CLI mode — local preview

### 3.1 Minimum SVG output to stdout

```sh
bin/metrics-action \
  --user octocat \
  --repo hello-world \
  --template repository \
  --token-env GITHUB_TOKEN \
  --output svg \
  --dryrun \
  --filename -
```

Expected: a valid SVG streams to stdout within ~3 seconds (real
GitHub API; faster with mocked deps). Pipe through `xmllint --format -`
for a readable view.

### 3.2 JSON output for downstream tooling

```sh
bin/metrics-action \
  --user octocat \
  --repo hello-world \
  --template repository \
  --token-env GITHUB_TOKEN \
  --output json \
  --dryrun \
  --filename - | jq '.data.repo'
```

Expected output (truncated):

```json
{
  "owner": "octocat",
  "name": "hello-world",
  "name_with_owner": "octocat/hello-world",
  "stargazers": 1234,
  "forks": 567,
  "default_branch": "master",
  "primary_language": { "name": "...", "color": "..." },
  "activity": { "recent_commits": 12, "open_issues": 3, "open_pull_requests": 1 }
}
```

### 3.3 PNG output to a file

```sh
bin/metrics-action \
  --user octocat \
  --repo hello-world \
  --template repository \
  --token-env GITHUB_TOKEN \
  --output png \
  --dryrun \
  --filename /tmp/repo-metrics.png
file /tmp/repo-metrics.png   # → PNG image data, ...
```

## 4. CLI mode — YAML config

Create `metrics-inputs.yaml`:

```yaml
user: octocat
repo: hello-world
template: repository
output: svg
filename: github-metrics.svg
dryrun: false
output_action: commit
output_condition: data-changed
committer:
  branch: ''         # defaults to repo default
  message: 'Update repository metrics'
plugins:
  languages: true
  contributors: true
  activity: true
  stargazers: true
```

Invoke:

```sh
bin/metrics-action --config metrics-inputs.yaml --token-env GITHUB_TOKEN
```

## 5. Docker

```sh
docker run --rm \
  -v "$PWD/out:/renders" \
  -w /renders \
  -e GITHUB_TOKEN \
  ghcr.io/mjun0812/github-metrics:vX.Y.Z \
  --user octocat \
  --repo hello-world \
  --template repository \
  --token-env GITHUB_TOKEN \
  --output svg \
  --filename github-metrics.svg
ls out/github-metrics.svg
```

## 6. GitHub Actions

### 6.1 Workflow snippet

`.github/workflows/repo-metrics.yml`:

```yaml
name: Repo metrics
on:
  schedule: [{cron: '0 0 * * *'}]
  workflow_dispatch:

jobs:
  metrics:
    runs-on: ubuntu-latest
    permissions:
      contents: write       # commit the rendered SVG back to this repo
    steps:
      - uses: mjun0812/github-metrics@vX.Y.Z
        with:
          user: ${{ github.repository_owner }}
          repo: ${{ github.event.repository.name }}
          template: repository
          token: ${{ secrets.METRICS_TOKEN }}
          plugin_languages: 'yes'
          plugin_contributors: 'yes'
          plugin_activity: 'yes'
          output_action: commit
          output_condition: data-changed
          committer_branch: main
          filename: 'docs/repo-metrics.svg'
```

The workflow:

1. Runs nightly + on manual dispatch
2. Targets the host repository (`<owner>/<name>` resolved from
   `github.repository_owner` + `github.event.repository.name`)
3. Renders an SVG via the repository template
4. Commits the SVG to `docs/repo-metrics.svg` on `main`, only when
   the rendered output's M3 hash differs from the existing file
   (per M6 FR-013)

### 6.2 README embed

After the workflow runs, embed the rendered SVG in the repository's
README:

```markdown
![Repository metrics](./docs/repo-metrics.svg)
```

## 7. Validation matrix

These scenarios should all complete without error and exit 0:

| Scenario                                                         | Command                                                                                       | Expected outcome                       |
|------------------------------------------------------------------|----------------------------------------------------------------------------------------------|---------------------------------------|
| SVG to stdout (dryrun)                                           | `metrics-action --user u --repo r --template repository --output svg --dryrun --filename -`   | `<svg>` on stdout, ~3s              |
| JSON shape includes `data.repo`                                  | `metrics-action --user u --repo r --template repository --output json --dryrun --filename -`  | `data.repo.name == "r"`             |
| PNG file is a valid PNG                                          | `--output png --filename /tmp/x.png`                                                          | `file /tmp/x.png` → PNG image data    |
| Classic template + repo input is ignored (backward compat FR-007)| `--user u --repo ignored --template classic --output json --dryrun --filename -`             | classic-shape JSON (no `data.repo`)  |

These should fail fast with exit 1 + clear error within 5 seconds:

| Scenario                                                         | Expected error                                                                                |
|------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| Repository template without `--repo`                             | `template 'repository' requires --repo / INPUT_REPO` (SC-003)                                |
| Repository template with unknown repo (`--repo nonexistent`)     | `repository "u/nonexistent" not found` (FR-008 non-retryable)                                |
| Token lacks `repo` scope on a private target                     | `token is missing some scopes; affected plugins will skip` (M6 warning, continues with partial result) |

## 8. Troubleshooting

- **`template 'repository' requires --repo / INPUT_REPO`**: pass
  `--repo <name>` (CLI) or `INPUT_REPO=<name>` (Action) or `repo: <name>`
  (YAML config).
- **`repository "u/r" not found`**: verify `<owner>/<name>` exists and
  the token has read access. The action does NOT retry this error
  (FR-008) — it's a configuration mismatch, not a transient failure.
- **Output looks identical to classic template**: confirm `template:
  repository` (not `template: classic`) and that `repo` is set. With
  classic + `repo`, the repo value is silently ignored (FR-007).
- **PNG / JPEG truncated or empty**: the M3 chromedp render needs a
  Chromium binary. On macOS set `METRICS_CHROME_PATH=$(which chromium
  || echo "/Applications/Google Chrome.app/Contents/MacOS/Google
  Chrome")`. Inside the published Docker image chromium is preinstalled.
