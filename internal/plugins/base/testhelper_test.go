package base_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// graphQLMux is the per-operation mock GraphQL transport the base
// plugin tests use. Each (operationName) maps to an ordered slice of
// responses; consecutive calls drain it left-to-right and the last
// entry sticks. handlers (rather than literal bodies) allow per-call
// branching on the request variables (e.g. cursor-aware paging mocks).
type graphQLMux struct {
	mu        sync.Mutex
	handlers  map[string][]gqlHandler
	calls     map[string]*atomic.Int32
	totalCall atomic.Int32
}

type gqlHandler func(vars map[string]any) gqlResp

type gqlResp struct {
	Status int
	Body   string
}

func newGraphQLMux() *graphQLMux {
	return &graphQLMux{
		handlers: map[string][]gqlHandler{},
		calls:    map[string]*atomic.Int32{},
	}
}

// OnSequence registers a fixed-order sequence of responses for the
// given operation. The first call to the op returns seq[0], the second
// seq[1], …; calls beyond len(seq) keep returning the last entry.
func (m *graphQLMux) OnSequence(op string, seq ...gqlResp) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hs := make([]gqlHandler, 0, len(seq))
	for _, r := range seq {
		r := r
		hs = append(hs, func(_ map[string]any) gqlResp { return r })
	}
	m.handlers[op] = hs
	m.calls[op] = &atomic.Int32{}
}

// OnFunc registers a per-call handler that may branch on variables.
// Useful for cursor-aware paging mocks.
func (m *graphQLMux) OnFunc(op string, fn gqlHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[op] = []gqlHandler{fn}
	if _, ok := m.calls[op]; !ok {
		m.calls[op] = &atomic.Int32{}
	}
}

// Calls returns how many times the given op was invoked.
func (m *graphQLMux) Calls(op string) int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.calls[op]; ok {
		return c.Load()
	}
	return 0
}

func (m *graphQLMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.totalCall.Add(1)
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()

	var payload struct {
		OpName    string         `json:"operationName"`
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	_ = json.Unmarshal(body, &payload)

	m.mu.Lock()
	c, ok := m.calls[payload.OpName]
	if !ok {
		c = &atomic.Int32{}
		m.calls[payload.OpName] = c
	}
	idx := c.Add(1) - 1
	handlers := m.handlers[payload.OpName]
	m.mu.Unlock()

	if len(handlers) == 0 {
		return jsonResponse(req, http.StatusBadRequest,
			`{"errors":[{"message":"no fixture for `+payload.OpName+`"}]}`), nil
	}
	h := handlers[len(handlers)-1]
	if int(idx) < len(handlers) {
		h = handlers[idx]
	}
	resp := h(payload.Variables)
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	return jsonResponse(req, resp.Status, resp.Body), nil
}

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	if body == "" {
		body = "{}"
	}
	h := http.Header{}
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

// newGraphQL builds a *githubapi.GraphQL backed by the given mux.
func newGraphQL(t *testing.T, mux *graphQLMux) *githubapi.GraphQL {
	t.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return gql
}

// newPCWithGraphQL constructs a minimal PluginContext wired to a mux-
// backed GraphQL client and a Settings.Repositories=100 default so the
// base plugin's paging loop starts at batch=100.
func newPCWithGraphQL(t *testing.T, mux *graphQLMux) *plugins.PluginContext {
	t.Helper()
	return &plugins.PluginContext{
		Settings: &config.Settings{Repositories: 100},
		Inputs:   map[string]any{},
		GraphQL:  newGraphQL(t, mux),
		Data:     plugins.NewData(),
	}
}
