<!-- AUTOGEN_START: title-and-description -->
# Plugin: repositories

This plugin displays a list of chosen featured repositories.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![repositories sample](../examples/plugin-repositories.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_repositories` | Enable repositories plugin | `no` | no | boolean |
| `plugin_repositories_featured` | Featured repositories | `` | no | array |
| `plugin_repositories_pinned` | Pinned repositories | `0` | no | number |
| `plugin_repositories_starred` | Featured most starred repositories | `0` | no | number |
| `plugin_repositories_random` | Featured random repositories | `0` | no | number |
| `plugin_repositories_order` | Featured repositories display order | `featured, pinned, starred, random` | no | array |
| `plugin_repositories_forks` | Include repositories forks | `no` | no | boolean |
| `plugin_repositories_affiliations` | Repositories affiliations | `owner` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v5
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_repositories: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_repositories=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/repositories/metadata.yml`](../../assets/plugins/repositories/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
