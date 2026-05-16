package githubapi_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/mjun0812/github-metrics/internal/githubapi"
)

func TestREST_Scopes_Authenticated(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	h := http.Header{}
	h.Set("X-OAuth-Scopes", "repo, read:user")
	mock.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})

	rest := newRESTWithMock(t, mock)
	got, err := rest.Scopes(context.Background())
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}
	want := []string{"repo", "read:user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scopes = %v, want %v", got, want)
	}
}

func TestREST_Scopes_UnauthenticatedReturnsEmpty(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	// no X-OAuth-Scopes header
	mock.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Body: []byte(`{}`)})

	rest := newRESTWithMock(t, mock)
	got, err := rest.Scopes(context.Background())
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Scopes = %v, want empty slice", got)
	}
}

func TestREST_Scopes_HTTPErrorReturnsError(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.Set("GET", "/", githubapi.MockResponse{Status: http.StatusInternalServerError, Body: []byte(`{"message":"boom"}`)})

	rest := newRESTWithMock(t, mock)
	_, err := rest.Scopes(context.Background())
	if err == nil {
		t.Fatalf("expected error on 500")
	}
}

func TestREST_Scopes_CachedAfterFirstCall(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	h := http.Header{}
	h.Set("X-OAuth-Scopes", "repo")
	mock.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})

	rest := newRESTWithMock(t, mock)

	got1, err := rest.Scopes(context.Background())
	if err != nil {
		t.Fatalf("Scopes #1: %v", err)
	}
	got2, err := rest.Scopes(context.Background())
	if err != nil {
		t.Fatalf("Scopes #2: %v", err)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("Scopes cache divergence: %v vs %v", got1, got2)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 HTTP call (cache hit on 2nd), got %d", len(calls))
	}
}
