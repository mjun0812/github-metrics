// Package mocks hosts the M9 shared mock RoundTrippers + PluginContext
// builder consumed by `*_test.go` files across the project.
//
// Contracts:
//   - specs/007-m9-test-infrastructure/contracts/rest-mock.md
//   - specs/007-m9-test-infrastructure/contracts/graphql-mock.md
package mocks

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// RESTMux is a per-path RoundTripper for REST tests. Construct with
// NewRESTMux(t); register handlers via OnFile / OnBody / OnHeader /
// OnFunc; consume the *http.Response via your code-under-test through
// the project's `internal/githubapi.NewREST` (pass it as
// `httpx.Options.Transport` together with `DisableRetries: true`).
//
// Goroutine-safe: dispatch (RoundTrip) takes an RLock; registration
// (On*) takes a write lock. Multiple `t.Run` + `t.Parallel` subtests
// sharing one mux do not race.
type RESTMux struct {
	t        *testing.T
	mu       sync.RWMutex
	handlers map[string]restHandler
	calls    map[string]int
}

// restHandler is the per-path dispatch closure. Either body (lazy
// fixture read) OR fn (dynamic) is set per entry.
type restHandler struct {
	status  int
	body    string
	header  http.Header
	fixture string // relative to tests/fixtures/; empty = inline body
	fn      func(req *http.Request) (status int, body string, header http.Header)
}

// NewRESTMux constructs a RESTMux ready to register handlers on.
// Registers a t.Cleanup that resets handler state at end of test.
func NewRESTMux(t *testing.T) *RESTMux {
	t.Helper()
	m := &RESTMux{
		t:        t,
		handlers: map[string]restHandler{},
		calls:    map[string]int{},
	}
	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.handlers = nil
		m.calls = nil
	})
	return m
}

// OnFile registers a 200 OK fixture-file-backed handler for the given
// request path. `fixturePath` is relative to `tests/fixtures/`.
// The file MUST exist at first matching dispatch; missing files
// trigger t.Fatalf.
func (m *RESTMux) OnFile(path, fixturePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[path] = restHandler{status: http.StatusOK, fixture: fixturePath}
}

// OnBody registers an inline-body handler.
func (m *RESTMux) OnBody(path string, status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[path] = restHandler{status: status, body: body}
}

// OnHeader registers an inline-body handler with a custom response
// header (e.g., Link header for paginated endpoints).
func (m *RESTMux) OnHeader(path string, status int, body string, header http.Header) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[path] = restHandler{status: status, body: body, header: header}
}

// OnFunc registers a per-call dynamic handler that sees the full
// *http.Request (use req.URL.RawQuery to inspect query strings).
func (m *RESTMux) OnFunc(path string, fn func(req *http.Request) (status int, body string, header http.Header)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[path] = restHandler{fn: fn}
}

// Calls returns the number of times RoundTrip dispatched to `path`.
func (m *RESTMux) Calls(path string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[path]
}

// RoundTrip satisfies http.RoundTripper.
func (m *RESTMux) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	m.mu.Lock()
	h, ok := m.handlers[path]
	m.calls[path]++
	m.mu.Unlock()

	if !ok {
		return mkResp(req, http.StatusNotFound, nil, `{"message":"Not Found"}`), nil
	}
	if h.fn != nil {
		status, body, header := h.fn(req)
		return mkResp(req, status, header, body), nil
	}
	body := h.body
	if h.fixture != "" {
		loaded, err := loadFixture(h.fixture)
		if err != nil {
			m.t.Fatalf("mocks.RESTMux: fixture not found for path %q: %v", path, err)
		}
		body = string(loaded)
	}
	return mkResp(req, h.status, h.header, body), nil
}

// mkResp builds an *http.Response with the canonical
// Content-Type: application/json header set (callers can override
// via the header argument).
func mkResp(req *http.Request, status int, h http.Header, body string) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// loadFixture reads a file under <repo-root>/tests/fixtures/<path>.
// Used by both RESTMux and GraphQLMux.
func loadFixture(relativePath string) ([]byte, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	abs := filepath.Join(root, "tests", "fixtures", relativePath)
	return os.ReadFile(abs) //nolint:gosec // path is operator-controlled (test fixture)
}

// repoRoot walks up from the current working directory until it
// finds the go.mod marker. Cached after first lookup.
var (
	repoRootCache string
	repoRootOnce  sync.Once
)

func repoRoot() (string, error) {
	var initErr error
	repoRootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			initErr = err
			return
		}
		for {
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				repoRootCache = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				initErr = os.ErrNotExist
				return
			}
			dir = parent
		}
	})
	return repoRootCache, initErr
}
