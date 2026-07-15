<!-- AUTOGEN_START: title-and-description -->
# Plugin: habits

This plugin displays coding habits based on recent activity, such as active hours and languages recently used.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![habits sample](../examples/plugin-habits.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_habits` | Enable habits plugin | `no` | no | boolean |
| `plugin_habits_from` | Events to use | `200` | no | number |
| `plugin_habits_skipped` | Skipped repositories | `` | no | array |
| `plugin_habits_days` | Event maximum age | `14` | no | number |
| `plugin_habits_facts` | Mildly interesting facts | `yes` | no | boolean |
| `plugin_habits_charts` | Charts | `yes` | no | boolean |
| `plugin_habits_charts_type` | Charts display type | `classic` | no | string |
| `plugin_habits_trim` | Trim unused hours on charts | `no` | no | boolean |
| `plugin_habits_languages_limit` | Display limit (languages) | `8` | no | number |
| `plugin_habits_languages_threshold` | Display threshold (percentage) | `0%` | no | string |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_habits: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_habits=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/habits/metadata.yml`](../../assets/plugins/habits/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
