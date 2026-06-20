<!-- AUTOGEN_START: title-and-description -->
# Plugin: projects

This plugin displays progress of profile and repository projects.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![projects sample](../examples/plugin-projects.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_projects` | Enable projects plugin | `no` | no | boolean |
| `plugin_projects_limit` | Display limit | `4` | no | number |
| `plugin_projects_repositories` | Featured repositories projects | `` | no | array |
| `plugin_projects_descriptions` | Projects descriptions | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_projects: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_projects=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/projects/metadata.yml`](../../assets/plugins/projects/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: public_access, public_repo, read:project
