package githubapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MockTransport is the M1 minimal http.RoundTripper used by REST tests
// (and US5 integration tests) to feed canned responses to the github
// API client without hitting the network. It is intentionally tiny;
// the full mock infrastructure that supports recorded fixtures lands
// with T-118 in M9.
//
// MockTransport routes by exact "<METHOD> <PATH>" key, falling back to
// "<METHOD> <PATH-prefix>*" wildcards. Unknown routes return HTTP 404
// with a diagnostic body so failing tests see what was missed.
type MockTransport struct {
	mu     sync.Mutex
	routes map[string]MockResponse
	calls  []MockCall
}

// MockResponse is the canned reply for a single route entry. Either
// Status or Body may be zero — defaults are 200 and `{}` respectively.
type MockResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// MockCall records a single observed request so tests can assert
// interaction history after the fact.
type MockCall struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// NewMockTransport returns an empty MockTransport.
func NewMockTransport() *MockTransport {
	return &MockTransport{routes: map[string]MockResponse{}}
}

// Set registers a response for the given method+path tuple. Path may
// end with "*" to match any suffix (greedy, longest-prefix wins).
func (m *MockTransport) Set(method, path string, resp MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[strings.ToUpper(method)+" "+path] = resp
}

// SetJSON is a convenience for the common "200 OK + JSON body" route.
func (m *MockTransport) SetJSON(method, path, body string) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	m.Set(method, path, MockResponse{Status: http.StatusOK, Header: h, Body: []byte(body)})
}

// Calls returns a snapshot of all observed requests.
func (m *MockTransport) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MockCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// RoundTrip implements http.RoundTripper.
func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := readBody(req.Body)
	call := MockCall{
		Method: req.Method,
		Path:   req.URL.Path,
		Header: req.Header.Clone(),
		Body:   body,
	}

	m.mu.Lock()
	m.calls = append(m.calls, call)
	resp, ok := m.match(req.Method, req.URL.Path)
	m.mu.Unlock()
	if !ok {
		return notFound(req, body), nil
	}
	return buildResponse(req, resp), nil
}

// match finds the response for method+path, preferring exact matches.
func (m *MockTransport) match(method, path string) (MockResponse, bool) {
	key := strings.ToUpper(method) + " " + path
	if r, ok := m.routes[key]; ok {
		return r, true
	}
	// Wildcard fallback: longest prefix wins.
	var (
		bestKey  string
		bestResp MockResponse
		found    bool
	)
	for k, r := range m.routes {
		if !strings.HasSuffix(k, "*") {
			continue
		}
		prefix := strings.TrimSuffix(k, "*")
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if !found || len(prefix) > len(bestKey) {
			bestKey, bestResp, found = prefix, r, true
		}
	}
	return bestResp, found
}

func buildResponse(req *http.Request, m MockResponse) *http.Response {
	status := m.Status
	if status == 0 {
		status = http.StatusOK
	}
	header := m.Header
	if header == nil {
		header = http.Header{}
	}
	body := m.Body
	if body == nil {
		body = []byte("{}")
	}
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
		Header:        header.Clone(),
	}
}

func notFound(req *http.Request, body []byte) *http.Response {
	msg := fmt.Sprintf(`{"error":"mock route missing","method":%q,"path":%q,"body":%q}`,
		req.Method, req.URL.Path, string(body))
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		Status:        http.StatusText(http.StatusNotFound),
		StatusCode:    http.StatusNotFound,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(strings.NewReader(msg)),
		ContentLength: int64(len(msg)),
		Request:       req,
		Header:        h,
	}
}

func readBody(r io.ReadCloser) []byte {
	if r == nil {
		return nil
	}
	defer r.Close()
	buf, _ := io.ReadAll(r)
	return buf
}
