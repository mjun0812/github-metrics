package mocks_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

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

func newGQLReq(body string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://mock.localhost/graphql", strings.NewReader(body))
	return req
}
