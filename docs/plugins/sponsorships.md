<!-- AUTOGEN_START: title-and-description -->
# Plugin: sponsorships

This plugin displays sponsorships funded through [GitHub sponsors](https://github.com/sponsors/).
<!-- AUTOGEN_END: title-and-description -->

## Sample

![sponsorships sample](../examples/plugin-sponsorships.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_sponsorships` | Enable sponsorships plugin | `no` | no | boolean |
| `plugin_sponsorships_sections` | Displayed sections | `amount, sponsorships` | no | array |
| `plugin_sponsorships_size` | Profile picture display size | `24` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_sponsorships: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_sponsorships=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/sponsorships/metadata.yml`](../../assets/plugins/sponsorships/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: read:user, read:org
