<!-- AUTOGEN_START: title-and-description -->

# Plugin: base

Restores the activity / community / repositories summary panels that
were originally bundled into the upstream `base` chrome. The panels
read aggregated data from the shared `dataprovider.Provider`
(Profile + RepositorySummary) — no GraphQL or REST fetching duplication.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![base sample](../examples/plugin-base.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input         | Description                                                                            | Default | Required | Type    |
| ------------- | -------------------------------------------------------------------------------------- | ------- | -------- | ------- |
| `plugin_base` | Enable base plugin (compat master switch; prefer the per-section `chrome_*` booleans). | `no`    | no       | boolean |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_base: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_base=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Requirements

Base reads `Provider.Profile(ctx)` and `Provider.RepositorySummary(ctx)` from the shared `internal/dataprovider`. Both are populated lazily by the standard GraphQL user/organization + repositories paging queries — no extra API scopes beyond `public_access` are required. The plugin emits no standalone card on its own; its two panels (gated by `chrome_activity` / `chrome_community` and `chrome_repositories` from `assets/plugins/chrome/metadata.yml`, #640) compose with any other plugin selection to restore the legacy `base` chrome look.

## Chrome inputs

The chrome section gate is controlled by six boolean Action inputs in the `chrome_*` namespace (introduced in v2.1.0 / #640). Each input defaults to `no` — opt-in only, matching the v2 per-plugin SVG default UX. See `assets/plugins/chrome/metadata.yml` for the canonical descriptions surfaced in `action.yml`.

> **v3.0 removal**: the legacy inputs in the left column below were removed in v3.0 (#649). The table is preserved as a migration reference for users upgrading from v2.x. New invocations must use the right-column form; passing a legacy key now emits a one-shot `slog.Warn` and is otherwise silently ignored.

| Legacy v2 input (removed in v3.0)       | v2.1+ / v3.0 replacement           |
| --------------------------------------- | ---------------------------------- |
| `--plugin base=header`                  | `--plugin chrome_header=yes`       |
| `--plugin base=activity`                | `--plugin chrome_activity=yes`     |
| `--plugin base=community`               | `--plugin chrome_community=yes`    |
| `--plugin base=repositories`            | `--plugin chrome_repositories=yes` |
| `--plugin base=metadata`                | `--plugin chrome_metadata=yes`     |
| `--plugin base=introduction`            | `--plugin chrome_introduction=yes` |
| `--plugin plugin_base_activity=yes`     | `--plugin chrome_activity=yes`     |
| `--plugin plugin_base_repositories=yes` | `--plugin chrome_repositories=yes` |

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/base/metadata.yml`](../../assets/plugins/base/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
