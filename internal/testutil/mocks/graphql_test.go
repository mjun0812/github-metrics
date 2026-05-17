package mocks_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestGraphQLMux_OnFile_HappyPath(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnFile("User", "github/graphql/user_octocat.json")

	resp, err := mux.RoundTrip(newGQLReq(`{"operationName":"User","variables":{}}`))
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	body := readGQLBody(t, resp)
	if !strings.Contains(body, `"login": "octocat"`) {
		t.Errorf("response should contain octocat login; got %q", body)
	}
	if mux.Calls("User") != 1 {
		t.Errorf("Calls(User) = %d, want 1", mux.Calls("User"))
	}
}

func TestGraphQLMux_OnBody_StatusAndBody(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("Foo", http.StatusTeapot, `{"data":{"foo":42}}`)
	resp, _ := mux.RoundTrip(newGQLReq(`{"operationName":"Foo","variables":{}}`))
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("status = %d, want 418", resp.StatusCode)
	}
	if body := readGQLBody(t, resp); body != `{"data":{"foo":42}}` {
		t.Errorf("body = %q", body)
	}
}

func TestGraphQLMux_OnFunc_DecodesVariables(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnFunc("Paged", func(vars map[string]any) (int, string) {
		cursor, _ := vars["after"].(string)
		if cursor == "" {
			return 200, `{"data":{"page":1}}`
		}
		return 200, `{"data":{"page":2}}`
	})

	resp1, _ := mux.RoundTrip(newGQLReq(`{"operationName":"Paged","variables":{"after":null}}`))
	if body := readGQLBody(t, resp1); !strings.Contains(body, `"page":1`) {
		t.Errorf("first page body = %q", body)
	}

	resp2, _ := mux.RoundTrip(newGQLReq(`{"operationName":"Paged","variables":{"after":"cursor1"}}`))
	if body := readGQLBody(t, resp2); !strings.Contains(body, `"page":2`) {
		t.Errorf("second page body = %q", body)
	}
}

// TestGraphQLMux_UnknownOpName_TFatalf verifies the unknown-opName
// path triggers t.Fatalf with the registered operation list. We run
// it as a subtest with a recorded t so the test framework's Fatalf
// short-circuit doesn't kill the parent.
func TestGraphQLMux_UnknownOpName_TFatalf(t *testing.T) {
	t.Parallel()
	// Run the t.Fatalf-triggering dispatch inside a subtest; the
	// subtest fails — the parent test ASSERTS that it failed.
	res := testing.RunTests(func(_, _ string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{
				Name: "Subtest_UnknownOpName",
				F: func(st *testing.T) {
					mux := mocks.NewGraphQLMux(st)
					mux.OnBody("Known", 200, "{}")
					_, _ = mux.RoundTrip(newGQLReq(`{"operationName":"Missing","variables":{}}`))
				},
			},
		})
	if res {
		t.Error("subtest should have failed (no handler for Missing) — t.Fatalf did not trigger")
	}
}

func TestGraphQLMux_MissingOperationName_TFatalf(t *testing.T) {
	t.Parallel()
	res := testing.RunTests(func(_, _ string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{
				Name: "Subtest_MissingOpName",
				F: func(st *testing.T) {
					mux := mocks.NewGraphQLMux(st)
					_, _ = mux.RoundTrip(newGQLReq(`{"operationName":"","variables":{}}`))
				},
			},
		})
	if res {
		t.Error("subtest should have failed on missing operationName")
	}
}

func newGQLReq(body string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://mock.localhost/graphql", strings.NewReader(body))
	return req
}

func readGQLBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	_ = resp.Body.Close()
	return string(b)
}
