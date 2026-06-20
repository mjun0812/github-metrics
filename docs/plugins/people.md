<!-- AUTOGEN_START: title-and-description -->
# Plugin: people

This plugin can display relationships with users, such as followers, sponsors, contributors, stargazers, watchers, members, etc.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![people sample](../examples/plugin-people.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_people` | Enable people plugin | `no` | no | boolean |
| `plugin_people_limit` | Display limit | `24` | no | number |
| `plugin_people_identicons` | Force identicons pictures | `no` | no | boolean |
| `plugin_people_identicons_hide` | Hide identicons pictures | `no` | no | boolean |
| `plugin_people_size` | Profile picture display size | `28` | no | number |
| `plugin_people_types` | Displayed sections | `followers, following` | no | array |
| `plugin_people_thanks` | Special thanks | `` | no | array |
| `plugin_people_sponsors_custom` | Custom sponsors | `` | no | array |
| `plugin_people_shuffle` | Shuffle data | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_people: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_people=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/people/metadata.yml`](../../assets/plugins/people/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: public_access
