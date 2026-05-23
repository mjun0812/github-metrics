# Contract: `contributors` Plugin Run Wiring (user/org mode)

**Feature**: `012-rest-data-fetch` | **Plugin**: `internal/plugins/contributors` | **User Story**: US2

## 1. Inputs

| Key | Type | Default |
|---|---|---|
| `plugin_contributors` | bool / truthy string | `false` |
| `plugin_contributors_limit` | int | `14` |
| `plugin_contributors_ignored` | []string (CSV) | `[]` (login filter) |

## 2. Preconditions

| Check | Failure → |
|---|---|
| `pc == nil \|\| pc.Data == nil` | return nil, nil |
| `truthy(pc.Inputs["plugin_contributors"])` | else `Skipped="plugin disabled"` |
| `Mode == "user" \|\| Mode == "organization"` | else fall through to existing repo-mode (UNCHANGED) |
| `pc.REST != nil` | else `Skipped="REST client unavailable"` |
| `len(Computed.RepositoryList) > 0` | else `List=[]` (partial 抑制) |

## 3. Execution

```pseudo
parallelize over RepositoryList with concurrency=4:
    ctx = WithTimeout(parent, 30s)
    resp = REST.Get(ctx, "/repos/{repo}/contributors?per_page=100&anon=false")
    if err:
        AppendError(*RetryableError("contributors %s: %w", repo, err))
        continue
    for each {login, avatar_url, contributions} in resp:
        if login ∈ plugin_contributors_ignored: skip
        agg[login].Commits += contributions
        agg[login].AvatarURL = avatar_url (first-write-wins)
        agg[login].Login = login

list = sort(agg.values(), by Commits desc, Login asc)
truncate list[:plugin_contributors_limit]
```

## 4. Outputs

| Field | Type | Populated when |
|---|---|---|
| `Result.List` | `[]Contributor` | aggregated + sorted + truncated |
| `Result.Mode` | string | "user" or "organization" |
| `Data.Errors` | `*RetryableError` 列 | per failed fetch |

`Contributor.Additions` / `Contributor.Deletions` は **常に 0** (contributor endpoint で取れないため)。partial は `Additions != 0 \|\| Deletions != 0` でガード済 → 差分行は emit されない。

## 5. Error model

| Scenario | Output |
|---|---|
| 1 repo 502 | `Data.Errors += 1 *RetryableError`, 他 repo の contributors は List に含まれる |
| 全 repo 失敗 | `List=[]`, `Skipped=false`, `Data.Errors` に N entries |
| empty repo (Git Repository is empty) | per-repo error として `Data.Errors` に記録、partial-results 継続 |

## 6. Test cases

### TC-001: Aggregation across 3 repos
- Setup: r1 → `[{alice,c=10}, {bob,c=5}]`, r2 → `[{alice,c=20}]`, r3 → `[{bob,c=3}]`
- Expect: `List = [{alice, c=30}, {bob, c=8}]`

### TC-002: Sort by commits desc, login asc tiebreak
- Setup: 3 contributors with same commit count 10: charlie, alice, bob
- Expect: `[alice, bob, charlie]` (login asc within tie)

### TC-003: Limit truncation
- Setup: 20 contributors, `plugin_contributors_limit=5`
- Expect: `len(List) == 5`

### TC-004: Ignored login
- Setup: `plugin_contributors_ignored="dependabot[bot]"`, repos contain dependabot
- Expect: dependabot absent from List

### TC-005: Mode = repository (M7 path)
- Setup: `Mode = "repository"`, RepositoryList = [{owner: x, repo: y}]
- Expect: existing repo-mode logic runs UNCHANGED (no REST changes to repo mode)

### TC-006: Empty RepositoryList
- Setup: `RepositoryList = []`
- Expect: `List=[]`, no HTTP calls
