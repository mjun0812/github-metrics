package action

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// restMock is a tiny RoundTripper for token validator tests. Routes
// requests by URL path to a canned response.
type restMock struct {
	mu       sync.Mutex
	scopes   string // X-OAuth-Scopes header for HEAD /
	rateBody string // GET /rate_limit body
	rateCode int    // GET /rate_limit status (0 → 200)
}

func (m *restMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := http.Header{}
	if req.URL.Path == "/" {
		h.Set("X-OAuth-Scopes", m.scopes)
		return mkResp(req, http.StatusOK, h, ""), nil
	}
	if strings.HasSuffix(req.URL.Path, "/rate_limit") {
		status := m.rateCode
		if status == 0 {
			status = http.StatusOK
		}
		return mkResp(req, status, h, m.rateBody), nil
	}
	return mkResp(req, http.StatusNotFound, h, "{}"), nil
}

func mkResp(req *http.Request, status int, h http.Header, body string) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func newREST(t *testing.T, mock http.RoundTripper) *githubapi.REST {
	t.Helper()
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mock, DisableRetries: true},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return rest
}

func TestValidator_GithubPatRejected(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{Token: config.NewToken("github_pat_xxx")}
	_, err := v.Validate(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var ie *InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *InputError", err)
	}
	if !strings.Contains(ie.Msg, "github_pat_") {
		t.Errorf("message should mention github_pat_ rejection; got %q", ie.Msg)
	}
}

func TestValidator_TokenMissing_NoMock_Rejected(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{Token: config.NewToken(""), UseMockedData: false}
	_, err := v.Validate(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var ie *InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *InputError", err)
	}
}

func TestValidator_TokenMissing_WithMock_Passes(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{Token: config.NewToken(""), UseMockedData: true}
	got, err := v.Validate(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !got.QuotaSufficient {
		t.Errorf("QuotaSufficient = false, want true for mocked path")
	}
}

func TestValidator_MockedToken_Passes(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{Token: config.NewToken("MOCKED_TOKEN")}
	got, err := v.Validate(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !got.QuotaSufficient {
		t.Errorf("QuotaSufficient = false, want true for MOCKED_TOKEN")
	}
}

func TestValidator_ScopeMissing_Warning(t *testing.T) {
	t.Parallel()
	mock := &restMock{
		scopes:   "repo",
		rateBody: `{"resources":{"core":{"remaining":5000,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`,
	}
	v := &TokenValidator{
		Token:          config.NewToken("ghp_valid"),
		REST:           newREST(t, mock),
		RequiredScopes: []string{"repo", "read:project", "read:org"},
	}
	got, err := v.Validate(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"read:project", "read:org"}
	if len(got.MissingScopes) != len(want) {
		t.Fatalf("MissingScopes = %v, want %v", got.MissingScopes, want)
	}
	for i := range want {
		if got.MissingScopes[i] != want[i] {
			t.Errorf("MissingScopes[%d] = %q, want %q", i, got.MissingScopes[i], want[i])
		}
	}
	if !got.QuotaSufficient {
		t.Errorf("QuotaSufficient false but rate has 5000/5000/30 against zero quota")
	}
}

func TestValidator_QuotaInsufficient_Skipped(t *testing.T) {
	t.Parallel()
	mock := &restMock{
		scopes:   "repo",
		rateBody: `{"resources":{"core":{"remaining":10,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`,
	}
	v := &TokenValidator{
		Token: config.NewToken("ghp_valid"),
		REST:  newREST(t, mock),
		Quota: Quota{REST: 100, GraphQL: 50, Search: 0},
	}
	got, err := v.Validate(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.QuotaSufficient {
		t.Errorf("QuotaSufficient = true, want false (REST remaining=10 < required=100)")
	}
}

func TestValidator_RateLimit5xx_Retryable(t *testing.T) {
	t.Parallel()
	mock := &restMock{
		scopes:   "repo",
		rateCode: http.StatusInternalServerError,
	}
	v := &TokenValidator{
		Token: config.NewToken("ghp_valid"),
		REST:  newREST(t, mock),
	}
	_, err := v.Validate(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("err type = %T, want *RetryableError; err=%v", err, err)
	}
}

func TestDiffScopes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		required []string
		have     []string
		want     []string
	}{
		{"all_present", []string{"repo"}, []string{"repo", "read:user"}, nil},
		{"one_missing", []string{"repo", "read:project"}, []string{"repo"}, []string{"read:project"}},
		{"empty_required", nil, []string{"repo"}, nil},
		{"with_whitespace", []string{"repo"}, []string{" repo ", "read:user"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diffScopes(tc.required, tc.have)
			if !equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
