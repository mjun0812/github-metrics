<!-- AUTOGEN_START: title-and-description -->
# Plugin: starlists

This plugin displays star lists.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![starlists sample](../examples/plugin-starlists.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_starlists` | Enable starlists plugin | `no` | no | boolean |
| `plugin_starlists_limit` | Display limit (star lists) | `2` | no | number |
| `plugin_starlists_limit_repositories` | Display limit (repositories per star list) | `2` | no | number |
| `plugin_starlists_languages` | Star lists languages statistics | `no` | no | boolean |
| `plugin_starlists_limit_languages` | Display limit (languages per star list) | `8` | no | number |
| `plugin_starlists_languages_ignored` | Ignored languages in star lists | `` | no | array |
| `plugin_starlists_languages_aliases` | Custom languages names in star lists | `` | no | string |
| `plugin_starlists_shuffle_repositories` | Shuffle data | `yes` | no | boolean |
| `plugin_starlists_ignored` | Skipped star lists | `` | no | array |
| `plugin_starlists_only` | Showcased star lists | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v5
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_starlists: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_starlists=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/starlists/metadata.yml`](../../assets/plugins/starlists/metadata.yml) — upstream metadata
- Supported account types: user
