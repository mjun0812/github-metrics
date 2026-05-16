package githubapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Scopes returns the OAuth scopes granted to the configured token, as
// reported by the GitHub API's X-OAuth-Scopes response header on a
// `GET /` request.
//
// The first call issues the HTTP probe and caches the result; subsequent
// calls return the cached slice without re-hitting the network. An empty
// slice (with nil error) means the token has no scopes — for example an
// unauthenticated request or a token whose grant carries no scope
// header. Plugins that gate behavior on a particular scope (projects,
// sponsors, traffic) can therefore call this helper without worrying
// about request amplification.
//
// On HTTP-level failure (network error, non-2xx status), the error is
// surfaced verbatim and nothing is cached, so callers can retry on the
// next request.
func (r *REST) Scopes(ctx context.Context) ([]string, error) {
	r.scopesMu.Lock()
	defer r.scopesMu.Unlock()

	if r.scopesCached {
		// Return a copy so callers cannot mutate the cached slice.
		out := make([]string, len(r.scopes))
		copy(out, r.scopes)
		return out, nil
	}

	resp, err := r.HeadRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("scopes: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("scopes: nil response")
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scopes: status %d", resp.StatusCode)
	}

	r.scopes = parseScopesHeader(resp.Header)
	r.scopesCached = true
	out := make([]string, len(r.scopes))
	copy(out, r.scopes)
	return out, nil
}

// parseScopesHeader splits the comma-separated X-OAuth-Scopes value into
// a clean slice. The empty header maps to an empty slice (not nil) so
// equality comparisons in tests stay predictable.
func parseScopesHeader(h http.Header) []string {
	raw := h.Get("X-OAuth-Scopes")
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
