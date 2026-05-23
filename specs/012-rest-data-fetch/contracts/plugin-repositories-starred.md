# Contract: `repositories.Starred` Population

**Feature**: `012-rest-data-fetch` | **Plugin**: `internal/plugins/repositories` | **User Story**: US3 (part 1)

## 1. Inputs

| Key | Type | Default |
|---|---|---|
| `plugin_repositories_starred` | bool / truthy string | `false` |
| `plugin_repositories_limit` | int | `8` |

## 2. Preconditions

| Check | Failure → |
|---|---|
| `pc.REST != nil` | else `Skipped="REST client unavailable"` (既存) |
| `truthy(pc.Inputs["plugin_repositories_starred"])` | else `Result.Starred = nil` (omitempty) |
| `pc.Data.User.Login != ""` | else `Result.Starred = []` |

## 3. Execution

```pseudo
ctx = WithTimeout(parent, 30s)
resp = REST.Get(ctx, "/users/{login}/starred?per_page=100&sort=created&direction=desc")
if err:
    AppendError(*RetryableError("starred %s: %w", login, err))
    return  // Starred stays nil
list = parse(resp, []githubREST.Repository)
mapped = list.map(restToRepository)
if len(mapped) > limit:
    mapped = mapped[:limit]
Result.Starred = mapped
```

`restToRepository` mapping は **data-model.md** §E-012-C 参照。

## 4. Outputs

| Field | Type | Populated when |
|---|---|---|
| `Result.Starred` | `[]plugins.Repository` | success — limit-truncated list |
| `Data.Errors` | `*RetryableError` | fetch fail |

## 5. Error model

| Scenario | Output |
|---|---|
| Network failure | `Starred=nil`, `Data.Errors += 1 *RetryableError` |
| 0 starred repos | `Starred=[]` (空 slice; partial は section 描画しない) |
| Pagination 2+ ページ | (本 feature では 1 ページのみ取得、limit ≤ 100 想定) |

## 6. Test cases

### TC-001: Happy path
- Setup: `/users/octocat/starred` returns 12 repos, limit=8
- Expect: `len(Starred) == 8`, first entry `NameWithOwner = "owner/most-recently-starred"`

### TC-002: Mapping correctness
- Setup: fixture has 1 repo with `private:true, fork:true, language:"Go"`
- Expect: `Repository{Visibility:"private", IsFork:true, Language:{Name:"Go"}}`

### TC-003: Network failure
- Setup: REST mux returns 502 for `/users/{login}/starred`
- Expect: `Starred=nil`, `Data.Errors` has 1 entry, `Featured` UNCHANGED

### TC-004: Not enabled
- Setup: `plugin_repositories_starred=false`
- Expect: `Starred=nil`, no HTTP call

### TC-005: Empty user login
- Setup: `Data.User.Login = ""`
- Expect: `Starred=[]`, no HTTP call
