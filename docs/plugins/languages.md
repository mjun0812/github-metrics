<!-- AUTOGEN_START: title-and-description -->
# Plugin: languages

This plugin can display which languages you use across all repositories you contributed to.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![languages sample](../examples/plugin-languages.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_languages` | Enable languages plugin | `no` | no | boolean |
| `plugin_languages_ignored` | Ignored languages | `` | no | array |
| `plugin_languages_skipped` | Skipped repositories | `` | no | array |
| `plugin_languages_limit` | Display limit | `8` | no | number |
| `plugin_languages_threshold` | Display threshold (percentage) | `0%` | no | string |
| `plugin_languages_other` | Group unknown, ignored and over-limit languages into "Other" category | `no` | no | boolean |
| `plugin_languages_colors` | Custom languages colors | `github` | no | array |
| `plugin_languages_aliases` | Custom languages names | `` | no | string |
| `plugin_languages_sections` | Displayed sections | `most-used` | no | array |
| `plugin_languages_details` | Additional details | `` | no | array |
| `plugin_languages_indepth` | Indepth mode | `no` | no | boolean |
| `plugin_languages_indepth_custom` | Indepth mode - Custom repositories | `` | no | array |
| `plugin_languages_analysis_timeout` | Indepth mode - Analysis timeout | `15` | no | number |
| `plugin_languages_analysis_timeout_repositories` | Indepth mode - Analysis timeout (repositories) | `7.5` | no | number |
| `plugin_languages_categories` | Indepth mode - Displayed categories (most-used section) | `markup, programming` | no | array |
| `plugin_languages_recent_categories` | Indepth mode - Displayed categories (recently-used section) | `markup, programming` | no | array |
| `plugin_languages_recent_load` | Indepth mode - Events to load (recently-used section) | `300` | no | number |
| `plugin_languages_recent_days` | Indepth mode - Events maximum age (day, recently-used section) | `14` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_languages: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_languages=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/languages/metadata.yml`](../../assets/plugins/languages/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: public_access
