# Output comparison

Side-by-side comparison of each plugin's output:

- **Left** — upstream [`lowlighter/metrics`](https://github.com/lowlighter/metrics) rendered with lowlighter's profile data. Source: [`docs/original_examples/`](original_examples/).
- **Middle** — upstream `lowlighter/metrics@v3.34` rendered with `mjun0812`'s data, as an apples-to-apples reference. Source: [`docs/reference_examples/`](reference_examples/).
- **Right** — this project's Go implementation rendered with `mjun0812`'s data. Source: [`docs/examples/`](examples/).

The left column uses a different data source, so the numbers will not match. The middle and right columns use the same data and should match at the DOM / layout level.

## Templates

| Type       | upstream (lowlighter)                                            | upstream (mjun0812)                                               | Go (mjun0812)                                           |
| ---------- | ---------------------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------- |
| header     | <img src="original_examples/metrics.base.svg" width="420">       | <img src="reference_examples/metrics.base.svg" width="420">       | <img src="examples/plugin-header.svg" width="420">      |
| classic    | <img src="original_examples/metrics.classic.svg" width="420">    | <img src="reference_examples/metrics.classic.svg" width="420">    | <img src="examples/metrics-classic.svg" width="420">    |
| repository | <img src="original_examples/metrics.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.svg" width="420"> | <img src="examples/metrics-repository.svg" width="420"> |

## Plugins

### languages

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go (`plugin-languages.svg`)                           |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.svg" width="420"> | <img src="examples/plugin-languages.svg" width="420"> |

**variant: details**

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go                                                            |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="examples/plugin-languages-details.svg" width="420"> |

**variant: recent**

| upstream (lowlighter)                                                         | upstream (mjun0812)                                | Go                                                           |
| ----------------------------------------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.languages.recent.svg" width="420"> | — _upstream v3.34 bug (TypeError in `recent.mjs`)_ | <img src="examples/plugin-languages-recent.svg" width="420"> |

**variant: indepth**

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go                                                            |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="examples/plugin-languages-indepth.svg" width="420"> |

### activity

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                       | Go (`plugin-activity.svg`)                           |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.activity.svg" width="420"> | — _upstream v3.34 bug (`TypeError: Cannot read properties of undefined`)_ | <img src="examples/plugin-activity.svg" width="420"> |

### achievements

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                            | Go (`plugin-achievements.svg`)                           |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.svg" width="420"> | — _GitHub deprecated the Projects (classic) API; upstream returns `NOT_FOUND`_ | <img src="examples/plugin-achievements.svg" width="420"> |

**variant: compact**

| upstream (lowlighter)                                                             | upstream (mjun0812)                                                            | Go                                                               |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.compact.svg" width="420"> | — _GitHub deprecated the Projects (classic) API; upstream returns `NOT_FOUND`_ | <img src="examples/plugin-achievements-compact.svg" width="420"> |

### repositories

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go (`plugin-repositories.svg`)                           |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.svg" width="420"> | <img src="examples/plugin-repositories.svg" width="420"> |

**variant: pinned**

| upstream (lowlighter)                                                            | upstream (mjun0812)                                                               | Go                                                              |
| -------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.pinned.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.pinned.svg" width="420"> | <img src="examples/plugin-repositories-pinned.svg" width="420"> |

### isocalendar

| upstream (lowlighter)                                                    | upstream (mjun0812)                                                       | Go (`plugin-isocalendar.svg`)                           |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------- | ------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="examples/plugin-isocalendar.svg" width="420"> |

**variant: fullyear**

| upstream (lowlighter)                                                             | upstream (mjun0812)                                                                | Go                                                               |
| --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="examples/plugin-isocalendar-fullyear.svg" width="420"> |

### calendar

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go (`plugin-calendar.svg`)                           |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.svg" width="420"> | <img src="examples/plugin-calendar.svg" width="420"> |

**variant: full**

| upstream (lowlighter)                                                      | upstream (mjun0812)                                                         | Go                                                        |
| -------------------------------------------------------------------------- | --------------------------------------------------------------------------- | --------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.full.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.full.svg" width="420"> | <img src="examples/plugin-calendar-full.svg" width="420"> |

### habits

| upstream (lowlighter)                                                      | upstream (mjun0812)                                                            | Go (`plugin-habits.svg`)                           |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — _upstream v3.34 bug (`TypeError: Cannot destructure 'author' of undefined`)_ | <img src="examples/plugin-habits.svg" width="420"> |

**variant: facts**

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                            | Go                                                       |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.facts.svg" width="420"> | — _upstream v3.34 bug (`TypeError: Cannot destructure 'author' of undefined`)_ | <img src="examples/plugin-habits-facts.svg" width="420"> |

**variant: charts**

| upstream (lowlighter)                                                      | upstream (mjun0812)                                                            | Go                                                        |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | --------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — _upstream v3.34 bug (`TypeError: Cannot destructure 'author' of undefined`)_ | <img src="examples/plugin-habits-charts.svg" width="420"> |

### stars

| upstream (lowlighter)                                              | upstream (mjun0812)                                                 | Go (`plugin-stars.svg`)                           |
| ------------------------------------------------------------------ | ------------------------------------------------------------------- | ------------------------------------------------- |
| <img src="original_examples/metrics.plugin.stars.svg" width="420"> | <img src="reference_examples/metrics.plugin.stars.svg" width="420"> | <img src="examples/plugin-stars.svg" width="420"> |

### topics

| upstream (lowlighter)                                               | upstream (mjun0812)                                                  | Go (`plugin-topics.svg`)                           |
| ------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.topics.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.svg" width="420"> | <img src="examples/plugin-topics.svg" width="420"> |

**variant: icons**

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go                                                 |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.topics.icons.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.icons.svg" width="420"> | — _the sample user has no topics on starred repos_ |

### starlists

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go (`plugin-starlists.svg`)                           |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.svg" width="420"> | <img src="reference_examples/metrics.plugin.starlists.svg" width="420"> | <img src="examples/plugin-starlists.svg" width="420"> |

**variant: languages**

| upstream (lowlighter)                                                            | upstream (mjun0812)                  | Go                                   |
| -------------------------------------------------------------------------------- | ------------------------------------ | ------------------------------------ |
| <img src="original_examples/metrics.plugin.starlists.languages.svg" width="420"> | — _the sample user has no starlists_ | — _the sample user has no starlists_ |

### people

| upstream (lowlighter)                                                         | upstream (mjun0812)                                                  | Go (`plugin-people.svg`)                           |
| ----------------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.followers.svg" width="420"> | <img src="reference_examples/metrics.plugin.people.svg" width="420"> | <img src="examples/plugin-people.svg" width="420"> |

**variant: repository**

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go                                                            |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420"> | <img src="examples/plugin-people-repo-types.svg" width="420"> |

### notable

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go (`plugin-notable.svg`)                           |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.svg" width="420"> | <img src="examples/plugin-notable.svg" width="420"> |

**variant: indepth**

| upstream (lowlighter)                                                        | upstream (mjun0812)                                                           | Go                                                          |
| ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="examples/plugin-notable-indepth.svg" width="420"> |

### contributors

| upstream (lowlighter)                                                                | upstream (mjun0812)                                                                   | Go                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.categories.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

**variant: contributions**

| upstream (lowlighter)                                                                   | upstream (mjun0812)                                                                   | Go                                                                          |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.contributions.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

### reactions

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go (`plugin-reactions.svg`)                           |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.reactions.svg" width="420"> | <img src="reference_examples/metrics.plugin.reactions.svg" width="420"> | <img src="examples/plugin-reactions.svg" width="420"> |

### projects

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                            | Go (`plugin-projects.svg`)                           |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.projects.svg" width="420"> | — _GitHub deprecated the Projects (classic) API; upstream returns `NOT_FOUND`_ | <img src="examples/plugin-projects.svg" width="420"> |

### sponsors

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go (`plugin-sponsors.svg`)                           |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="examples/plugin-sponsors.svg" width="420"> |

**variant: full**

| upstream (lowlighter)                                                      | upstream (mjun0812)                 | Go                                  |
| -------------------------------------------------------------------------- | ----------------------------------- | ----------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.full.svg" width="420"> | — _the sample user has no sponsors_ | — _the sample user has no sponsors_ |

### sponsorships

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go (`plugin-sponsorships.svg`)                           |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="examples/plugin-sponsorships.svg" width="420"> |

### stargazers

| upstream (lowlighter)                                                   | upstream (mjun0812)                                                      | Go (`plugin-stargazers.svg`)                           |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="examples/plugin-stargazers.svg" width="420"> |

**variant: graph**

| upstream (lowlighter)                                                         | upstream (mjun0812)                                                            | Go                                                           |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="examples/plugin-stargazers-graph.svg" width="420"> |

### traffic

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go (`plugin-traffic.svg`)                           |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.traffic.svg" width="420"> | <img src="reference_examples/metrics.plugin.traffic.svg" width="420"> | <img src="examples/plugin-traffic.svg" width="420"> |

## Repository mode

`--template repository --user mjun0812 --repo flash-attention-prebuild-wheels`.

| Type         | upstream (mjun0812)                                                                   | Go                                                                          |
| ------------ | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| repository   | <img src="reference_examples/metrics.repository.svg" width="420">                     | <img src="examples/metrics-repository.svg" width="420">                     |
| languages    | <img src="reference_examples/metrics.repository.plugin.languages.svg" width="420">    | <img src="examples/plugin-languages-repo.svg" width="420">                  |
| contributors | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |
| people       | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420">       | <img src="examples/plugin-people-repo-types.svg" width="420">               |
| stargazers   | <img src="reference_examples/metrics.repository.plugin.stargazers.svg" width="420">   | <img src="examples/metrics-repository.svg" width="420">                     |
