<!-- AUTOGEN_START: title-and-description -->

# Plugin: stars

This plugin displays recently starred repositories.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![stars sample](../examples/plugin-stars.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                | Description         | Default | Required | Type    |
| -------------------- | ------------------- | ------- | -------- | ------- |
| `plugin_stars`       | Enable stars plugin | `no`    | no       | boolean |
| `plugin_stars_limit` | Display limit       | `4`     | no       | number  |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@latest
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_stars: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_stars=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**At least one starred repository.** Reads starred repositories via GraphQL (`user.starredRepositories`). Accounts that never starred anything see an empty card.

## Notes

- Starred dates are rendered as relative labels (`3 days ago`, `2 hours ago`) when recent, and fall back to the absolute date for older entries.

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/stars/metadata.yml`](../../assets/plugins/stars/metadata.yml) — `lowlighter/metrics` metadata
- Supported account types: user
- Required scopes: public_access
