<!-- AUTOGEN_START: title-and-description -->

# Plugin: sponsors

This plugin displays sponsors and introduction text from [GitHub sponsors](https://github.com/sponsors/).

<!-- AUTOGEN_END: title-and-description -->

## Sample

![sponsors sample](../examples/plugin-sponsors.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                      | Description                  | Default             | Required | Type    |
| -------------------------- | ---------------------------- | ------------------- | -------- | ------- |
| `plugin_sponsors`          | Enable sponsors plugin       | `no`                | no       | boolean |
| `plugin_sponsors_sections` | Displayed sections           | `goal, list, about` | no       | array   |
| `plugin_sponsors_past`     | Past sponsorships            | `no`                | no       | boolean |
| `plugin_sponsors_size`     | Profile picture display size | `24`                | no       | number  |
| `plugin_sponsors_title`    | Title caption                | `Sponsor Me!`       | no       | string  |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@latest
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_sponsors: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_sponsors=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Requirements

The user must be **sponsored via GitHub Sponsors** (incoming sponsorships) for a non-empty card. Non-sponsored accounts produce an empty card; this is normal.

## Notes

- The `about` section uses GitHub Sponsors `fullDescription` when available and renders it under an `About Me` heading.

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/sponsors/metadata.yml`](../../assets/plugins/sponsors/metadata.yml) — `lowlighter/metrics` metadata
- Supported account types: user, organization, repository
- Required scopes: read:user, read:org
