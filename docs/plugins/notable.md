<!-- AUTOGEN_START: title-and-description -->

# Plugin: notable

This plugin displays badges for notable contributions on repositories.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![notable sample](../examples/plugin-notable.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                         | Description                          | Default        | Required | Type    |
| ----------------------------- | ------------------------------------ | -------------- | -------- | ------- |
| `plugin_notable`              | Enable notable plugin                | `no`           | no       | boolean |
| `plugin_notable_filter`       | Query filter                         | ``             | no       | string  |
| `plugin_notable_skipped`      | Skipped repositories                 | ``             | no       | array   |
| `plugin_notable_from`         | Repository owner account type filter | `organization` | no       | string  |
| `plugin_notable_repositories` | Repository name                      | `no`           | no       | boolean |
| `plugin_notable_indepth`      | Indepth mode                         | `no`           | no       | boolean |
| `plugin_notable_types`        | Contribution types filter            | `commit`       | no       | array   |
| `plugin_notable_self`         | Include own repositories             | `no`           | no       | boolean |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_notable: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_notable=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/notable/metadata.yml`](../../assets/plugins/notable/metadata.yml) — upstream metadata
- Supported account types: user
- Required scopes: public_access
