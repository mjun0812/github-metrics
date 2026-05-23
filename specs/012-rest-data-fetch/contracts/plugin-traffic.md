# Contract: `traffic` Plugin Run Wiring

**Feature**: `012-rest-data-fetch` | **Plugin**: `internal/plugins/traffic` | **User Story**: US1

## 1. Inputs (from `pc.Inputs`)

| Key | Type | Default | Source |
|---|---|---|---|
| `plugin_traffic` | bool / truthy string | `false` | upstream metadata.yml |
| `plugin_traffic_skipped` | []string (CSV) | `[]` | upstream metadata.yml (re-uses `repositories.skipped`) |

## 2. Preconditions

| Check | Failure → |
|---|---|
| `pc == nil \|\| pc.Data == nil` | return nil, nil |
| `truthy(pc.Inputs["plugin_traffic"])` | else `Skipped="plugin disabled"` |
| `pc.REST != nil` | else `Skipped="REST client unavailable"` |
| `scopes ⊇ {"repo"}` | else `Skipped="missing repo scope"` |
| `len(Computed.RepositoryList) > 0` | else `Skipped=false, Views={}, Total={}` (partial 抑制) |

## 3. Execution

```pseudo
parallelize over RepositoryList with concurrency=4:
    skip if repo.NameWithOwner ∈ plugin_traffic_skipped
    ctx = WithTimeout(parent, 30s)
    resp = REST.Get(ctx, "/repos/{repo}/traffic/views")
    if err:
        AppendError(*RetryableError("traffic %s: %w", repo, err))
        continue
    Views[repo.NameWithOwner] = TrafficView{Count: resp.count, Uniques: resp.uniques}
    Total.Count += resp.count
    Total.Uniques += resp.uniques
```

## 4. Outputs

| Field | Type | Populated when |
|---|---|---|
| `Result.Skipped` | bool | Run preconditions fail |
| `Result.SkippedReason` | string | Skipped=true |
| `Result.Views` | `map[string]TrafficView` | per-repo success |
| `Result.Total` | `TrafficView` | aggregate of all successful fetches |
| `Data.Errors` | `*RetryableError` 列 | per failed fetch (rate-limit, 502, timeout) |

## 5. Error model

| Scenario | Output |
|---|---|
| Network timeout on 1/N repos | `Data.Errors += 1 *RetryableError`, Total includes N-1 repos |
| Rate limit (429 / X-RateLimit-Remaining=0) | `Data.Errors += 1 *RetryableError("rate limit reset at ...")` |
| All N repos fail | `Skipped=false`, `Views={}`, `Total={0,0}`, `Data.Errors` has N entries |
| 403 on a repo (insufficient scope per-repo) | upstream throws fatal; we record as per-repo `*RetryableError` and continue. Plugin overall remains non-Skipped |

## 6. Test cases

### TC-001: Happy path (3 repos, all succeed)
- Setup: `RepositoryList = [r1, r2, r3]`, each returns `{count: 100, uniques: 50}`
- Expect: `Views = {r1: {100,50}, r2: {100,50}, r3: {100,50}}`, `Total = {300, 150}`, `Data.Errors = []`

### TC-002: One repo 404
- Setup: r2 returns 404
- Expect: `Views = {r1, r3}`, `Total = {200, 100}`, `Data.Errors = [1 *RetryableError]`

### TC-003: No `repo` scope
- Setup: `Scopes` returns `["public_repo"]`
- Expect: `Skipped=true`, `SkippedReason="missing repo scope"`, no HTTP calls

### TC-004: Empty RepositoryList
- Setup: `RepositoryList = []`
- Expect: `Skipped=false`, `Views={}`, `Total={0,0}`, no HTTP calls

### TC-005: `plugin_traffic_skipped` filter
- Setup: `plugin_traffic_skipped = "owner/repo-private"`, RepositoryList includes that repo
- Expect: that repo is excluded from Views, no HTTP call made for it
