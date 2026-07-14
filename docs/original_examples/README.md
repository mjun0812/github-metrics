# original_examples: `lowlighter/metrics` output reference

This directory stores **officially rendered SVGs from `lowlighter/metrics`** for reference during the Go reimplementation.
Only outputs corresponding to the plugins / templates adopted by this project ([docs/scope.md](../scope.md) §2) are extracted.

## Provenance

- Repository: `lowlighter/metrics` (origin of `org_repo/`)
- Branch: `examples`
- Commit: `1dac69e` (`chore: update examples`, 2025-07-03)
- Producer: artifacts rendered by `lowlighter/metrics` CI using lowlighter's own profile data
  (= not re-run with our own token; these are the official samples published by `lowlighter/metrics`)

How obtained (no network required, extracted from the locally fetched branch):

```bash
cd org_repo
git show "origin/examples:metrics.plugin.languages.svg" > ../docs/original_examples/metrics.plugin.languages.svg
```

## Templates / base

| File                     | Content                                         |
| ------------------------ | ----------------------------------------------- |
| `metrics.base.svg`       | classic output for base/core only (no plugins)  |
| `metrics.classic.svg`    | comprehensive sample of the classic template    |
| `metrics.repository.svg` | comprehensive sample of the repository template |

## Output of adopted plugins (19 plugin dirs)

| plugin         | files                                                                                                                   |
| -------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `languages`    | `metrics.plugin.languages.svg` / `.details.svg` / `.recent.svg` (languages.recent) / `.indepth.svg` (languages.indepth) |
| `activity`     | `metrics.plugin.activity.svg`                                                                                           |
| `achievements` | `metrics.plugin.achievements.svg` / `.compact.svg`                                                                      |
| `repositories` | `metrics.plugin.repositories.svg` / `.pinned.svg`                                                                       |
| `isocalendar`  | `metrics.plugin.isocalendar.svg` / `.fullyear.svg`                                                                      |
| `calendar`     | `metrics.plugin.calendar.svg` / `.full.svg`                                                                             |
| `habits`       | `metrics.plugin.habits.charts.svg` / `.facts.svg`                                                                       |
| `stars`        | `metrics.plugin.stars.svg`                                                                                              |
| `topics`       | `metrics.plugin.topics.svg` / `.icons.svg`                                                                              |
| `starlists`    | `metrics.plugin.starlists.svg` / `.languages.svg`                                                                       |
| `people`       | `metrics.plugin.people.followers.svg` / `.repository.svg`                                                               |
| `notable`      | `metrics.plugin.notable.svg` / `.indepth.svg`                                                                           |
| `contributors` | `metrics.plugin.contributors.categories.svg` / `.contributions.svg`                                                     |
| `reactions`    | `metrics.plugin.reactions.svg`                                                                                          |
| `projects`     | `metrics.plugin.projects.svg`                                                                                           |
| `sponsors`     | `metrics.plugin.sponsors.svg` / `.full.svg`                                                                             |
| `sponsorships` | `metrics.plugin.sponsorships.svg`                                                                                       |
| `stargazers`   | `metrics.plugin.stargazers.svg` / `.graph.svg` / `.worldmap.svg` / `.chartist.svg`                                      |
| `traffic`      | `metrics.plugin.traffic.svg`                                                                                            |

## Notes

- **Unadopted plugins are not included**: `lines` / `gists` / `code` / `introduction` / `followup` /
  `discussions` / `skyline` / `licenses` / `support`, as well as external API / community plugins (`wakatime` /
  `anilist` / `chess`, etc.), exist in `lowlighter/metrics`'s examples branch but were not extracted since they are out of adoption scope.
- **Variants that are backlog items in the Go implementation** are also included for reference:
  - `metrics.plugin.stargazers.worldmap.svg` ... the world map requires a Google Maps API key (currently a Skipped path)
  - `metrics.plugin.stargazers.chartist.svg` ... a different chart renderer
  - a comprehensive sample of organization mode (`metrics.organization.svg`) was not extracted, since only user mode is implemented
- These are intended **for visual/layout reference (parity checking) only**. The data belongs to lowlighter,
  so it cannot be used to verify a match against your own output.
