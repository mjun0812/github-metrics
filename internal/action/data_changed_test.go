package action

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

// contentsMock is a small RoundTripper that serves GET /contents/...
// with configurable status + body. Used by data_changed_test.
type contentsMock struct {
	mu     sync.Mutex
	body   string
	status int
}

func (m *contentsMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}, nil
}

func newRESTComparator(t *testing.T, mock http.RoundTripper) *githubapi.REST {
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

// b64 returns the GitHub-style base64 (newline every 60 chars) of s.
func b64(s string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(s))
	// Wrap at 60 chars to mimic GitHub Contents API responses.
	var out []byte
	for len(enc) > 60 {
		out = append(out, []byte(enc[:60])...)
		out = append(out, '\n')
		enc = enc[60:]
	}
	out = append(out, []byte(enc)...)
	return string(out)
}

const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg"><text>hi</text></svg>`

func TestHashComparator_Equal_MatchingContent(t *testing.T) {
	t.Parallel()
	body := fmt.Sprintf(`{"content":%q,"encoding":"base64"}`, b64(sampleSVG))
	mock := &contentsMock{body: body}
	cmp := &HashComparator{
		REST:      newRESTComparator(t, mock),
		RepoOwner: "o", RepoName: "r", Branch: "main",
		Filename: "github-metrics.svg",
		NewBody:  []byte(sampleSVG),
	}
	eq, err := cmp.Equal(context.Background())
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if !eq {
		t.Errorf("expected hashes to match")
	}
}

func TestHashComparator_Equal_MismatchedContent(t *testing.T) {
	t.Parallel()
	body := fmt.Sprintf(`{"content":%q,"encoding":"base64"}`, b64(`<svg><text>old</text></svg>`))
	mock := &contentsMock{body: body}
	cmp := &HashComparator{
		REST:      newRESTComparator(t, mock),
		RepoOwner: "o", RepoName: "r",
		Filename: "x.svg",
		NewBody:  []byte(`<svg><text>new</text></svg>`),
	}
	eq, err := cmp.Equal(context.Background())
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if eq {
		t.Errorf("expected hashes to differ")
	}
}

func TestHashComparator_Equal_404IsNewFile(t *testing.T) {
	t.Parallel()
	mock := &contentsMock{status: http.StatusNotFound, body: `{"message":"Not Found"}`}
	cmp := &HashComparator{
		REST:      newRESTComparator(t, mock),
		RepoOwner: "o", RepoName: "r",
		Filename: "x.svg",
		NewBody:  []byte(sampleSVG),
	}
	eq, err := cmp.Equal(context.Background())
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if eq {
		t.Errorf("404 should be treated as 'changed'; got eq=true")
	}
}

func TestHashComparator_Equal_5xxRetryable(t *testing.T) {
	t.Parallel()
	mock := &contentsMock{status: http.StatusInternalServerError, body: `{}`}
	cmp := &HashComparator{
		REST:      newRESTComparator(t, mock),
		RepoOwner: "o", RepoName: "r",
		Filename: "x.svg",
		NewBody:  []byte(sampleSVG),
	}
	_, err := cmp.Equal(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("err type = %T, want *RetryableError; err=%v", err, err)
	}
}

func TestHashComparator_Equal_UnsupportedEncoding(t *testing.T) {
	t.Parallel()
	body := `{"content":"not-base64","encoding":"utf-8"}`
	mock := &contentsMock{body: body}
	cmp := &HashComparator{
		REST:      newRESTComparator(t, mock),
		RepoOwner: "o", RepoName: "r",
		Filename: "x.svg",
		NewBody:  []byte(sampleSVG),
	}
	if _, err := cmp.Equal(context.Background()); err == nil {
		t.Errorf("expected error for unsupported encoding")
	}
}
