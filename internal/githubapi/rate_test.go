package githubapi_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

func newRESTForRate(t *testing.T, body string) (*githubapi.REST, *githubapi.MockTransport) {
	t.Helper()
	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", body)
	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaa"), "", httpx.Options{
		Transport:  mock,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return rest, mock
}

func TestResources_RefreshPopulatesAllBuckets(t *testing.T) {
	t.Parallel()

	rest, _ := newRESTForRate(t, `{
		"resources": {
			"core":    {"limit":5000,"used":10,"remaining":4990,"reset":1700000000},
			"graphql": {"limit":5000,"used":3,"remaining":4997,"reset":1700000100},
			"search":  {"limit":30,"used":1,"remaining":29,"reset":1700000200}
		},
		"rate": {"limit":5000,"used":10,"remaining":4990,"reset":1700000000}
	}`)

	res := githubapi.NewResources()
	if err := res.Refresh(context.Background(), rest); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	snap := res.Snapshot()
	if snap.REST.Remaining != 4990 {
		t.Errorf("REST.Remaining = %d", snap.REST.Remaining)
	}
	if snap.GraphQL.Remaining != 4997 {
		t.Errorf("GraphQL.Remaining = %d", snap.GraphQL.Remaining)
	}
	if snap.Search.Remaining != 29 {
		t.Errorf("Search.Remaining = %d", snap.Search.Remaining)
	}
	if !snap.REST.Reset.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Errorf("REST.Reset = %v", snap.REST.Reset)
	}
}

func TestResources_RefreshFailureKeepsPreviousState(t *testing.T) {
	t.Parallel()

	rest, mock := newRESTForRate(t, `{
		"resources": {
			"core":    {"limit":5000,"used":10,"remaining":4990,"reset":1700000000},
			"graphql": {"limit":5000,"used":0,"remaining":5000,"reset":0},
			"search":  {"limit":30,"used":0,"remaining":30,"reset":0}
		},
		"rate": {"limit":5000,"used":10,"remaining":4990,"reset":1700000000}
	}`)
	res := githubapi.NewResources()
	if err := res.Refresh(context.Background(), rest); err != nil {
		t.Fatalf("first Refresh: %v", err)
	}
	want := res.Snapshot()

	// Swap the route to return an error.
	mock.Set("GET", "/rate_limit", githubapi.MockResponse{Status: 500, Body: []byte(`{}`)})

	if err := res.Refresh(context.Background(), rest); err == nil {
		t.Fatalf("expected error from 500 response")
	}
	got := res.Snapshot()
	if got.REST.Remaining != want.REST.Remaining {
		t.Errorf("previous state should survive a failed refresh; got %d, was %d",
			got.REST.Remaining, want.REST.Remaining)
	}
}

func TestResources_ConcurrentRefreshAndSnapshot(t *testing.T) {
	t.Parallel()

	rest, _ := newRESTForRate(t, `{
		"resources": {
			"core":    {"limit":5000,"used":10,"remaining":4990,"reset":1700000000},
			"graphql": {"limit":5000,"used":0,"remaining":5000,"reset":0},
			"search":  {"limit":30,"used":0,"remaining":30,"reset":0}
		},
		"rate": {"limit":5000,"used":10,"remaining":4990,"reset":1700000000}
	}`)

	res := githubapi.NewResources()

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = res.Refresh(context.Background(), rest)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = res.Snapshot()
			}
		}()
	}
	wg.Wait()
	// The race detector enabled via `go test -race` is the real
	// assertion; if we reach here without panic we are good.
}
