<!-- AUTOGEN_START: title-and-description -->

# Plugin: traffic

This plugin displays the number of page views across affiliated repositories.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![traffic sample](../examples/plugin-traffic.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                       | Description                                                   | Default | Required | Type    |
| --------------------------- | ------------------------------------------------------------- | ------- | -------- | ------- |
| `plugin_traffic`            | Enable traffic plugin                                         | `no`    | no       | boolean |
| `plugin_traffic_skipped`    | Skipped repositories                                          | ``      | no       | array   |
| `plugin_traffic_hide_empty` | Hide repositories with zero views from the per-repo breakdown | `yes`   | no       | boolean |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@latest
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_traffic: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_traffic=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Notes

- **Display location**: The standalone traffic partial is a stub implementation that returns an empty string and has no dedicated section in the SVG output. The total view count (`TotalViews`) is inlined at the bottom of the right column of `plugin_base.RepositoriesPartial` as `<N> view[s] in last two weeks` (v2.0.2 / #638). The `plugin-traffic` sample is generated with the combination of `chrome_repositories=yes` + `plugin_traffic=yes`, and the views line appears at the end of the repositories panel (as of v3.0 / #649, v2 aliases such as `plugin_base_repositories` are no longer needed).
- **Per-repo breakdown is JSON output only**: The `Result.Views` map is only accessible via GraphQL/JSON output. Showing per-repo rows in the SVG would require a future implementation of `traffic.Partial` (it is currently an intentional stub).
- **Hiding zero-view repositories**: `plugin_traffic_hide_empty` defaults to `yes` (true). Repositories where `v.Count == 0` are excluded from the `Result.Views` map (this affects JSON output). Since the SVG display only shows the total view count, this input has no observable effect in SVG mode.
- **Requires repository admin permission**: GitHub's traffic endpoints (`/repos/{owner}/{repo}/traffic/views`, `/repos/{owner}/{repo}/traffic/clones`) are only accessible to repository administrators. A non-admin token will receive a 403. Running with `--template=repository --repo owner/name` is recommended.
- **Token scope requirement**: The `repo` scope is required. Without it, the plugin is Skipped (`Result.Skipped = true`, `SkippedReason = "missing repo scope"`) and the views line is not rendered either.
- **Handling of 403s**: If an individual repository returns a 403 (e.g., insufficient collaborator permission), only that repository is dropped and aggregation continues.

## References

- [`action.yml`](../../action.yml): canonical input schema
- [`assets/plugins/traffic/metadata.yml`](../../assets/plugins/traffic/metadata.yml): `lowlighter/metrics` metadata
- Supported account types: user, organization, repository
- Required scopes: repo
