package mocks

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
)

// GraphQLMux dispatches genqlient GraphQL requests by the JSON
// request body's `operationName`. Implements http.RoundTripper —
// callers wire it into `internal/githubapi.NewGraphQL` via
// `httpx.Options.Transport` + `DisableRetries: true`.
//
// Goroutine-safe: dispatch (RoundTrip) takes an RLock; registration
// (On*) takes a write lock.
//
// Unknown operationName triggers t.Fatalf so missing handlers
// surface immediately with an actionable list of registered ops.
type GraphQLMux struct {
	t        *testing.T
	mu       sync.RWMutex
	handlers map[string]gqlHandler
	calls    map[string]int
}

type gqlHandler struct {
	status  int
	body    string
	fixture string
	fn      func(vars map[string]any) (status int, body string)
}

// NewGraphQLMux constructs an empty mux. The returned value is
// goroutine-safe and registers a t.Cleanup that resets handler state
// at end of test.
func NewGraphQLMux(t *testing.T) *GraphQLMux {
	t.Helper()
	m := &GraphQLMux{
		t:        t,
		handlers: map[string]gqlHandler{},
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

// OnFile registers a 200 OK fixture-file-backed handler keyed by
// operationName. `fixturePath` is relative to `tests/fixtures/`.
func (m *GraphQLMux) OnFile(opName, fixturePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[opName] = gqlHandler{status: http.StatusOK, fixture: fixturePath}
}

// OnBody registers an inline-body handler.
func (m *GraphQLMux) OnBody(opName string, status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[opName] = gqlHandler{status: status, body: body}
}

// OnFunc registers a per-call dynamic handler. The decoded
// `variables` map is passed in so cursor-aware paging mocks can
// branch on it.
func (m *GraphQLMux) OnFunc(opName string, fn func(vars map[string]any) (status int, body string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[opName] = gqlHandler{fn: fn}
}

// Calls returns the number of times RoundTrip dispatched to the
// given operationName.
func (m *GraphQLMux) Calls(opName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[opName]
}

// RoundTrip satisfies http.RoundTripper. The genqlient client posts
// `{"operationName": "...", "query": "...", "variables": {...}}` —
// we decode it, look up the handler by operationName, and return
// the canned response.
func (m *GraphQLMux) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()

	var payload struct {
		OpName    string         `json:"operationName"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		m.t.Fatalf("graphql mock: decode request body: %v", err)
		return nil, err
	}
	if payload.OpName == "" {
		m.t.Fatalf("graphql mock: request missing operationName; body=%s", string(body))
		return nil, nil
	}

	m.mu.Lock()
	h, ok := m.handlers[payload.OpName]
	m.calls[payload.OpName]++
	m.mu.Unlock()

	if !ok {
		m.t.Fatalf("graphql mock: no handler for opName %q (registered: %v)",
			payload.OpName, m.registeredOps())
		return nil, nil
	}
	if h.fn != nil {
		status, b := h.fn(payload.Variables)
		return mkGQLResp(req, status, b), nil
	}
	respBody := h.body
	if h.fixture != "" {
		loaded, err := loadFixture(h.fixture)
		if err != nil {
			m.t.Fatalf("graphql mock: fixture not found for opName %q: %v", payload.OpName, err)
			return nil, nil
		}
		respBody = string(loaded)
	}
	return mkGQLResp(req, h.status, respBody), nil
}

func (m *GraphQLMux) registeredOps() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.handlers))
	for k := range m.handlers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mkGQLResp(req *http.Request, status int, body string) *http.Response {
	h := http.Header{"Content-Type": []string{"application/json"}}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
