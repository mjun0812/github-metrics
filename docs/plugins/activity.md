<!-- AUTOGEN_START: title-and-description -->

# Plugin: activity

This plugin displays recent activity on GitHub.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![activity sample](../examples/plugin-activity.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                        | Description            | Default | Required | Type    |
| ---------------------------- | ---------------------- | ------- | -------- | ------- |
| `plugin_activity`            | Enable activity plugin | `no`    | no       | boolean |
| `plugin_activity_limit`      | Display limit          | `5`     | no       | number  |
| `plugin_activity_load`       | Events to load         | `300`   | no       | number  |
| `plugin_activity_days`       | Events maximum age     | `14`    | no       | number  |
| `plugin_activity_visibility` | Events visibility      | `all`   | no       | string  |
| `plugin_activity_timestamps` | Events timestamps      | `no`    | no       | boolean |
| `plugin_activity_skipped`    | Skipped repositories   | ``      | no       | array   |
| `plugin_activity_ignored`    | Ignored users          | ``      | no       | array   |
| `plugin_activity_filter`     | Events types           | `all`   | no       | array   |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@latest
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_activity: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_activity=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/activity/metadata.yml`](../../assets/plugins/activity/metadata.yml) — `lowlighter/metrics` metadata
- Supported account types: user, organization, repository
- Required scopes: public_access
