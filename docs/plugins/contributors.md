<!-- AUTOGEN_START: title-and-description -->
# Plugin: contributors

This plugin display repositories contributors from a commit range along with additional stats.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![contributors sample](../examples/plugin-contributors-repo-contributions.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_contributors` | Enable contributors plugin | `no` | no | boolean |
| `plugin_contributors_base` | Base reference | `` | no | string |
| `plugin_contributors_head` | Head reference | `master` | no | string |
| `plugin_contributors_ignored` | Ignored users | `` | no | array |
| `plugin_contributors_contributions` | Contributions count | `no` | no | boolean |
| `plugin_contributors_sections` | Displayed sections | `contributors` | no | array |
| `plugin_contributors_categories` | Contribution categories | `{ "📚 Documentation": ["README.md", "docs/**"], "💻 Code": ["source/**", "src/**"], "#️⃣ Others": ["*"] }` | no | json |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v5
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_contributors: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_contributors=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/contributors/metadata.yml`](../../assets/plugins/contributors/metadata.yml) — upstream metadata
- Supported account types: repository
- Required scopes: public_access
