<!-- AUTOGEN_START: title-and-description -->
# Plugin: header

Renders the profile header card: avatar, display name, join date,
follower count, and a 2-week contribution calendar mini-bar.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![header sample](../examples/plugin-header.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_header` | Enable header plugin | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_header: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_header=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

A valid GitHub username or organization login and a token with the `public_access` scope (matches `metadata.yml`). The header plugin reads `Provider.Profile(ctx)` and `Provider.CommitCalendar(ctx)`, both populated lazily by `internal/dataprovider` via the shared GraphQL `user(login:)` / `contributionsCollection` queries.

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/header/metadata.yml`](../../assets/plugins/header/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
