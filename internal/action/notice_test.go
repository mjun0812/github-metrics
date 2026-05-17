package action

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

type releaseMock struct {
	mu     sync.Mutex
	body   string
	status int
}

func (m *releaseMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.status
	if status == 0 {
		status = http.StatusOK
	}
	h := http.Header{"Content-Type": []string{"application/json"}}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}, nil
}

func newRESTNotice(t *testing.T, mock http.RoundTripper) *githubapi.REST {
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

func TestCheckLatestRelease_Newer(t *testing.T) {
	t.Parallel()
	mock := &releaseMock{body: `{"tag_name":"v2.0.0","html_url":"https://github.com/mjun0812/github-metrics/releases/tag/v2.0.0"}`}
	rest := newRESTNotice(t, mock)
	msg := CheckLatestRelease(context.Background(), rest, "mjun0812/github-metrics", "v1.0.0")
	if !strings.Contains(msg, "v2.0.0") {
		t.Errorf("expected newer-version notice; got %q", msg)
	}
	if !strings.Contains(msg, "v1.0.0") {
		t.Errorf("notice should mention current version; got %q", msg)
	}
}

func TestCheckLatestRelease_SameVersion_NoNotice(t *testing.T) {
	t.Parallel()
	mock := &releaseMock{body: `{"tag_name":"v1.0.0"}`}
	rest := newRESTNotice(t, mock)
	msg := CheckLatestRelease(context.Background(), rest, "mjun0812/github-metrics", "v1.0.0")
	if msg != "" {
		t.Errorf("expected no notice for same version; got %q", msg)
	}
}

func TestCheckLatestRelease_API5xx_NoNoticeNoError(t *testing.T) {
	t.Parallel()
	mock := &releaseMock{body: ``, status: http.StatusInternalServerError}
	rest := newRESTNotice(t, mock)
	msg := CheckLatestRelease(context.Background(), rest, "mjun0812/github-metrics", "v1.0.0")
	if msg != "" {
		t.Errorf("expected empty notice on 5xx (best-effort); got %q", msg)
	}
}

func TestCheckLatestRelease_NilOrEmpty(t *testing.T) {
	t.Parallel()
	// nil rest
	if CheckLatestRelease(context.Background(), nil, "x/y", "v1.0.0") != "" {
		t.Errorf("nil REST should return empty")
	}
	// empty repo
	mock := &releaseMock{body: `{"tag_name":"v2.0.0"}`}
	rest := newRESTNotice(t, mock)
	if CheckLatestRelease(context.Background(), rest, "", "v1.0.0") != "" {
		t.Errorf("empty repo should return empty")
	}
}
