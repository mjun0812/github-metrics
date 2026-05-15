package githubapi

import (
	"fmt"
	"net/http"
	"strings"
)

// mockedGuardRoundTripper wraps an inner transport and panics when it
// observes a request bound for a real GitHub host. It is installed
// whenever the active credential classifies as TokenMocked so a test
// that forgets to plumb a fixture never escapes to the public network
// (constitution principle IV; FR-017).
type mockedGuardRoundTripper struct {
	inner http.RoundTripper
}

// newMockedGuard wraps inner. inner must be non-nil; passing nil
// returns a guard that always panics, which is what we want when
// callers forget to install a mock backend.
func newMockedGuard(inner http.RoundTripper) http.RoundTripper {
	return &mockedGuardRoundTripper{inner: inner}
}

// RoundTrip enforces the panic gate. The error return is never reached
// for the real-host case; we panic so even Go's recover-less callers
// surface the violation as a failed test.
func (t *mockedGuardRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("mocked guard: nil request or URL")
	}
	if IsRealGitHubHost(req.URL.Host) {
		panic(fmt.Sprintf(
			"MOCKED_TOKEN active but request hit real GitHub host: %s %s",
			req.Method, req.URL.String(),
		))
	}
	if t.inner == nil {
		return nil, fmt.Errorf("mocked guard: no inner transport configured (test missing a fixture for %s %s)",
			req.Method, req.URL.String())
	}
	return t.inner.RoundTrip(req)
}

// IsRealGitHubHost reports whether host names a production GitHub
// endpoint. Exported so callers (including this package's tests) can
// run the same predicate without re-implementing it.
//
// The match is case-insensitive and considers:
//   - api.github.com
//   - github.com
//   - *.githubusercontent.com (raw, avatars, codeload, ...)
func IsRealGitHubHost(host string) bool {
	host = strings.ToLower(host)
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if host == "api.github.com" || host == "github.com" {
		return true
	}
	if strings.HasSuffix(host, ".githubusercontent.com") {
		return true
	}
	return false
}
