<!-- AUTOGEN_START: title-and-description -->
# Plugin: achievements

This plugin displays several highlights about what an account has achieved on GitHub.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![achievements sample](../examples/plugin-achievements.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_achievements` | Enable achievements plugin | `no` | no | boolean |
| `plugin_achievements_threshold` | Rank threshold filter | `C` | no | string |
| `plugin_achievements_secrets` | Secrets achievements | `yes` | no | boolean |
| `plugin_achievements_display` | Display style | `detailed` | no | string |
| `plugin_achievements_limit` | Display limit | `0` | no | number |
| `plugin_achievements_ignored` | Ignored achievements | `` | no | array |
| `plugin_achievements_only` | Showcased achievements | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v5
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_achievements: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_achievements=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/achievements/metadata.yml`](../../assets/plugins/achievements/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
