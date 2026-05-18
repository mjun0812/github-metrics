package mocks_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
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

// TestGraphQLMux_OnFile_MissingFile_TFatalf verifies the lazy fixture
// load path: a missing file fails fast at first dispatch (not at
// registration) so the failure message points at the right opName.
func TestGraphQLMux_OnFile_MissingFile_TFatalf(t *testing.T) {
	t.Parallel()
	res := testing.RunTests(func(_, _ string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{
				Name: "Subtest_MissingFixtureFile",
				F: func(st *testing.T) {
					mux := mocks.NewGraphQLMux(st)
					mux.OnFile("DoesNotExist", "github/graphql/__missing__.json")
					_, _ = mux.RoundTrip(newGQLReq(`{"operationName":"DoesNotExist","variables":{}}`))
				},
			},
		})
	if res {
		t.Error("subtest should have failed (fixture file does not exist)")
	}
}

// TestGraphQLMux_Calls_CountsDispatchesPerOpName covers FR-004: the
// counter must increment independently per opName and survive across
// multiple dispatches.
func TestGraphQLMux_Calls_CountsDispatchesPerOpName(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("Alpha", 200, `{"data":{"alpha":1}}`)
	mux.OnBody("Beta", 200, `{"data":{"beta":1}}`)
	for i := 0; i < 4; i++ {
		_, _ = mux.RoundTrip(newGQLReq(`{"operationName":"Alpha","variables":{}}`))
	}
	for i := 0; i < 7; i++ {
		_, _ = mux.RoundTrip(newGQLReq(`{"operationName":"Beta","variables":{}}`))
	}
	if got := mux.Calls("Alpha"); got != 4 {
		t.Errorf("Calls(Alpha) = %d, want 4", got)
	}
	if got := mux.Calls("Beta"); got != 7 {
		t.Errorf("Calls(Beta) = %d, want 7", got)
	}
	if got := mux.Calls("Gamma"); got != 0 {
		t.Errorf("Calls(Gamma) = %d, want 0 (never dispatched)", got)
	}
}

// TestGraphQLMux_Concurrent_NoRace covers FR-003: dispatch and Calls
// must be goroutine-safe under -race. Drives multiple opNames across
// goroutines and verifies the final counter matches dispatches.
func TestGraphQLMux_Concurrent_NoRace(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("OpA", 200, `{"data":{"a":1}}`)
	mux.OnBody("OpB", 200, `{"data":{"b":1}}`)

	const goroutines = 20
	const perGoroutine = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		op := "OpA"
		if i%2 == 1 {
			op = "OpB"
		}
		body := `{"operationName":"` + op + `","variables":{}}`
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				_, _ = mux.RoundTrip(newGQLReq(body))
				_ = mux.Calls(op)
			}
		}()
	}
	wg.Wait()
	want := goroutines / 2 * perGoroutine
	if got := mux.Calls("OpA"); got != want {
		t.Errorf("Calls(OpA) = %d, want %d", got, want)
	}
	if got := mux.Calls("OpB"); got != want {
		t.Errorf("Calls(OpB) = %d, want %d", got, want)
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
