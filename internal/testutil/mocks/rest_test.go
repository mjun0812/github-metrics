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

func TestRESTMux_OnFile_HappyPath(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	mux.OnFile("/contributors", "github/rest/contributors_hello_world.json")
	resp, err := mux.RoundTrip(newGET("/contributors"))
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "octocat") {
		t.Errorf("body should mention octocat; got %q", body)
	}
}

func TestRESTMux_OnBody_StatusAndBody(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	mux.OnBody("/x", http.StatusTeapot, `{"hi":"there"}`)
	resp, err := mux.RoundTrip(newGET("/x"))
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("status = %d, want 418", resp.StatusCode)
	}
	if body := readBody(t, resp); body != `{"hi":"there"}` {
		t.Errorf("body = %q", body)
	}
}

func TestRESTMux_OnHeader_LinkHeader(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	hdr := http.Header{
		"Link": []string{`<https://x?page=42>; rel="last"`},
	}
	mux.OnHeader("/paged", http.StatusOK, `[]`, hdr)
	resp, _ := mux.RoundTrip(newGET("/paged"))
	if got := resp.Header.Get("Link"); !strings.Contains(got, `page=42`) {
		t.Errorf("Link header = %q", got)
	}
}

func TestRESTMux_OnFunc_PerCallDynamic(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	calls := 0
	mux.OnFunc("/dyn", func(req *http.Request) (int, string, http.Header) {
		calls++
		return http.StatusOK, `{"call":` + itoa(calls) + `}`, nil
	})
	for i := 1; i <= 3; i++ {
		resp, _ := mux.RoundTrip(newGET("/dyn"))
		body := readBody(t, resp)
		want := `{"call":` + itoa(i) + `}`
		if body != want {
			t.Errorf("call #%d body = %q, want %q", i, body, want)
		}
	}
}

func TestRESTMux_Calls_CountsDispatchesPerPath(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	mux.OnBody("/a", 200, "{}")
	mux.OnBody("/b", 200, "{}")
	for i := 0; i < 5; i++ {
		_, _ = mux.RoundTrip(newGET("/a"))
	}
	for i := 0; i < 2; i++ {
		_, _ = mux.RoundTrip(newGET("/b"))
	}
	if got := mux.Calls("/a"); got != 5 {
		t.Errorf("Calls(a) = %d, want 5", got)
	}
	if got := mux.Calls("/b"); got != 2 {
		t.Errorf("Calls(b) = %d, want 2", got)
	}
	// Unknown path 404 still counts (a programming-error test would
	// notice "no handler hit, yet Calls=0").
	_, _ = mux.RoundTrip(newGET("/unknown"))
	if got := mux.Calls("/unknown"); got != 1 {
		t.Errorf("Calls(unknown) = %d, want 1", got)
	}
}

func TestRESTMux_Concurrent_NoRace(t *testing.T) {
	t.Parallel()
	mux := mocks.NewRESTMux(t)
	mux.OnBody("/x", 200, "{}")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = mux.RoundTrip(newGET("/x"))
				_ = mux.Calls("/x")
			}
		}()
	}
	wg.Wait()
	if got := mux.Calls("/x"); got != 20*50 {
		t.Errorf("Calls(x) = %d, want %d", got, 20*50)
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
