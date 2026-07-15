<!-- AUTOGEN_START: title-and-description -->
# Plugin: isocalendar

This plugin displays an isometric view of a user commit calendar along with a few additional statistics like current streak and average number of commit per day.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![isocalendar sample](../examples/plugin-isocalendar.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_isocalendar` | Enable isocalendar plugin | `no` | no | boolean |
| `plugin_isocalendar_duration` | Time range | `half-year` | no | string |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_isocalendar: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_isocalendar=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

Same data source as the `calendar` plugin (the user's public contribution calendar). Public contributions within the past year are required for a non-empty heatmap. The `full-year` mode renders 52 weeks of the same dataset.

## Notes

- The label reads "Commits per day", but the data source is the daily `contributionCount` from GitHub GraphQL's `contributionsCollection.contributionCalendar`, which is not a commit-only count (it also includes issues / PRs / reviews, etc.). This matches the behavior of `lowlighter/metrics`.
- Private contributions are folded into `contributionCount` on GitHub's side when the user's GitHub setting "Include private contributions on my profile" is enabled. The plugin does not perform any public/private filtering itself.
- The aggregation period matches `lowlighter/metrics`: `half-year` starts at now−180 days, and `full-year` starts at now−1 year, each rounded down to the preceding Sunday 00:00 UTC.
- The calendar is fetched in 4-week chunks, same as `lowlighter/metrics`. GitHub normalizes the heatmap color (`ContributionDay.color`) against the **maximum value within the queried period**, so fetching in chunks produces the same color gradient as `lowlighter/metrics` (fetching a full year in one request would collapse most days to the lightest shade — #467).
- If the GraphQL client is unavailable or the fetch fails, it falls back to a degraded path that slices the trailing 26 weeks / 53 weeks from the shared indepth calendar (past 1 year).

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/isocalendar/metadata.yml`](../../assets/plugins/isocalendar/metadata.yml) — upstream metadata
- Supported account types: user
- Required scopes: public_access
