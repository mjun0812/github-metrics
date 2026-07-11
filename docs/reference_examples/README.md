# Reference examples (upstream, real data)

This directory contains reference cards generated with the upstream
[`lowlighter/metrics`](https://github.com/lowlighter/metrics) **itself**. They target
`mjun0812`'s own GitHub data and the repository
[`mjun0812/flash-attention-prebuild-wheels`](https://github.com/mjun0812/flash-attention-prebuild-wheels).

## Why a separate directory

- [`docs/org_examples`](../org_examples) (= `original_examples`) is a straight import of
  upstream's `examples` branch, whose **data subject is lowlighter himself**. It can be
  used for structural comparison, but since the data differs, it does not allow an
  apples-to-apples comparison of "how do upstream and the Go implementation differ given
  the same input."
- This directory is the output of feeding **the same data (mjun0812 /
  flash-attention-prebuild-wheels)** into upstream. Placing it side by side with
  [`docs/examples`](../examples) (this project's Go implementation output) lets you
  check the visual/DOM diff with the data held constant.

## Generation conditions

| Item              | Value                                                                                                                                               |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Tool              | `lowlighter/metrics@v3.34` (upstream itself, GitHub Action)                                                                                         |
| Execution method  | A temporary GitHub Actions workflow was run once on a GitHub-hosted runner (`ubuntu-latest`), and `github-actions[bot]` committed to this directory |
| Target user       | `mjun0812`                                                                                                                                          |
| Target repository | `mjun0812/flash-attention-prebuild-wheels` (`template: repository`)                                                                                 |
| Target plugins    | **Only plugins already implemented in Go by this project** (unadopted plugins are not generated)                                                    |
| `output_action`   | `none` (render only; committing is done separately by the workflow's final step)                                                                    |
| `config_timezone` | `Asia/Tokyo` (some cards)                                                                                                                           |
| Generation date   | 2026-05-31                                                                                                                                          |

> Addendum: `metrics.plugin.people.svg` was regenerated on its own on 2026-06-05 via a
> local Docker run of `ghcr.io/lowlighter/metrics:v3.34`, because the follower count was
> updated to 60 when `docs/examples` was regenerated.

> Warning: these are **raw**, non-normalized upstream output. Dynamic parts such as the
> footer's generation timestamp and the `Metrics` version string change on every
> regeneration. The `docs/examples` side (this project's Go implementation output) also
> retains raw timestamps in the same way, so skip over these dynamic parts when comparing.

## Cards that failed to render correctly (removed)

The following cards were **removed from this directory** because, at generation time, an
**error on the upstream v3.34 side** caused `Unexpected error` to be rendered on the card
itself, or the target data could not be fetched and the card came out empty. None of
these are bugs in this project's Go implementation; they are all code bugs in the
upstream tool, GitHub API spec changes, or permission/data issues.

| Removed card                              | Cause                                                                                                                                                                                                                                                |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `metrics.plugin.achievements.svg`         | GitHub **deprecated the Projects (classic) API** ([sunset 2024-05](https://github.blog/changelog/2024-05-23-sunset-notice-projects-classic/)). Since achievements internally references classic projects, GraphQL `NOT_FOUND` -> `Unexpected error`. |
| `metrics.plugin.achievements.compact.svg` | Same as above (compact display of achievements).                                                                                                                                                                                                     |
| `metrics.plugin.projects.svg`             | Same as above. The `projects` plugin directly references the Projects (classic) API, causing `NOT_FOUND` -> `Unexpected error`.                                                                                                                      |
| `metrics.plugin.activity.svg`             | An upstream code bug. `TypeError: Cannot read properties of undefined (reading 'filter')` (`source/plugins/activity`) causes `Unexpected error`.                                                                                                     |
| `metrics.repository.plugin.activity.svg`  | Same as above (activity in the repository template).                                                                                                                                                                                                 |
| `metrics.plugin.habits.facts.svg`         | An upstream code bug. `TypeError: Cannot destructure property 'author' of 'undefined'` (`source/plugins/habits/index.mjs:51`) causes `Unexpected error`.                                                                                             |
| `metrics.plugin.habits.charts.svg`        | Same as above (charts display of habits).                                                                                                                                                                                                            |
| `metrics.repository.plugin.traffic.svg`   | The `traffic` API requires repository push/admin permission; data could not be fetched, so the section was skipped. Removed because it results in an empty card with no content (height ~8px).                                                       |

### Not generated (excluded from the start)

| Card                                  | Cause                                                                                                                                                                                                                                                                                       |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `metrics.plugin.languages.recent.svg` | Upstream v3.34's `source/plugins/languages/analyzer/recent.mjs:70` deterministically throws an exception (`Array.filter`) on `mjun0812`'s data. This only occurs when `plugin_languages_sections: recently-used` is specified, so it was not included in the workflow's generation targets. |

> Notes:
>
> - `metrics.plugin.stargazers.svg` / `.graph.svg` have no errors and render real data; they are **normal**.
> - `metrics.plugin.traffic.svg` (user template) has no data for the traffic section, but the
>   card itself, including other elements, is rendered and is retained.
> - The `languages` plugin itself and the `.details` / `.indepth` cards are all normal.
> - Cards with little text, such as `metrics.plugin.reactions.svg`, are not errors; they are
>   simply normal cards with little target data.

## File list

### User cards (`user: mjun0812`)

- `metrics.base.svg` / `metrics.classic.svg`
- `metrics.plugin.calendar.svg` / `metrics.plugin.calendar.full.svg`
- `metrics.plugin.isocalendar.svg` / `metrics.plugin.isocalendar.fullyear.svg`
- `metrics.plugin.languages.svg` / `.details.svg` / `.indepth.svg`
- `metrics.plugin.notable.svg` / `metrics.plugin.notable.indepth.svg`
- `metrics.plugin.people.svg`
- `metrics.plugin.reactions.svg`
- `metrics.plugin.repositories.svg` / `metrics.plugin.repositories.pinned.svg`
- `metrics.plugin.sponsors.svg` / `metrics.plugin.sponsorships.svg`
- `metrics.plugin.stargazers.svg` / `metrics.plugin.stargazers.graph.svg`
- `metrics.plugin.stars.svg`
- `metrics.plugin.starlists.svg`
- `metrics.plugin.topics.svg` / `metrics.plugin.topics.icons.svg`
- `metrics.plugin.traffic.svg`

> achievements / achievements.compact / activity / habits.facts / habits.charts /
> projects could not be rendered due to upstream v3.34 errors and were removed (see
> "Cards that failed to render correctly" above).

### Repository cards (`repo: flash-attention-prebuild-wheels`, `template: repository`)

- `metrics.repository.svg`
- `metrics.repository.plugin.languages.svg`
- `metrics.repository.plugin.contributors.svg`
- `metrics.repository.plugin.people.svg`
- `metrics.repository.plugin.stargazers.svg`

> activity / traffic could not be rendered due to upstream v3.34 errors and insufficient
> data, and were removed (see "Cards that failed to render correctly" above).

## Regeneration

The reference cards were generated with a temporary GitHub Actions workflow. The flow is:
set the repository secret `METRICS_TOKEN` (a classic PAT), push the workflow to the
target branch, run it once on a GitHub-hosted runner -> `github-actions[bot]` commits to
this directory. With `github.token`, data outside the repository cannot be fetched and
the cards come out empty, so a PAT secret is required.

> Warning: this does not work on self-hosted runners. The upstream action mounts
> `--volume $GITHUB_EVENT_PATH`, but since no actual file exists on the host side,
> `@actions/github` inside the container dies immediately with `EISDIR` (reading
> `event.json` as a directory), and not a single card gets generated. You must use a
> GitHub-hosted runner (`ubuntu-latest`).

### One-off regeneration with local Docker

To regenerate just one card locally, you can run the upstream image directly. Set
`GITHUB_TOKEN` (a PAT with roughly `repo` / `read:user` scope) as an environment
variable, and pass `INPUT_*` to the upstream image. Note that values must be url-encoded
with `jq @uri` when passed (upstream's `metadata.mjs` decodes them with
`decodeURIComponent`). Example:

```sh
docker run --init --rm -v "$PWD/docs/reference_examples:/renders" \
  -e INPUT_TOKEN="$GITHUB_TOKEN" \
  -e INPUT_OUTPUT_ACTION=none \
  -e INPUT_USER=mjun0812 \
  -e INPUT_BASE= \
  -e INPUT_PLUGIN_LANGUAGES=yes \
  -e INPUT_FILENAME=metrics.plugin.languages.svg \
  ghcr.io/lowlighter/metrics:v3.34
```
