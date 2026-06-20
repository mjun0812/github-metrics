<!-- AUTOGEN_START: title-and-description -->
# Plugin: reactions

This plugin displays overall user reactions on recent issues, comments and discussions.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![reactions sample](../examples/plugin-reactions.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_reactions` | Enable reactions plugin | `no` | no | boolean |
| `plugin_reactions_limit` | Display limit (issues and pull requests comments) | `200` | no | number |
| `plugin_reactions_limit_issues` | Display limit (issues and pull requests, first comment) | `100` | no | number |
| `plugin_reactions_limit_discussions` | Display limit (discussions, first comment) | `100` | no | number |
| `plugin_reactions_limit_discussions_comments` | Display limit (discussions comments) | `100` | no | number |
| `plugin_reactions_days` | Comments maximum age | `0` | no | number |
| `plugin_reactions_display` | Display mode | `absolute` | no | string |
| `plugin_reactions_details` | Additional details | `` | no | array |
| `plugin_reactions_ignored` | Ignored users | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_reactions: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_reactions=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/reactions/metadata.yml`](../../assets/plugins/reactions/metadata.yml) — upstream metadata
- Supported account types: user
- Required scopes: public_access
