package action

import (
	"context"
	"errors"
	"net/http"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

type nilResponseRoundTripper struct{}

func (nilResponseRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

// TestHashComparator_Equal_ErrorEdges covers malformed and nil response
// cases from the Contents API.
func TestHashComparator_Equal_ErrorEdges(t *testing.T) {
	t.Parallel()
	if _, err := (&HashComparator{}).Equal(context.Background()); err == nil {
		t.Fatalf("expected nil REST error")
	}

	cmp := &HashComparator{REST: newRESTComparator(t, nilResponseRoundTripper{})}
	_, err := cmp.Equal(context.Background())
	var retryable *xerrors.RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("nil response err = %T, want RetryableError", err)
	}

	cmp.REST = newRESTComparator(t, &contentsMock{body: `{`})
	if _, err := cmp.Equal(context.Background()); err == nil {
		t.Fatalf("expected JSON decode error")
	}

	cmp.REST = newRESTComparator(t, &contentsMock{body: `{"content":"%%%","encoding":"base64"}`})
	if _, err := cmp.Equal(context.Background()); err == nil {
		t.Fatalf("expected base64 decode error")
	}
}
