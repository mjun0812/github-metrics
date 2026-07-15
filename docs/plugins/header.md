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
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_header=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

A valid GitHub username or organization login and a token with the `public_access` scope (matches `metadata.yml`). The header plugin reads `Provider.Profile(ctx)` and `Provider.CommitCalendar(ctx)`, both populated lazily by `internal/dataprovider` via the shared GraphQL `user(login:)` / `contributionsCollection` queries.

## Rendering paths (v2.1+)

The header card has two equivalent rendering entry points that produce byte-equivalent SVG from the same `Provider` data:

- **`base.header` static partial** — fires when `chrome_header=yes` is set (or the legacy `base=header` CSV is resolved by the deprecation translator). Rendered first in the classic dispatcher's `_.json` order, so the header lands at the TOP of the SVG above any plugin partial output. Lazily fetches via `Provider` even when `plugin_header=no`. This is the path used by the `metrics-classic` composite sample.
- **`plugin.header` plugin partial** — fires when `plugin_header=yes` is set. Rendered after the static partials, wrapped in `<div class="plugin-header" data-plugin="header">`. Standalone embedding case: produce `plugin-header.svg` and `<img>` it into a profile README.

The classic dispatcher de-duplicates `plugin.header` when `chrome_header=yes` is also enabled, so the card never renders twice when both gates are flipped (#635).

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/header/metadata.yml`](../../assets/plugins/header/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
