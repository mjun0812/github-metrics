<!-- AUTOGEN_START: title-and-description -->
# Plugin: header

Identity block: avatar / display name / follower & following counters /
trailing two-week commit calendar / contributed-to repositories.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![header sample](../examples/plugin-header.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_header` | Enable header plugin | `no` | no | boolean |
| `plugin_header_indepth` | Indepth mode | `no` | no | boolean |
| `plugin_header_hireable` | Show `Available for hire!` in the header section | `no` | no | boolean |
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

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/header/metadata.yml`](../../assets/plugins/header/metadata.yml) — upstream metadata
- Supported account types: user, organization
- Required scopes: public_access
