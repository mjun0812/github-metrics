package mocks_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestRESTMux_UnknownPath_Returns404(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	req := newGET("/unregistered")
	resp, err := mux.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Not Found") {
		t.Errorf("body should mention 'Not Found'; got %q", body)
	}
}

func newGET(path string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://mock.localhost"+path, nil)
	return req
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	_ = resp.Body.Close()
	return string(b)
}
