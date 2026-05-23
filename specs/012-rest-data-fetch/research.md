# Phase 0 Research: REST Data-Fetch Wiring

**Feature**: `012-rest-data-fetch` | **Date**: 2026-05-22

このドキュメントは Plan の Technical Context にあった「未確認」要素を埋めるための研究結果を集約する。spec.md / plan.md からのリンクで参照される。

---

## R-001: REST endpoint shape for `traffic`

**Decision**: GitHub REST API `GET /repos/{owner}/{repo}/traffic/views?per=day` を使用。Response は `{count: int, uniques: int, views: [{timestamp, count, uniques}]}` 形式。

**Rationale**:
- 上流 `org_repo/source/plugins/traffic/index.mjs:19` が `rest.repos.getViews({owner, repo})` を呼び出し、SDK ラッパは内部で本エンドポイントを叩く
- per-day breakdown 自体は upstream 描画では未使用 (count + uniques 集計のみ)。`per` パラメータは省略可 (default daily)
- 403 レスポンスは「token に repo scope なし」を示す。spec FR-003 で Skipped 扱い

**Alternatives considered**:
- `per=week` クエリ: count/uniques 集計値は同じだが、将来の per-time-series 拡張に備えて daily が無難
- GraphQL: traffic は GraphQL に exposed されていないため REST 一択

**Implementation note**: `pc.REST.Get(ctx, fmt.Sprintf("/repos/%s/traffic/views", repo.NameWithOwner))` ベースで decode。`type rawViews struct { Count int; Uniques int }` で十分。

---

## R-002: REST endpoint shape for `contributors`

**Decision**: `GET /repos/{owner}/{repo}/contributors?per_page=100&anon=false` を per-repo に並列実行、login をキーに `contributions` を集計する。

**Rationale**:
- Response 1 件あたり `{login, id, avatar_url, contributions}` の軽量フォーマット (per-page top 30 contributors、`per_page=100` でも 100 件上限)
- 上流 `repo-mode` は `rest.repos.listCommits` で per-commit author を集めるが、本 feature は user/org mode の「複数 repo の contributor login の総和」を出すだけなので contributor endpoint で十分
- anonymous contributors (`anon=true`) はメールアドレス出現する可能性があり PII リスクなので除外

**Alternatives considered**:
- `rest.repos.listCommits` per-repo paged loop: 上流 repo-mode 経路と同形だが ~100x API コール。spec の SC-002 (3+ Featured で ≥1 row) を達成するだけなら overkill
- GraphQL `defaultBranchRef.target.history`: 既存 `user_indepth.graphql` query に近いが per-repo commit 集計が contributor-aware ではない (author 情報の取り出しが多段)

**Implementation note**:
- 1 ページ目のみ取得 (top 100 contributors per repo)。pagination は不要 (アクティブ contributor は per-repo で十分少ない)
- `Result.List []Contributor{Login, AvatarURL, Commits, Additions=0, Deletions=0}` — additions/deletions は contributor endpoint では取れないので 0 で埋める。partial は `additions != 0 || deletions != 0` でガードしているため diff 行は emit されない

---

## R-003: REST endpoint shape for `repositories.Starred`

**Decision**: `GET /users/{login}/starred?per_page=100&sort=created&direction=desc` を `plugin_repositories_limit` まで取得。

**Rationale**:
- `sort=created direction=desc` で「最近 star した順」になり upstream `stars.ejs` の "Recently starred repositories" 表現と意味的に一致
- Response は `repository` オブジェクトのフラットなリスト (header `Accept: application/vnd.github.star+json` を渡せば `starred_at` も拾えるが、本 feature では不要)
- `plugin_repositories_limit` (default 8 per upstream) まで取れば十分

**Alternatives considered**:
- GraphQL `user.starredRepositories`: paged かつ cursor sorted。動くが本 feature の他 3 つが REST 経路なので transport を統一
- header `Accept: application/vnd.github.star+json` で starred_at を取得: stars plugin (別) では使うが、`repositories.Starred` 自体は star タイムスタンプを描画しないので不要

**Implementation note**:
- per_page=100 で 1 ページ取得後、結果を `plugin_repositories_limit` で truncate
- limit が 100 を超えるケースは upstream metadata で禁止 (max 100) — 想定不要

---

## R-004: Deterministic shuffle for `repositories.Random`

**Decision**: `math/rand/v2` の `*rand.Rand` (PCG-based) + `Shuffle` 関数を使用、seed は `plugin_repositories_random_seed` (int) を `*rand.PCG` の seed1/seed2 に投入。seed 未指定 (0) 時のみ `time.Now().UnixNano()`。

**Rationale**:
- `math/rand/v2` は Go 1.22+ stdlib で利用可能 (我々は 1.26.3、constitution §V 言語ポリシー準拠)
- PCG は spec FR-009 の reproducibility 要件 (同 seed → 同出力) を満たす
- `Shuffle` は in-place で `O(n)`、Featured のコピーに対して適用 (original を破壊しない)

**Alternatives considered**:
- 旧 `math/rand`: グローバル state を持ち並列で flakey。`math/rand/v2` の `*Rand` インスタンス分離が安全
- `crypto/rand`: deterministic seeding 不可 — spec FR-009 不適合

**Implementation note**:
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

---

## R-005: Concurrency + per-request timeout pattern

**Decision**: `golang.org/x/sync/errgroup` + `g.SetLimit(4)` + `context.WithTimeout(ctx, 30*time.Second)` を per-request に適用。

**Rationale**:
- `languages.indepth.Run` で既に確立済みのパターン (R-001 of 004 spec)
- `g.SetLimit(4)` で同時 in-flight 4 requests に絞れる
- per-request context は overall plugin context の child — overall timeout 到達時に全 outstanding work が cancel される

**Alternatives considered**:
- worker pool (semaphore channel): 同等だが errgroup の方が cancellation 伝播が綺麗
- 並列度 1 (sequential): 100 repos × 200ms latency = 20s — overall budget 圧迫

**Implementation note**:
```go
g, gctx := errgroup.WithContext(overallCtx)
g.SetLimit(4)
for _, repo := range repos {
    repo := repo
    g.Go(func() error {
        reqCtx, cancel := context.WithTimeout(gctx, 30*time.Second)
        defer cancel()
        v, err := fetchViews(reqCtx, repo)
        if err != nil {
            // Don't return — log + record on Data.Errors so other repos continue
            mu.Lock()
            errs = append(errs, fmt.Errorf("%s: %w", repo.NameWithOwner, err))
            mu.Unlock()
            return nil
        }
        mu.Lock()
        views[repo.NameWithOwner] = v
        mu.Unlock()
        return nil
    })
}
_ = g.Wait()
```

---

## R-006: REST mock fixture format for testing

**Decision**: 既存 `internal/testutil/mocks/rest_mux.go` の `RESTMux` を path pattern + response body の組で extend。`http.RoundTripper` の実装は維持。

**Rationale**:
- `tests/integration/plugins_p1_test.go` の `restEventsMux` パターンと同形 (constitution 原則 IV: 「外部 API は `internal/testutil/mocks` の RESTMux で差し替え MUST」)
- per-path body の lookup table を持てば fixture 追加コストが極小

**Alternatives considered**:
- `httptest.Server` で起動: テストごとに ephemeral port、speed と決定性が劣化
- `gomock` 風: code-gen 必要、constitution §V (依存最小化) に反する

**Implementation note**: 各 plugin のテストファイルが必要な fixture (`traffic/views.json`, `repos/.../contributors.json`, `users/.../starred.json`) を inline 文字列で持ち、`restMux` の Map に登録する。

---

## R-007: Token-scope detection

**Decision**: `pc.REST.Scopes(ctx)` を呼び、`hasScope(scopes, "repo")` で判定 (sponsors / projects と同パターン)。

**Rationale**:
- 既存 `internal/plugins/sponsors/sponsors.go` Run line 118-126 で確立済みのパターン
- `Scopes()` は `GET /user` 経由で `X-OAuth-Scopes` header をパースする (REST client 内部実装)
- error 時 (`Scopes` 自体が失敗) は Skipped にする方針 — 既存 sponsors と同じ

**Alternatives considered**:
- 全 plugin で都度呼ぶ vs Result キャッシュ: 既存パターンは都度呼び。1 plugin 1 Scope check で実害なし
- GraphQL `query { viewer { id } }` の error response を見る: 確実性低い

---

## 解決済 NEEDS CLARIFICATION 一覧

spec.md / plan.md に NEEDS CLARIFICATION マーカーは **0 件** だったため、本 phase で新規解決は不要。
すべてのデフォルト (concurrency=4 / timeout=30s / random seed semantics / contributor endpoint vs listCommits) は upstream + 既存内部パターンの踏襲で justify 済み。
