<!-- AUTOGEN_START: title-and-description -->
# Plugin: base

Foundational data-fetching plugin: walks the user / organization
/ repository pages and seeds the shared computed fields downstream
plugins consume. Owns no rendering of its own after #602 — the
identity card moved to the `header` plugin and the activity /
community / repositories summary panels were dropped outright.
<!-- AUTOGEN_END: title-and-description -->

## Sample

This plugin emits no standalone SVG; its inputs are documented below.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `repositories` | Fetched repositories | `100` | no | number |
| `repositories_batch` | Fetched repositories per query | `100` | no | number |
| `repositories_forks` | Include forks | `no` | no | boolean |
| `repositories_affiliations` | Repositories affiliations | `owner` | no | array |
| `repositories_skipped` | Default skipped repositories | `` | no | array |
| `users_ignored` | Default ignored users | `github-actions[bot], dependabot[bot], dependabot-preview[bot], actions-user, action@github.com` | no | array |
| `commits_authoring` | Identifiers that has been used for authoring commits | `.user.login` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    repositories: 100
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --setting 'repositories=100'
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**A valid GitHub username (or organization login) and a token with at minimum `read:user` + `public_repo`.** The base plugin queries the GraphQL `user(login:)` / `organization(login:)` endpoint to populate the shared `Data.User` / `Data.Organization` payload and walks the repositories connection (paged, with batch-halving on transient 5xx) to seed `Computed.RepositoryList` for every downstream plugin.

Per #602 the identity card moved to the dedicated [`header` plugin](header.md); enable it with `plugin_header: yes` to render the avatar / login / counters / two-week commit calendar block.

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/base/metadata.yml`](../../assets/plugins/base/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: public_access
