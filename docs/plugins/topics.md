<!-- AUTOGEN_START: title-and-description -->

# Plugin: topics

This plugin displays [starred topics](https://github.com/stars?filter=topics).

<!-- AUTOGEN_END: title-and-description -->

## Sample

![topics sample](../examples/plugin-topics.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                 | Description          | Default   | Required | Type    |
| --------------------- | -------------------- | --------- | -------- | ------- |
| `plugin_topics`       | Enable topics plugin | `no`      | no       | boolean |
| `plugin_topics_mode`  | Display mode         | `starred` | no       | string  |
| `plugin_topics_sort`  | Sorting method       | `stars`   | no       | string  |
| `plugin_topics_limit` | Display limit        | `15`      | no       | number  |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_topics: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_topics=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/topics/metadata.yml`](../../assets/plugins/topics/metadata.yml) — upstream metadata
- Supported account types: user
