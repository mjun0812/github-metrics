# Contract: GitHub API (HTTP / REST / GraphQL / Rate)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/httpx` および `internal/githubapi` が公開する API 契約を定義する。constitution 原則 IV「mocked 経路を通らない外部呼び出しは panic」を最重要不変条件とする。

## 1. `httpx.Client`

```go
package httpx

import (
    "context"
    "io"
    "net/http"
    "net/url"
)

type Client struct {
    inner     *retryablehttp.Client
    userAgent string
    logger    *slog.Logger
}

func New(opts Options) *Client

type Options struct {
    UserAgent  string         // e.g. "metrics/0.1.0 (+https://github.com/mjun0812/github-metrics)"
    Logger     *slog.Logger
    Transport  http.RoundTripper // 差し替え可。テスト時に mock を注入。
    MaxRetries int            // default: 4
    BaseDelay  time.Duration  // default: 1s
}

// Get issues an HTTP GET and returns the body bytes.
func (c *Client) Get(ctx context.Context, url string, header http.Header) ([]byte, *http.Response, error)

// PostJSON marshals body and posts as application/json.
func (c *Client) PostJSON(ctx context.Context, url string, body any, header http.Header) ([]byte, *http.Response, error)

// PostForm posts form-encoded values.
func (c *Client) PostForm(ctx context.Context, url string, values url.Values) ([]byte, *http.Response, error)

// Binary fetches binary payload (used by twemoji / image fetch in M3).
func (c *Client) Binary(ctx context.Context, url string) ([]byte, string /*mime*/, error)

// ImgB64 fetches an image and returns "data:image/<mime>;base64,..." string.
func (c *Client) ImgB64(ctx context.Context, url string) (string, error)
```

### 1.1 リトライポリシー (FR-013)

- 5xx / 429 / network エラー: 指数バックオフで最大 `MaxRetries` 回再試行 (default 4)。
- 4xx: 再試行 **しない**。
- `Retry-After` ヘッダがあれば値を尊重 (秒 / HTTP-date 形式)。
- context 経由 timeout / cancel は再試行しない (dispatcher 上位責務)。

### 1.2 ヘッダ

- 全リクエストに `User-Agent: metrics/<version> (+https://github.com/mjun0812/github-metrics)` を設定。
- 呼び出し側が `header` 引数で同名ヘッダを渡した場合、呼び出し側を優先 (テスト容易性)。

## 2. `githubapi.REST`

```go
package githubapi

type REST struct {
    client    *httpx.Client
    token     config.Token
    baseURL   string  // default https://api.github.com
}

func NewREST(token config.Token, customBaseURL string, opts httpx.Options) (*REST, error)

// Endpoint helpers (M1 では最小セット、M4 で拡張)
func (r *REST) RateLimit(ctx context.Context) (*RateLimitResponse, error)
func (r *REST) HeadRoot(ctx context.Context) (*http.Response, error) // X-OAuth-Scopes 確認用 (M6)

type RateLimitResponse struct {
    Resources Resources `json:"resources"`
    Rate      Quota     `json:"rate"`
}
```

### 2.1 認証 (FR-014)

token 種別判定は `internal/githubapi/auth.go::ClassifyToken(raw string) TokenKind` で行う。

| TokenKind | 検出条件 | 扱い |
|---|---|---|
| `TokenClassic` | `ghp_` / `gho_` / `ghu_` / `ghs_` / `ghr_` プレフィックス | 受理。`Authorization: token <raw>` |
| `TokenFineGrained` | `github_pat_` プレフィックス | **早期拒否** → `*InputError{Field: "token"}` |
| `TokenNone` | `NOT_NEEDED` | クライアントは 401 を期待。HTTP 通信は許容するが auth header なし。 |
| `TokenMocked` | `MOCKED_TOKEN` | mock RoundTripper を強制注入。実 URL ヒットで panic (§4)。 |
| `TokenUnknown` | 上記いずれにも該当しない | `*InputError{Field: "token"}` |

## 3. `githubapi.GraphQL`

```go
package githubapi

type GraphQL struct {
    client  graphql.Client // genqlient 生成インタフェース
    token   config.Token
    baseURL string         // default https://api.github.com/graphql
}

func NewGraphQL(token config.Token, customBaseURL string, opts httpx.Options) (*GraphQL, error)

// Generated functions (assets/plugins/base/queries/*.graphql から生成)
func (g *GraphQL) User(ctx context.Context, login string) (*UserResponse, error)
func (g *GraphQL) UserX(ctx context.Context, login string, fields []string) (*UserXResponse, error)
// ... base 系のみ M1 でカバー。M4 で plugin ごとに追加。
```

### 3.1 `genqlient` 設定

`genqlient.yaml`:

```yaml
schema: assets/plugins/base/schema.graphql   # GitHub GraphQL schema dump (固定 commit)
operations:
  - assets/plugins/base/queries/*.graphql
generated: internal/githubapi/graphql_gen.go
package: githubapi
optional: pointer
use_struct_references: true
```

- スキーマは固定 commit (`go generate ./...` で更新可)。
- operations の追加は新 plugin 実装 PR の中で同時に行う。

## 4. `MOCKED_TOKEN` パニックガード (NON-NEGOTIABLE)

`internal/githubapi` 内のすべての HTTP transport は、token が `TokenMocked` の場合に下記 RoundTripper でラップされる。

```go
type mockedGuardRoundTripper struct {
    inner http.RoundTripper // mock transport (M1 stub or M9 mocks.RESTMux)
}

func (t *mockedGuardRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    if isRealGitHubHost(req.URL.Host) {
        panic(fmt.Sprintf("MOCKED_TOKEN active but request hit real GitHub: %s", req.URL))
    }
    return t.inner.RoundTrip(req)
}

func isRealGitHubHost(host string) bool {
    return host == "api.github.com" || host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}
```

CI ジョブで panic は test failure として可視化される (SC-004)。

## 5. `Resources` (rate)

```go
type Resources struct {
    REST    Quota `json:"core"`     // GitHub の resources.core
    GraphQL Quota `json:"graphql"`
    Search  Quota `json:"search"`
    mu      sync.RWMutex
}

type Quota struct {
    Limit     int       `json:"limit"`
    Used      int       `json:"used"`
    Remaining int       `json:"remaining"`
    Reset     time.Time `json:"reset"`
}

// NewResources zeroes all quotas.
func NewResources() *Resources

// Refresh fetches /rate_limit and updates internal state atomically.
func (r *Resources) Refresh(ctx context.Context, c *REST) error

// Snapshot returns a read-only copy of the current state.
func (r *Resources) Snapshot() Resources
```

### 5.1 並行制約

- `Refresh` と `Snapshot` は race detector clean (R-012)。
- `Refresh` 失敗時は前回値を保持。エラーは戻り値で伝搬し、内部状態は変更しない。

## 6. テスト契約

- `internal/httpx/client_test.go`: `httptest.Server` で 503/200 シナリオを `assert.Eventually` 不要で同期テスト (FR-013, US3 AS1)。
- `internal/githubapi/rest_test.go`: `RateLimit` を mock RoundTripper でテスト (US3 AS2)。
- `internal/githubapi/graphql_test.go`: mock GraphQL transport で base `User` クエリの decode 成功を確認 (US3 AS3)。
- `internal/githubapi/auth_test.go`: `ClassifyToken` のテーブルテスト (US3 AS4)。
- `internal/githubapi/rate_test.go`: `Refresh` の並行アクセスを `go test -race` で検証 (US3 AS2)。
- M1 段階の最小 mock は `internal/githubapi/testhelper.go` に置き、M9 (T-118 / T-119) で `internal/testutil/mocks` に full 実装へ昇格。
