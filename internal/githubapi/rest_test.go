package githubapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

func newRESTWithMock(t *testing.T, mock *githubapi.MockTransport) *githubapi.REST {
	t.Helper()
	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaaa"), "", httpx.Options{
		Transport:  mock,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return rest
}

func TestNewREST_RejectsFineGrainedToken(t *testing.T) {
	t.Parallel()

	_, err := githubapi.NewREST(config.NewToken("github_pat_xxx"), "", httpx.Options{})
	if err == nil {
		t.Fatalf("expected error for fine-grained token")
	}
	var ie *xerrors.InputError
	if !xerrors.As(err, &ie) {
		t.Fatalf("expected *InputError, got %T", err)
	}
}

func TestNewREST_AcceptsClassicTokenAndSetsAuthHeader(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", `{
		"resources": {
			"core":    {"limit":5000,"used":1,"remaining":4999,"reset":1700000000},
			"graphql": {"limit":5000,"used":0,"remaining":5000,"reset":1700000000},
			"search":  {"limit":30,"used":0,"remaining":30,"reset":1700000000}
		},
		"rate": {"limit":5000,"used":1,"remaining":4999,"reset":1700000000}
	}`)

	rest := newRESTWithMock(t, mock)
	if rest.TokenKind() != githubapi.TokenClassic {
		t.Fatalf("TokenKind = %v, want TokenClassic", rest.TokenKind())
	}

	rl, err := rest.RateLimit(context.Background())
	if err != nil {
		t.Fatalf("RateLimit: %v", err)
	}
	if rl.Resources.Core.Limit != 5000 {
		t.Errorf("Core.Limit = %d", rl.Resources.Core.Limit)
	}
	if !rl.Resources.Core.Reset.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Errorf("Core.Reset = %v", rl.Resources.Core.Reset)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "/rate_limit" {
		t.Errorf("path = %q", calls[0].Path)
	}
	auth := calls[0].Header.Get("Authorization")
	if auth != "token ghp_aaaaa" {
		t.Errorf("Authorization header = %q", auth)
	}
	if calls[0].Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
		t.Errorf("missing X-GitHub-Api-Version header")
	}
}

func TestNewREST_NotNeededOmitsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", `{"resources":{"core":{"limit":60,"used":0,"remaining":60,"reset":0},"graphql":{"limit":0,"used":0,"remaining":0,"reset":0},"search":{"limit":10,"used":0,"remaining":10,"reset":0}},"rate":{"limit":60,"used":0,"remaining":60,"reset":0}}`)

	rest, err := githubapi.NewREST(config.NewToken("NOT_NEEDED"), "", httpx.Options{Transport: mock, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	if _, err := rest.RateLimit(context.Background()); err != nil {
		t.Fatalf("RateLimit: %v", err)
	}
	calls := mock.Calls()
	if calls[0].Header.Get("Authorization") != "" {
		t.Errorf("NOT_NEEDED should not send Authorization header")
	}
}

func TestNewREST_MockedTokenPanicsOnRealGitHub(t *testing.T) {
	t.Parallel()

	// inner transport explicitly nil; the guard must panic before consulting it.
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"https://api.github.com", // real production URL
		httpx.Options{MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for MOCKED_TOKEN to real host")
		} else if s, ok := r.(string); !ok || !strings.Contains(s, "MOCKED_TOKEN") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	_, _ = rest.RateLimit(context.Background())
}

func TestREST_RateLimit_ErrorOnNon200(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.Set("GET", "/rate_limit", githubapi.MockResponse{Status: http.StatusForbidden, Body: []byte(`{"message":"nope"}`)})

	rest := newRESTWithMock(t, mock)
	if _, err := rest.RateLimit(context.Background()); err == nil {
		t.Fatalf("expected error on 403")
	}
}

func TestREST_BaseURLOverride(t *testing.T) {
	t.Parallel()

	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaa"), "https://example.invalid/api/v3", httpx.Options{})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	if rest.BaseURL() != "https://example.invalid/api/v3" {
		t.Fatalf("BaseURL = %q", rest.BaseURL())
	}
}

func TestREST_HeadRoot(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	h := http.Header{}
	h.Set("X-OAuth-Scopes", "repo, read:org")
	mock.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})

	rest := newRESTWithMock(t, mock)
	resp, err := rest.HeadRoot(context.Background())
	if err != nil {
		t.Fatalf("HeadRoot: %v", err)
	}
	if got := resp.Header.Get("X-OAuth-Scopes"); got != "repo, read:org" {
		t.Errorf("X-OAuth-Scopes = %q", got)
	}
}
