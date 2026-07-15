<!-- AUTOGEN_START: title-and-description -->
# Plugin: stargazers

This plugin displays stargazers evolution across affiliated repositories.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![stargazers sample](../examples/plugin-stargazers.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_stargazers` | Enable stargazers plugin | `no` | no | boolean |
| `plugin_stargazers_days` | Time range | `14` | no | number |
| `plugin_stargazers_charts` | Charts | `yes` | no | boolean |
| `plugin_stargazers_charts_type` | Charts display type | `classic` | no | string |
| `plugin_stargazers_worldmap` | Stargazers worldmap | `no` | no | boolean |
| `plugin_stargazers_worldmap_token` | Stargazers worldmap token | `` | no | token |
| `plugin_stargazers_worldmap_sample` | Stargazers worldmap sample | `0` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_stargazers: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_stargazers=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/stargazers/metadata.yml`](../../assets/plugins/stargazers/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: public_access
