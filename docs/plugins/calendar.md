<!-- AUTOGEN_START: title-and-description -->
# Plugin: calendar

This plugin can display commit calendar across several years.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![calendar sample](../examples/plugin-calendar.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_calendar` | Enable calendar plugin | `no` | no | boolean |
| `plugin_calendar_limit` | Years to display | `1` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v5
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_calendar: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_calendar=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/calendar/metadata.yml`](../../assets/plugins/calendar/metadata.yml) — upstream metadata
- Supported account types: user
- Required scopes: public_access
