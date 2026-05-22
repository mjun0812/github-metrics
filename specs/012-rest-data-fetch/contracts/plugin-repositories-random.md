# Contract: `repositories.Random` Population

**Feature**: `012-rest-data-fetch` | **Plugin**: `internal/plugins/repositories` | **User Story**: US3 (part 2)

## 1. Inputs

| Key | Type | Default |
|---|---|---|
| `plugin_repositories_random` | int (≥0) | `0` (機能 off) |
| `plugin_repositories_random_seed` | int64 | `0` (per-run fresh seed) |

## 2. Preconditions

| Check | Failure → |
|---|---|
| `n := plugin_repositories_random > 0` | else `Result.Random = nil` |
| `len(Result.Featured) > 0` | else `Result.Random = []` |

## 3. Execution

```go
n := readInt("plugin_repositories_random", 0)
seed := readInt64("plugin_repositories_random_seed", 0)
if n <= 0 || len(Featured) == 0 {
    return
}
Result.Random = deterministicShuffle(Featured, seed, n)
```

`deterministicShuffle` (research.md §R-004):

```go
func deterministicShuffle(in []Repository, seed int64, n int) []Repository {
    cp := append([]Repository(nil), in...)
    var src *rand.Rand
    if seed == 0 {
        src = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))
    } else {
        src = rand.New(rand.NewPCG(uint64(seed), 0))
    }
    src.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
    if n < len(cp) {
        cp = cp[:n]
    }
    return cp
}
```

## 4. Outputs

| Field | Type | Populated when |
|---|---|---|
| `Result.Random` | `[]plugins.Repository` | n > 0 && len(Featured) > 0 |

`Result.Random` は **`Result.Featured` のコピー**から作られる。Featured 自体は変更しない (immutability)。

## 5. Error model

このセクションは **network 不要**のため、エラー経路なし。preconditions に該当しない場合は単に空 / nil を返す。

## 6. Test cases

### TC-001: Deterministic with same seed
- Setup: `Featured = [r1, r2, r3, r4, r5]`, `random=3`, `seed=42`
- Expect: Run twice → same output (e.g. `[r3, r1, r5]`) both times

### TC-002: Different seed → different order
- Setup: same Featured, `random=3`, `seed=42` vs `seed=99`
- Expect: outputs differ

### TC-003: n > len(Featured)
- Setup: `Featured = [r1, r2]`, `random=5`
- Expect: `len(Random) == 2` (clamp), all Featured entries included

### TC-004: Empty Featured
- Setup: `Featured = []`, `random=3`
- Expect: `Random = []`

### TC-005: seed=0 → per-run fresh
- Setup: `random=3, seed=0`, run twice
- Expect: outputs may differ (uses `time.Now().UnixNano()`)

### TC-006: random=0 (機能 off)
- Setup: `Featured = [r1, r2, r3]`, `random=0`
- Expect: `Random = nil` (omitempty 経由で JSON から消える)
