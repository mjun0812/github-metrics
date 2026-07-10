package dataprovider_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// countingTransport is a tiny http.RoundTripper that tallies hits
// keyed by the GraphQL operation name (extracted from the captured POST
// body). Each named operation can be configured with a fixed response
// body and status code.
type countingTransport struct {
	mu         sync.Mutex
	counts     map[string]*int64
	responses  map[string]string
	statusCode int
}

func newCountingTransport() *countingTransport {
	return &countingTransport{
		counts:    map[string]*int64{},
		responses: map[string]string{},
	}
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	op := operationName(string(body))
	c.mu.Lock()
	if _, ok := c.counts[op]; !ok {
		var n int64
		c.counts[op] = &n
	}
	counter := c.counts[op]
	respBody, hasResp := c.responses[op]
	status := c.statusCode
	c.mu.Unlock()
	atomic.AddInt64(counter, 1)
	if !hasResp {
		respBody = `{"data": null}`
	}
	if status == 0 {
		status = http.StatusOK
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Body:          io.NopCloser(strings.NewReader(respBody)),
		Header:        h,
		Request:       req,
		ContentLength: int64(len(respBody)),
	}, nil
}

// operationName extracts genqlient's "operationName" field from the
// JSON-encoded POST body. Genqlient always includes it so the helper
// works across all generated queries.
func operationName(body string) string {
	const marker = `"operationName":"`
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func (c *countingTransport) count(op string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.counts[op]; ok {
		return atomic.LoadInt64(v)
	}
	return 0
}

func (c *countingTransport) setResponse(op, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses[op] = body
}

func newProviderWith(t *testing.T, transport http.RoundTripper) *dataprovider.Provider {
	t.Helper()
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:  transport,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return dataprovider.New("octocat", "", gql, nil, nil, dataprovider.Options{})
}

// newProviderWithRepo is newProviderWith but in repository-template mode
// (M7): synthesizeRepoResult fires instead of the paging fetch, which
// the skip-private bypass test (TestProvider_SkipPrivate_RepoModeBypassesFilter)
// uses to confirm `--repo` wins over the global filter.
func newProviderWithRepo(t *testing.T, repo string, transport http.RoundTripper) *dataprovider.Provider {
	t.Helper()
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:  transport,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return dataprovider.New("octocat", repo, gql, nil, nil, dataprovider.Options{SkipPrivate: true})
}

// newProviderWithOpts is newProviderWith but with an explicit Options
// override, used by the skip-private filter tests.
func newProviderWithOpts(t *testing.T, transport http.RoundTripper, opts dataprovider.Options) *dataprovider.Provider {
	t.Helper()
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:  transport,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return dataprovider.New("octocat", "", gql, nil, nil, opts)
}

const userResponseBody = `{
  "data": {
    "user": {
      "databaseId": 1,
      "id": "U1",
      "login": "octocat",
      "name": "Octo",
      "location": "Earth",
      "createdAt": "2010-01-01T00:00:00Z",
      "avatarUrl": "https://example.invalid/avatar.png",
      "websiteUrl": "",
      "twitterUsername": null,
      "email": "",
      "bio": "",
      "company": ""
    }
  }
}`

const orgResponseBody = `{
  "data": {
    "organization": {
      "databaseId": 2,
      "id": "O1",
      "login": "octoorg",
      "name": "Octo Org",
      "location": "Earth",
      "description": "An org",
      "avatarUrl": "https://example.invalid/org.png",
      "websiteUrl": "",
      "twitterUsername": null,
      "email": ""
    }
  }
}`

func TestProvider_Profile_SingleflightCollapsesConcurrentCalls(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	p := newProviderWith(t, tr)

	const N = 100
	results := make([]*plugins.Profile, N)
	errs := make([]error, N)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = p.Profile(context.Background())
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if results[i] == nil {
			t.Fatalf("call %d: nil profile", i)
		}
	}
	if got := tr.count("User"); got != 1 {
		t.Fatalf("User operation hit %d times; want exactly 1 thanks to singleflight", got)
	}
}

func TestProvider_Profile_MemoizesAcrossCalls(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	p := newProviderWith(t, tr)

	first, err := p.Profile(context.Background())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := p.Profile(context.Background())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Fatalf("memoization broken: pointer identity differs")
	}
	if got := tr.count("User"); got != 1 {
		t.Fatalf("User hit %d times; expected 1 (second call should hit cache)", got)
	}
}

func TestProvider_Profile_ErrorIsCachedAndReplayed(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	// 400 is a non-retryable HTTP error so the underlying httpx client
	// does not loop, keeping the test fast. Both User and Organization
	// fail, so Profile surfaces an error which the cache-replay
	// assertion then checks does not retrigger any fetch.
	tr.statusCode = http.StatusBadRequest
	tr.setResponse("User", `{"errors":[{"message":"bad"}]}`)
	tr.setResponse("Organization", `{"errors":[{"message":"bad"}]}`)
	p := newProviderWith(t, tr)

	_, err1 := p.Profile(context.Background())
	if err1 == nil {
		t.Fatalf("expected error on first call")
	}
	_, err2 := p.Profile(context.Background())
	if err2 == nil {
		t.Fatalf("expected error on second call")
	}
	// Provider issued (at least) one User + one Organization call on
	// the first invocation; the second call must be served from cache
	// without re-issuing either operation.
	firstUser := tr.count("User")
	firstOrg := tr.count("Organization")
	if firstUser == 0 {
		t.Fatalf("User never called")
	}
	// The cache must replay the same error without any new fetches.
	_, _ = p.Profile(context.Background())
	if got := tr.count("User"); got != firstUser {
		t.Fatalf("User re-called: got %d, want %d (cached error must not retry)", got, firstUser)
	}
	if got := tr.count("Organization"); got != firstOrg {
		t.Fatalf("Organization re-called: got %d, want %d (cached error must not retry)", got, firstOrg)
	}
}

func TestProvider_User_RejectsOrganizationProfile(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	// User query returns no user (data.user is null), Organization
	// query returns the org payload so Profile resolves as an org.
	tr.setResponse("User", `{"data": {"user": null}}`)
	tr.setResponse("Organization", orgResponseBody)
	p := newProviderWith(t, tr)

	_, err := p.User(context.Background())
	if err == nil {
		t.Fatalf("expected ErrWrongAccountKind from User on org profile")
	}
	if !errors.Is(err, dataprovider.ErrWrongAccountKind) {
		t.Fatalf("expected ErrWrongAccountKind, got %v", err)
	}

	// Inverse: when profile is a user, Organization must reject it.
	tr2 := newCountingTransport()
	tr2.setResponse("User", userResponseBody)
	p2 := newProviderWith(t, tr2)
	_, err = p2.Organization(context.Background())
	if err == nil {
		t.Fatalf("expected ErrWrongAccountKind from Organization on user profile")
	}
	if !errors.Is(err, dataprovider.ErrWrongAccountKind) {
		t.Fatalf("expected ErrWrongAccountKind, got %v", err)
	}
}

// ctxAwareTransport returns ctx.Err() when the request's ctx is
// already canceled / past its deadline; otherwise it delegates to the
// embedded countingTransport. This lets us simulate the "caller A
// observed a context cancellation" scenario without depending on real
// network timing.
type ctxAwareTransport struct {
	inner *countingTransport
}

func (c *ctxAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		// Mirror net/http's behaviour when a transport observes a
		// canceled ctx: surface ctx.Err() so callers can detect it
		// via errors.Is(err, context.Canceled).
		return nil, err
	}
	return c.inner.RoundTrip(req)
}

func TestProvider_Profile_DoesNotCacheContextCanceled(t *testing.T) {
	t.Parallel()
	inner := newCountingTransport()
	inner.setResponse("User", userResponseBody)
	tr := &ctxAwareTransport{inner: inner}
	p := newProviderWith(t, tr)

	// Caller A: pre-cancel the ctx, then call Profile. The transport
	// surfaces context.Canceled; fetchProfile wraps it. memoize must
	// NOT store the error so a later caller with a fresh ctx can
	// re-enter the fetch.
	ctxA, cancelA := context.WithCancel(context.Background())
	cancelA()
	_, errA := p.Profile(ctxA)
	if errA == nil {
		t.Fatalf("caller A: expected context.Canceled-derived error, got nil")
	}
	if !errors.Is(errA, context.Canceled) {
		t.Fatalf("caller A: expected errors.Is(context.Canceled), got %v", errA)
	}

	// Caller B: fresh ctx. If the cache poisoned the result, B would
	// observe errA replayed without any new RoundTrip. Instead, B
	// must trigger a fresh fetch and observe success.
	prof, errB := p.Profile(context.Background())
	if errB != nil {
		t.Fatalf("caller B: expected fresh fetch to succeed, got %v", errB)
	}
	if prof == nil {
		t.Fatalf("caller B: expected non-nil profile after retry")
	}
	if prof.Kind != plugins.ProfileKindUser || prof.User == nil {
		t.Fatalf("caller B: expected user profile, got kind=%q user=%v", prof.Kind, prof.User)
	}

	// Caller B's success MUST have hit the transport at least once
	// (the User op count is the load-bearing assertion: zero would
	// mean B was served the cached error).
	if got := inner.count("User"); got == 0 {
		t.Fatalf("User op never reached transport on caller B; cache replayed the canceled error")
	}
}

func TestProvider_Profile_DoesNotCacheContextDeadlineExceeded(t *testing.T) {
	t.Parallel()
	inner := newCountingTransport()
	inner.setResponse("User", userResponseBody)
	tr := &ctxAwareTransport{inner: inner}
	p := newProviderWith(t, tr)

	// Caller A: deadline already in the past, so the transport surfaces
	// context.DeadlineExceeded. memoize must NOT cache this error — it
	// is the partner branch of the context.Canceled carve-out and a
	// regression here would silently re-introduce request-wide cache
	// poisoning whenever any single plugin hits its timeout.
	ctxA, cancelA := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelA()
	_, errA := p.Profile(ctxA)
	if errA == nil {
		t.Fatalf("caller A: expected context.DeadlineExceeded-derived error, got nil")
	}
	if !errors.Is(errA, context.DeadlineExceeded) {
		t.Fatalf("caller A: expected errors.Is(context.DeadlineExceeded), got %v", errA)
	}

	// Caller B: fresh ctx. If the cache poisoned the result, B would
	// observe errA replayed without any new RoundTrip. Instead, B must
	// trigger a fresh fetch and observe success.
	prof, errB := p.Profile(context.Background())
	if errB != nil {
		t.Fatalf("caller B: expected fresh fetch to succeed, got %v", errB)
	}
	if prof == nil {
		t.Fatalf("caller B: expected non-nil profile after retry")
	}
	if prof.Kind != plugins.ProfileKindUser || prof.User == nil {
		t.Fatalf("caller B: expected user profile, got kind=%q user=%v", prof.Kind, prof.User)
	}

	if got := inner.count("User"); got == 0 {
		t.Fatalf("User op never reached transport on caller B; cache replayed the deadline-exceeded error")
	}
}

// userRepositoriesResponseBody is a minimal two-repo paging fixture for
// the UserRepositories operation: one repo with issues=5 / PRs=3, one
// with issues=2 / PRs=7. Both branches of fetchOneRepoPage must accumulate
// these per-node totals into ComputedRepositories.Issues / .PullRequests
// (otherwise the JSON wire keys computed.repositories.issues /
// .pullRequests stay 0 — the wire-format regression the deleted base
// plugin used to prevent via its UserIndepth-based hydration).
const userRepositoriesResponseBody = `{
  "data": {
    "user": {
      "repositories": {
        "totalCount": 2,
        "pageInfo": {"hasNextPage": false, "endCursor": null},
        "nodes": [
          {
            "databaseId": 1,
            "id": "R1",
            "name": "alpha",
            "nameWithOwner": "octocat/alpha",
            "description": null,
            "url": "https://example.invalid/alpha",
            "isPrivate": false,
            "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 10,
            "forkCount": 1,
            "issues": {"totalCount": 5},
            "pullRequests": {"totalCount": 3},
            "watchers": {"totalCount": 4},
            "primaryLanguage": null,
            "languages": null,
            "diskUsage": 100,
            "releases": {"totalCount": 1},
            "packages": {"totalCount": 0},
            "deployments": {"totalCount": 2},
            "licenseInfo": null
          },
          {
            "databaseId": 2,
            "id": "R2",
            "name": "beta",
            "nameWithOwner": "octocat/beta",
            "description": null,
            "url": "https://example.invalid/beta",
            "isPrivate": false,
            "isFork": false,
            "createdAt": "2021-01-01T00:00:00Z",
            "pushedAt": "2021-01-02T00:00:00Z",
            "updatedAt": "2021-01-02T00:00:00Z",
            "stargazerCount": 20,
            "forkCount": 0,
            "issues": {"totalCount": 2},
            "pullRequests": {"totalCount": 7},
            "watchers": {"totalCount": 6},
            "primaryLanguage": null,
            "languages": null,
            "diskUsage": 200,
            "releases": {"totalCount": 0},
            "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0},
            "licenseInfo": null
          }
        ]
      }
    }
  }
}`

// TestProvider_RepositorySummary_IncludesIssuesAndPullRequests guards the
// JSON wire format keys computed.repositories.issues /
// computed.repositories.pullRequests, which the deleted base plugin used
// to populate (via its UserIndepth hydration into
// pc.Data.Computed.Repositories.{Issues,PullRequests}). After base's
// removal RepositorySummary is the sole producer of those keys, so the
// paging accumulator must sum the per-node issues.totalCount /
// pullRequests.totalCount that the UserRepositories /
// OrganizationRepositories GraphQL queries already return.
func TestProvider_RepositorySummary_IncludesIssuesAndPullRequests(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	tr.setResponse("UserRepositories", userRepositoriesResponseBody)
	p := newProviderWith(t, tr)

	summary, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("RepositorySummary returned nil summary")
	}
	if got, want := summary.Issues, 7; got != want {
		t.Errorf("Issues: got %d, want %d (sum of per-node issues.totalCount)", got, want)
	}
	if got, want := summary.PullRequests, 10; got != want {
		t.Errorf("PullRequests: got %d, want %d (sum of per-node pullRequests.totalCount)", got, want)
	}
}

// userRepositoriesForkedResponseBody is a two-repo fixture where the
// second node is a fork. Used by TestProvider_RepositorySummary_Forked
// to anchor the ComputedRepositories.Forked accumulator that plugin_base
// (#625) reads to render the "<N> Repositories (including <F> forks)"
// heading. Both nodes carry the same shape as userRepositoriesResponseBody
// to avoid drifting away from the production fetchOneRepoPage decoder.
const userRepositoriesForkedResponseBody = `{
  "data": {
    "user": {
      "repositories": {
        "totalCount": 2,
        "pageInfo": {"hasNextPage": false, "endCursor": null},
        "nodes": [
          {
            "databaseId": 1, "id": "R1", "name": "alpha",
            "nameWithOwner": "octocat/alpha", "description": null,
            "url": "https://example.invalid/alpha",
            "isPrivate": false, "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 0, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          },
          {
            "databaseId": 2, "id": "R2", "name": "beta",
            "nameWithOwner": "octocat/beta", "description": null,
            "url": "https://example.invalid/beta",
            "isPrivate": false, "isFork": true,
            "createdAt": "2021-01-01T00:00:00Z",
            "pushedAt": "2021-01-02T00:00:00Z",
            "updatedAt": "2021-01-02T00:00:00Z",
            "stargazerCount": 0, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          }
        ]
      }
    }
  }
}`

// TestProvider_RepositorySummary_Forked guards the
// ComputedRepositories.Forked accumulator added in #625. plugin_base's
// RepositoriesPartial renders the heading "<N> Repositories (including
// <F> forks)" — if the per-node node.isFork is not summed during paging
// the bracketed clause disappears and the wire-format key
// computed.repositories.forked stays 0 on otherwise valid responses.
func TestProvider_RepositorySummary_Forked(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	tr.setResponse("UserRepositories", userRepositoriesForkedResponseBody)
	p := newProviderWith(t, tr)

	summary, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("RepositorySummary returned nil summary")
	}
	if got, want := summary.Forked, 1; got != want {
		t.Errorf("Forked: got %d, want %d (count of nodes with isFork=true)", got, want)
	}
	if got, want := summary.Count, 2; got != want {
		t.Errorf("Count: got %d, want %d (total nodes including forks)", got, want)
	}

	// Second call exercises the singleflight memoization path: must
	// return the same Forked total without re-fetching.
	again, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary (memoized): %v", err)
	}
	if again.Forked != summary.Forked {
		t.Errorf("memoized Forked drifted: got %d, want %d", again.Forked, summary.Forked)
	}
}

// userRepositoriesSkipPrivateFixture is a four-node paging fixture
// (2 public + 2 private) used by the repositories_skip_private (#656)
// tests. Each node carries a distinct stargazerCount so the filter's
// effect on the aggregate (RepositorySummary.Stargazers) is observable
// and not just a list-length comparison.
const userRepositoriesSkipPrivateFixture = `{
  "data": {
    "user": {
      "repositories": {
        "totalCount": 4,
        "pageInfo": {"hasNextPage": false, "endCursor": null},
        "nodes": [
          {
            "databaseId": 1, "id": "R1", "name": "pub-a",
            "nameWithOwner": "octocat/pub-a", "description": null,
            "url": "https://example.invalid/pub-a",
            "isPrivate": false, "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 10, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          },
          {
            "databaseId": 2, "id": "R2", "name": "priv-a",
            "nameWithOwner": "octocat/priv-a", "description": null,
            "url": "https://example.invalid/priv-a",
            "isPrivate": true, "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 100, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          },
          {
            "databaseId": 3, "id": "R3", "name": "pub-b",
            "nameWithOwner": "octocat/pub-b", "description": null,
            "url": "https://example.invalid/pub-b",
            "isPrivate": false, "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 20, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          },
          {
            "databaseId": 4, "id": "R4", "name": "priv-b",
            "nameWithOwner": "octocat/priv-b", "description": null,
            "url": "https://example.invalid/priv-b",
            "isPrivate": true, "isFork": false,
            "createdAt": "2020-01-01T00:00:00Z",
            "pushedAt": "2020-01-02T00:00:00Z",
            "updatedAt": "2020-01-02T00:00:00Z",
            "stargazerCount": 200, "forkCount": 0,
            "issues": {"totalCount": 0}, "pullRequests": {"totalCount": 0},
            "watchers": {"totalCount": 0}, "primaryLanguage": null,
            "languages": null, "diskUsage": 0,
            "releases": {"totalCount": 0}, "packages": {"totalCount": 0},
            "deployments": {"totalCount": 0}, "licenseInfo": null
          }
        ]
      }
    }
  }
}`

// TestProvider_SkipPrivate_FiltersFromList guards the
// repositories_skip_private (#656) acceptance: when the flag is on, the
// account-wide paging fetch drops isPrivate nodes before they reach the
// repoPagingState accumulator, so both Repositories() and the aggregates
// in RepositorySummary() reflect the public-only subset.
func TestProvider_SkipPrivate_FiltersFromList(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	tr.setResponse("UserRepositories", userRepositoriesSkipPrivateFixture)
	p := newProviderWithOpts(t, tr, dataprovider.Options{SkipPrivate: true})

	repos, err := p.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories: %v", err)
	}
	if got, want := len(repos), 2; got != want {
		t.Fatalf("len(Repositories) = %d, want %d (private nodes must be dropped)", got, want)
	}
	for _, r := range repos {
		if strings.Contains(r.NameWithOwner, "priv-") {
			t.Errorf("private repo %q leaked into Repositories()", r.NameWithOwner)
		}
	}

	summary, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("RepositorySummary returned nil")
	}
	if got, want := summary.Count, 2; got != want {
		t.Errorf("Count = %d, want %d (visible-only count)", got, want)
	}
	// Stargazers spot-check: 10 (pub-a) + 20 (pub-b) = 30. The
	// private nodes contribute 100 + 200 = 300 and MUST be absent.
	if got, want := summary.Stargazers, 30; got != want {
		t.Errorf("Stargazers = %d, want %d (private stars must be excluded)", got, want)
	}
}

// TestProvider_SkipPrivate_DefaultOffIncludesPrivate is the negative
// control: with the flag absent the existing behavior is preserved —
// every node (public + private) appears in the list and aggregates.
func TestProvider_SkipPrivate_DefaultOffIncludesPrivate(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("User", userResponseBody)
	tr.setResponse("UserRepositories", userRepositoriesSkipPrivateFixture)
	p := newProviderWithOpts(t, tr, dataprovider.Options{}) // flag off

	repos, err := p.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories: %v", err)
	}
	if got, want := len(repos), 4; got != want {
		t.Fatalf("len(Repositories) = %d, want %d (flag off: all 4 nodes pass through)", got, want)
	}

	summary, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary: %v", err)
	}
	if got, want := summary.Count, 4; got != want {
		t.Errorf("Count = %d, want %d (server-side totalCount)", got, want)
	}
	// 10 + 100 + 20 + 200 = 330 — every node contributes.
	if got, want := summary.Stargazers, 330; got != want {
		t.Errorf("Stargazers = %d, want %d (no nodes filtered out)", got, want)
	}
}

// repositorySecretResponseBody is a single-repo Repository operation
// response simulating an explicitly-requested private repo. The
// repo-mode bypass test pairs it with SkipPrivate: true to confirm the
// `--repo` argument wins over the global filter (synthesizeRepoResult
// never consults p.skipPrivate).
const repositorySecretResponseBody = `{
  "data": {
    "repository": {
      "databaseId": 9001,
      "name": "secret",
      "nameWithOwner": "octocat/secret",
      "description": "private repo, explicitly requested",
      "createdAt": "2020-01-01T00:00:00Z",
      "diskUsage": 0,
      "stargazerCount": 7,
      "forkCount": 0,
      "isArchived": false,
      "watchers": {"totalCount": 0},
      "languages": null,
      "deployments": {"totalCount": 0},
      "environments": {"totalCount": 0},
      "primaryLanguage": null,
      "licenseInfo": null,
      "defaultBranchRef": {"name": "main"},
      "owner": {"__typename": "User", "login": "octocat", "avatarUrl": "https://example.invalid/avatar.png"},
      "issues": {"totalCount": 0},
      "pullRequests": {"totalCount": 0}
    }
  }
}`

// flakyRepoTransport fails the first `fails` UserRepositories round-trips
// with a transient (503) error, then delegates to inner. fails < 0 fails
// every UserRepositories call. Non-repository operations (e.g. the User
// profile lookup) always pass through to inner so Profile resolves. It
// backs the batch=1 degraded-path retry regression tests (#748): the
// transient error must reach fetchRepoResult's batch-halving loop, which
// requires the HTTP layer to NOT swallow it via its own retries — hence
// newProviderNoHTTPRetry below disables the retryablehttp loop.
type flakyRepoTransport struct {
	inner *countingTransport
	mu    sync.Mutex
	fails int
}

func (f *flakyRepoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	if operationName(string(body)) == "UserRepositories" {
		f.mu.Lock()
		if f.fails != 0 {
			if f.fails > 0 {
				f.fails--
			}
			f.mu.Unlock()
			return nil, errors.New("503 Service Unavailable")
		}
		f.mu.Unlock()
	}
	return f.inner.RoundTrip(req)
}

// newProviderNoHTTPRetry builds a Provider whose HTTP client performs no
// retries (DisableRetries), so a transient error surfaces to
// fetchRepoResult on the first attempt instead of being absorbed by the
// retryablehttp loop. This isolates the dataprovider-level batch-halving
// retry logic under test.
func newProviderNoHTTPRetry(t *testing.T, transport http.RoundTripper) *dataprovider.Provider {
	t.Helper()
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:      transport,
		DisableRetries: true,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return dataprovider.New("octocat", "", gql, nil, nil, dataprovider.Options{})
}

// TestProvider_RepoResult_BatchOneRecoversAfterHalvingPhase is the #748
// regression: transient failures run continuously through the entire
// batch-halving phase (100→50→25→12→6→3→1 = 6 failures) and then one
// more at batch=1 before the fetch finally succeeds. Before the fix the
// halving phase and the batch=1 phase shared one counter, so the six
// halving failures alone exhausted maxConsecutiveAttempts and the first
// batch=1 failure errored out — the batch=1 success below was never
// reached. With batch=1 owning its own retry budget the loop rides out
// the transient burst and returns the repositories.
func TestProvider_RepoResult_BatchOneRecoversAfterHalvingPhase(t *testing.T) {
	t.Parallel()
	inner := newCountingTransport()
	inner.setResponse("User", userResponseBody)
	inner.setResponse("UserRepositories", userRepositoriesResponseBody)
	// 6 halving failures + 1 batch=1 failure, then success.
	tr := &flakyRepoTransport{inner: inner, fails: 7}
	p := newProviderNoHTTPRetry(t, tr)

	repos, err := p.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories: expected recovery after transient burst, got %v", err)
	}
	if got, want := len(repos), 2; got != want {
		t.Fatalf("len(Repositories) = %d, want %d (both nodes accumulated on the successful batch=1 page)", got, want)
	}
}

// TestProvider_RepoResult_BatchOneRetriesMultipleTimes exercises the
// batch=1 retry budget directly: after the halving phase, batch=1 fails
// three consecutive times and then succeeds. Each batch=1 failure must
// consume one unit of the dedicated budget (which resets on the eventual
// success), not error out immediately.
func TestProvider_RepoResult_BatchOneRetriesMultipleTimes(t *testing.T) {
	t.Parallel()
	inner := newCountingTransport()
	inner.setResponse("User", userResponseBody)
	inner.setResponse("UserRepositories", userRepositoriesResponseBody)
	// 6 halving failures + 3 batch=1 failures, then success.
	tr := &flakyRepoTransport{inner: inner, fails: 9}
	p := newProviderNoHTTPRetry(t, tr)

	repos, err := p.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories: expected recovery after 3 batch=1 retries, got %v", err)
	}
	if got, want := len(repos), 2; got != want {
		t.Fatalf("len(Repositories) = %d, want %d", got, want)
	}
}

// TestProvider_RepoResult_BatchOneExhaustsBudget locks the error contract
// when batch=1 never recovers: the message must report the actual number
// of batch=1 retries (6), not the shared counter's inflated total.
func TestProvider_RepoResult_BatchOneExhaustsBudget(t *testing.T) {
	t.Parallel()
	inner := newCountingTransport()
	inner.setResponse("User", userResponseBody)
	inner.setResponse("UserRepositories", userRepositoriesResponseBody)
	// fails < 0: every UserRepositories call fails transiently forever.
	tr := &flakyRepoTransport{inner: inner, fails: -1}
	p := newProviderNoHTTPRetry(t, tr)

	_, err := p.Repositories(context.Background())
	if err == nil {
		t.Fatal("Repositories: expected error when batch=1 never recovers")
	}
	if !strings.Contains(err.Error(), "batch=1 failed after 6 retries") {
		t.Fatalf("error = %q, want it to report 6 batch=1 retries", err.Error())
	}
}

// TestProvider_SkipPrivate_RepoModeBypassesFilter guards the carve-out
// from the issue's Acceptance ("Repository-mode bypasses the filter —
// the user explicitly chose this repo"). synthesizeRepoResult is the
// only producer in repo-mode; it does not — and must not — consult
// p.skipPrivate. Even with SkipPrivate=true and a repo that would be
// dropped by the account-wide paging filter, the synthesized list still
// contains the named repo.
func TestProvider_SkipPrivate_RepoModeBypassesFilter(t *testing.T) {
	t.Parallel()
	tr := newCountingTransport()
	tr.setResponse("Repository", repositorySecretResponseBody)
	p := newProviderWithRepo(t, "secret", tr)

	repos, err := p.Repositories(context.Background())
	if err != nil {
		t.Fatalf("Repositories: %v", err)
	}
	if got, want := len(repos), 1; got != want {
		t.Fatalf("len(Repositories) = %d, want %d (repo-mode must bypass skipPrivate)", got, want)
	}
	if got, want := repos[0].NameWithOwner, "octocat/secret"; got != want {
		t.Errorf("NameWithOwner = %q, want %q", got, want)
	}

	summary, err := p.RepositorySummary(context.Background())
	if err != nil {
		t.Fatalf("RepositorySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("RepositorySummary returned nil in repo-mode")
	}
	if got, want := summary.Count, 1; got != want {
		t.Errorf("Count = %d, want %d (single synthesized repo)", got, want)
	}
}
