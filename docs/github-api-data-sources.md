# GitHub Data Retrieval Reference

A list of which GitHub data each adopted plugin fetches, and via which retrieval method (REST / GraphQL / HTML scraping / local computation). See [`scope.md`](scope.md) for the finalized list of adopted plugins.

> Initial survey date: 2026-06-04

---

## Table of Contents

- [1. Common API Client](#1-common-api-client)
- [2. Endpoints by Retrieval Method](#2-endpoints-by-retrieval-method)
  - [2.1 REST API Only](#21-rest-api-only)
  - [2.2 GraphQL Only](#22-graphql-only)
  - [2.3 HTML Scraping](#23-html-scraping)
  - [2.4 Local Computation (No API Calls)](#24-local-computation-no-api-calls)
- [3. Data Available via Both REST and GraphQL](#3-data-available-via-both-rest-and-graphql)
- [4. Authentication Tokens](#4-authentication-tokens)
- [5. Known Limitations and Caveats](#5-known-limitations-and-caveats)

---

## 1. Common API Client

| File                            | Role                                                                              |
| ------------------------------- | --------------------------------------------------------------------------------- |
| `internal/githubapi/graphql.go` | GraphQL client (genqlient wrapper)                                                |
| `internal/githubapi/rest.go`    | REST client (httpx wrapper)                                                       |
| `internal/githubapi/scopes.go`  | Checks token scopes via the `X-OAuth-Scopes` header of `HEAD /` (for Classic PAT) |

---

## 2. Endpoints by Retrieval Method

### 2.1 REST API Only

REST is used because no equivalent GraphQL endpoint exists.

| Endpoint                                              | Data Retrieved                                                                | Plugins Using It                    |
| ----------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------- |
| `GET /users/{login}/events?per_page=100&page={n}`     | List of the user's public events (PushEvent, IssueEvent, etc.)                | activity, habits, languages(recent) |
| `GET /repos/{owner}/{repo}/commits/{sha}`             | List of changed files for a single commit                                     | habits, languages(recent)           |
| `GET /repos/{owner}/{repo}/compare/{before}...{head}` | List of changed files for a commit range                                      | habits, languages(recent)           |
| `GET /repos/{owner}/{repo}/stats/contributors`        | Commit counts and addition/deletion line counts per contributor (202 polling) | contributors                        |
| `GET /repos/{owner}/{repo}/contributors?per_page={n}` | List of repository contributors (name, commit count)                          | people(repo)                        |
| `GET /repos/{owner}/{repo}/stargazers?per_page={n}`   | List of repository stargazers                                                 | people(repo)                        |
| `GET /repos/{owner}/{repo}/subscribers?per_page={n}`  | List of repository watchers                                                   | people(repo)                        |
| `GET /users/{login}/starred?per_page=100&page={n}`    | List of repositories starred by the user                                      | repositories(starred)               |
| `GET /repos/{owner}/{repo}/traffic/views`             | Repository page view and unique visitor counts (requires `repo` scope)        | traffic                             |
| `HEAD /` (`X-OAuth-Scopes` header)                    | Token scope check                                                             | traffic                             |

### 2.2 GraphQL Only

GraphQL is used because no equivalent REST endpoint exists.

| Field / Query                                              | Data Retrieved                                                             | Plugins Using It                        |
| ---------------------------------------------------------- | -------------------------------------------------------------------------- | --------------------------------------- |
| `User(login)`                                              | Basic user information (name, bio, avatar, follower count, etc.)           | base                                    |
| `Organization(login)`                                      | Basic organization information                                             | base                                    |
| `Repository(owner, repo)`                                  | Details of a single repository                                             | base                                    |
| `UserRepositories(login, first, after)`                    | List of the user's repositories (with pagination)                          | base                                    |
| `user.contributionsCollection.contributionCalendar`        | Contribution calendar (counts by week and day) — not exposed via REST      | base → reused by calendar / isocalendar |
| `user.contributionsCollection.*`                           | Yearly statistics for commits / issues / PRs / reviews                     | base                                    |
| `user.repositoriesContributedTo(orderBy: STARGAZERS_DESC)` | List of other people's repositories the user has contributed to            | notable                                 |
| `user.followers(first: limit)`                             | List of followers                                                          | people                                  |
| `user.following(first: limit)`                             | List of accounts the user is following                                     | people                                  |
| `user.issues.reactions.content`                            | Aggregated reactions on issues                                             | reactions                               |
| `user.issueComments.reactions.content`                     | Aggregated reactions on issue comments                                     | reactions                               |
| `user.sponsorshipsAsMaintainer(first: limit)`              | List of sponsors (tier, start date)                                        | sponsors                                |
| `viewer.sponsorshipsAsSponsor(first: limit)`               | List of maintainers the user sponsors (tier, total amount)                 | sponsorships                            |
| `repository.stargazers(orderBy: STARRED_AT)`               | Time series of a repository's stargazers                                   | stargazers                              |
| `user.lists` / `list.items.repository`                     | List of Star Lists plus the repositories in each list — no REST equivalent | starlists                               |
| `user.starredRepositories(orderBy: STARRED_AT_DESC)`       | List of starred repositories (with language, license, stats)               | stars                                   |

### 2.3 HTML Scraping

HTML is parsed directly because the GitHub API does not support this data.

| URL                                      | Data Retrieved                                     | Plugin Using It |
| ---------------------------------------- | -------------------------------------------------- | --------------- |
| `https://github.com/stars/{user}/topics` | Starred topics (topic name, description, icon URL) | topics          |

Implementation: uses `goquery` to collect anchors matching the `a[href^="/topics/"]` selector.

### 2.4 Local Computation (No API Calls)

Simply processes data already fetched by the base plugin, with no additional API calls.

| Data Source                         | Generated Information                                     | Plugins Using It          |
| ----------------------------------- | --------------------------------------------------------- | ------------------------- |
| base's `ContributionCalendar.Weeks` | Monthly contribution histogram                            | calendar                  |
| base's `ContributionCalendar.Weeks` | ISO week calendar, streaks, statistics                    | isocalendar               |
| base's `RepositoryList.Languages`   | Byte distribution by language (standard mode)             | languages                 |
| base's various statistics values    | Tiered achievement badges                                 | achievements              |
| PushEvent changed files + go-enry   | Language detection (inferred from file extension/content) | habits, languages(recent) |

---

## 3. Data Available via Both REST and GraphQL

The following data can be retrieved via either REST or GraphQL, but the current implementation picks one.

| Data                                            | Current Implementation | REST Alternative                         | GraphQL Alternative                          |
| ----------------------------------------------- | ---------------------- | ---------------------------------------- | -------------------------------------------- |
| Repository list                                 | GraphQL                | `GET /users/{login}/repos`               | `user.repositories`                          |
| Followers / following                           | GraphQL                | `GET /users/{login}/followers` etc.      | `user.followers` / `user.following`          |
| Starred repositories                            | GraphQL                | `GET /users/{login}/starred`             | `user.starredRepositories`                   |
| List of a repository's stargazers               | GraphQL                | `GET /repos/{owner}/{repo}/stargazers`   | `repository.stargazers`                      |
| List of a repository's contributors (name only) | REST                   | `GET /repos/{owner}/{repo}/contributors` | `repository.defaultBranchRef.target.history` |
| List of a repository's watchers                 | REST                   | `GET /repos/{owner}/{repo}/subscribers`  | `repository.watchers`                        |

---

## 4. Authentication Tokens

### Classic PAT (currently used)

- Scope detection: the `X-OAuth-Scopes` response header of `HEAD /` (`scopes.go`)
- All plugins function normally
- Minimum required scopes:
  - Basic (public information only): `public_repo`
  - traffic plugin: `repo`

### Fine-grained PAT (support needed in the future)

- Does not return the `X-OAuth-Scopes` header, so current scope detection does not work
- The `traffic` plugin is always skipped
- Required permission: traffic → `Metadata: read`
- Support is planned once the deprecation of Classic PAT is announced

---

## 5. Known Limitations and Caveats

### REST: contributor statistics return 0 for large repositories

- `GET /repos/{owner}/{repo}/stats/contributors` returns `additions` / `deletions` as 0 for all entries on repositories with **more than 10,000 commits**
- This is a limitation of the GitHub API's specification (cannot be worked around)
- The contributors plugin should log a warning when all entries return 0

### REST: the events API is not real-time

- `GET /users/{login}/events` can have a delay of up to **6 hours**
- The GitHub documentation explicitly states "This API is not built to serve real-time use cases"
- The activity / habits / languages(recent) plugins may be affected

### HTML scraping: the topics class names have changed

- The current GitHub HTML no longer has the `.topic-name` / `h3` / `p.f3` / `p.f4` classes
- The topic name now exists as a text node directly under the anchor
- The current code works via a fallback to the slug, but retrieving the display name should add `a.Text()` as an intermediate fallback (around `http_navigator.go:109`)

```go
// Suggested fix
name := firstNonEmptyText(a, "h3", ".topic-name", "p.f3", "p.f4", "p.lh-condensed", "p")
if name == "" {
    name = strings.TrimSpace(a.Text()) // fetch from the text directly under the anchor
}
if name == "" {
    name = slug
}
```

### Fine-grained PAT: scope detection does not work

- The `X-OAuth-Scopes` header read in `scopes.go` is Classic PAT-only
- Fine-grained PAT requires using the `X-Accepted-GitHub-Permissions` header instead
- Workaround: replace the upfront scope check with an optimistic call plus graceful skip on 403

```go
// Suggested fix (traffic plugin example)
result, err := rest.TrafficViews(ctx, owner, repo)
if err != nil {
    var apiErr *githubapi.HTTPError
    if errors.As(err, &apiErr) && apiErr.StatusCode == 403 {
        return skip("insufficient permissions (repo write access required)")
    }
    return err
}
```
