package githubapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMockedGuard_PanicsOnRealGitHubHost is the cornerstone of FR-017:
// when MOCKED_TOKEN is in play, any request that targets a production
// host MUST fail loudly so the offending test cannot accidentally hit
// the real API.
func TestMockedGuard_PanicsOnRealGitHubHost(t *testing.T) {
	t.Parallel()

	cases := []string{
		"https://api.github.com/user",
		"https://github.com/octocat",
		"https://raw.githubusercontent.com/octocat/Hello-World/master/README",
		"https://api.github.com:443/rate_limit",
		"https://API.GITHUB.COM/foo", // case-insensitive
	}
	for _, target := range cases {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			guard := newMockedGuard(nil) // inner nil; panic should fire before it is consulted
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic for %s, got none", target)
				}
			}()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			_, _ = guard.RoundTrip(req)
		})
	}
}

func TestMockedGuard_AllowsLocalhostThroughInner(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()

	// Use the test server's own transport so the guard forwards to it.
	guard := newMockedGuard(srv.Client().Transport)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := guard.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip on localhost should not error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestMockedGuard_NilInnerErrorsCleanlyForMockHosts(t *testing.T) {
	t.Parallel()

	guard := newMockedGuard(nil)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://mock.localhost/x", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := guard.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("expected error when inner transport is nil")
	}
}

func TestIsRealGitHubHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		want bool
	}{
		{"api.github.com", true},
		{"github.com", true},
		{"raw.githubusercontent.com", true},
		{"avatars.githubusercontent.com", true},
		{"objects.githubusercontent.com", true},
		{"github.io", false},
		{"localhost", false},
		{"127.0.0.1", false},
		{"example.com", false},
		{"API.GITHUB.COM", true},
		{"api.github.com:443", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			t.Parallel()
			if got := IsRealGitHubHost(tc.host); got != tc.want {
				t.Fatalf("IsRealGitHubHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
