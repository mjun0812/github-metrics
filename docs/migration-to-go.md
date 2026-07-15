# Migration Guide from lowlighter/metrics to github-metrics (Go port)

**Target version**: `mjun0812/github-metrics` latest
**Last updated**: 2026-05-24

## 1. Overview

This project is a Go port of [`lowlighter/metrics`](https://github.com/lowlighter/metrics).
Rather than covering all `lowlighter/metrics` features, it provides a subset focused on
the commonly used **21 plugins + 2 templates + 4 output formats**.

**Intended audience for this guide**: Users currently running
`lowlighter/metrics` who are considering migrating to the Go port.

**Key premises on migration cost**:

- **Input compatibility**: The `with:` input names, default values, and
  types for supported features are fully compatible with `lowlighter/metrics` (you only
  need to replace the `uses:` line for it to work).
- **Output compatibility**: JSON is byte-compatible with `lowlighter/metrics`, and SVG
  is equivalent to `lowlighter/metrics` at the **DOM structure level** (differences in
  version strings and generation timestamps are allowed).

## 2. Supported features

### 2.1 Plugins (21)

All plugins work with just a GitHub API token and regular HTTP fetches.
`topics` / `starlists` scrape pages using Headless Chromium in `lowlighter/metrics`, but
in this port they have been replaced with HTML parsing via goquery, so no
browser is required.

| Name         | `lowlighter/metrics` slug | Notes                                                |
| ------------ | ------------------------- | ---------------------------------------------------- |
| base         | base                      | Basic profile information (internal use)             |
| core         | core                      | Config injection + parallel execution (internal use) |
| languages    | languages                 | Supports `recent` / `indepth` submodes               |
| activity     | activity                  |                                                      |
| achievements | achievements              |                                                      |
| repositories | repositories              | Featured / Pinned / Starred / Random                 |
| isocalendar  | isocalendar               | 3D isometric calendar                                |
| calendar     | calendar                  | Multi-year calendar                                  |
| habits       | habits                    | Day-of-week / time-of-day trends                     |
| stars        | stars                     | Recently starred repositories                        |
| people       | people                    | Followers / following                                |
| notable      | notable                   |                                                      |
| contributors | contributors              | For the repository template                          |
| reactions    | reactions                 | Reaction aggregation                                 |
| projects     | projects                  | GitHub Projects (requires `read:project`)            |
| sponsors     | sponsors                  | (requires `read:user` / `read:org`)                  |
| sponsorships | sponsorships              | (requires `read:user` / `read:org`)                  |
| stargazers   | stargazers                | Cumulative star chart                                |
| traffic      | traffic                   | View counts (requires `repo`)                        |
| topics       | topics                    | HTML scraping (goquery)                              |
| starlists    | starlists                 | HTML scraping (goquery)                              |

### 2.2 Templates (2)

| Name       | Notes                                                            |
| ---------- | ---------------------------------------------------------------- |
| classic    | Default template for users / organizations                       |
| repository | Metrics for a single repository (`--user <owner> --repo <name>`) |

### 2.3 Output formats (4)

| Format | CLI flag        | Notes                                     |
| ------ | --------------- | ----------------------------------------- |
| SVG    | `--output svg`  | Default                                   |
| JSON   | `--output json` | Byte-compatible with `lowlighter/metrics` |
| PNG    | `--output png`  | Rasterizes the native SVG with resvg      |
| JPEG   | `--output jpeg` | Re-encodes the resvg PNG to JPEG in Go    |

> **No browser is required for rendering.**
> `lowlighter/metrics` uses Headless Chromium (puppeteer) to measure the final SVG
> height and to produce PNG/JPEG output, but this port computes the height
> using Go-side font metrics and rasterizes PNG/JPEG with
> [resvg](https://github.com/linebender/resvg). As a result, chromium has
> been removed from the Docker image, structurally eliminating the class of
> failures where a distro package update breaks browser startup (background
> in [#409](https://github.com/mjun0812/github-metrics/issues/409)).

## 3. Unsupported features

The following are **not implemented** in this Go port. If a feature you use
in `lowlighter/metrics` falls under this list, migration is not recommended.

### 3.1 Runtime / Mode

| Type | Name                  | Reason for exclusion                                           |
| ---- | --------------------- | -------------------------------------------------------------- |
| Mode | Web instance          | Operational cost too high (only Action / CLI are supported)    |
| Mode | OAuth / Insights HTML | Predicated on the web instance (excluded along with the above) |

### 3.2 Templates (community-based)

| Name      | Reason for exclusion                       |
| --------- | ------------------------------------------ |
| terminal  | Low compatibility value, niche use case    |
| markdown  | Tightly coupled with Markdown / PDF output |
| community | Community extension mechanism not adopted  |

### 3.3 GitHub data plugins (6)

| slug        | Reason for exclusion                                   |
| ----------- | ------------------------------------------------------ |
| lines       | Heavy load due to extensive use of REST stats          |
| gists       | Low priority                                           |
| followup    | Low priority                                           |
| discussions | Low priority                                           |
| skyline     | 3D city / skyline -- heavy, low priority               |
| support     | Already discontinued `lowlighter/metrics` (deprecated) |

### 3.4 Community extension plugins (3)

| slug         | Reason for exclusion                  |
| ------------ | ------------------------------------- |
| code         | Via the community extension mechanism |
| introduction | Via the community extension mechanism |
| licenses     | Via the community extension mechanism |

### 3.5 Social / external API plugins (19)

| slug            | Size | Reason for exclusion |
| --------------- | ---- | -------------------- |
| anilist         | S    | External API         |
| chess           | -    | Community            |
| crypto          | -    | Community            |
| fortune         | -    | Community            |
| leetcode        | S    | External API         |
| music           | L    | Complex OAuth        |
| nightscout      | -    | Community            |
| pagespeed       | S    | External API         |
| poopmap         | -    | Community            |
| posts           | S    | External API         |
| rss             | S    | Arbitrary URL        |
| screenshot      | -    | Community            |
| splatoon        | -    | Community            |
| stackoverflow   | S    | External API         |
| steam           | M    | External API         |
| stock           | -    | Community            |
| tweets          | -    | Deprecated           |
| wakatime        | M    | External API         |
| 16personalities | -    | Community            |

### 3.6 Output formats

| Name     | Reason for exclusion                                                                            |
| -------- | ----------------------------------------------------------------------------------------------- |
| pdf      | `lowlighter/metrics` renders via Puppeteer (browser) -- out of scope for this browser-free port |
| markdown | Tightly coupled with the community template                                                     |

## 4. Input compatibility

- **Supported inputs** (plugin / template / output-related inputs from §2
  above) are **fully compatible** with `lowlighter/metrics`. Key names, default values,
  and types are identical.
- **Unsupported inputs** (the `plugin_<slug>` gates and associated inputs
  for plugins covered in §3) are **silently no-ops**. Workflow files work
  without any changes, and only the output of unsupported plugins is not
  generated.
- Invalid input names or types do **not** cause a hard error, either. As in
  `lowlighter/metrics`, unknown keys are passed through untouched.

### 4.1 Example behavior

The following workflow works **as-is** with the Go port:

```yaml
- uses: mjun0812/github-metrics@latest
  with:
    user: octocat
    plugin_languages: yes # Supported -> reflected in output
    plugin_languages_limit: "5" # Supported -> reflected
    plugin_anilist: yes # Unsupported -> silently no-op
    plugin_anilist_medias: anime # Unsupported -> silently no-op
```

Result: the languages panel is drawn in the SVG, and the anilist panel is
not included. No warning or error occurs.

## 5. Migration steps

### (Optional) Automated first pass with `migrate-from-lowlighter.sh`

The repo ships a helper script that mechanically rewrites `uses:` lines
for you and, on request, strips `plugin_*` gates for plugins listed in
§3.3 -- §3.5 above. It is a convenience wrapper for the mechanical parts
of steps 1 and 2 below; there is no requirement to use it.

```bash
# Preview the changes without touching any file.
bash scripts/migrate-from-lowlighter.sh --dry-run

# Rewrite in place. Each modified file gets a <file>.bak backup.
bash scripts/migrate-from-lowlighter.sh

# Also strip `plugin_*` gates for unported plugins (see §3.3 -- §3.5).
bash scripts/migrate-from-lowlighter.sh --strip-unported
```

By default the script scans `.github/workflows`; pass an explicit file
or directory to override. See `bash scripts/migrate-from-lowlighter.sh --help`
for the full option list. The script only rewrites `uses:` lines; the
`with:` compatibility rules in §4 still apply.

### Step 1: Replace the `uses:` line

```diff
- uses: lowlighter/metrics@v3.34
+ uses: mjun0812/github-metrics@latest
```

`@latest` always resolves to the newest published release (no workflow
changes are needed when new releases ship). If you want to pin to a
byte-exact version, use the exact `vX.Y.Z` form (e.g. `@v5.0.0`), or a
floating major-tag such as `@v5` which tracks the latest `v5.x.y`.

### Step 2: (Optional) remove unsupported inputs

Removing unsupported `plugin_*` gates from the workflow improves
readability. This is **not required**, since silently no-op behavior is
guaranteed per §4.

### Step 3: Re-run

Trigger the workflow via `workflow_dispatch`, or wait for the next
scheduled run. Output actions such as `output_action: commit` also work
with the same semantics as `lowlighter/metrics`.

### Step 4: Verify the output

- **JSON output**: Since it is byte-compatible with `lowlighter/metrics`, you can
  confirm that `diff old.json new.json` produces no output.
- **SVG output**: Equivalence is at the DOM structure level. The version
  string (`github-metrics@vX.Y.Z`) and generation timestamp
  (`Last updated ...`) will differ. To check the structure, format with
  `xmllint --format` before diffing.

## 6. Rollback

You can fully roll back by reverting the `uses:` line to its previous value
(since supported inputs are drop-in compatible, no changes are needed to
your configuration files):

```diff
- uses: mjun0812/github-metrics@latest
+ uses: lowlighter/metrics@v3.34
```

Commit / push to finish. `lowlighter/metrics` behavior is fully restored.
