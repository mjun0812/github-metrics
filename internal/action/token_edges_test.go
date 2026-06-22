package action

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TestTokenValidator_FetchHelpers_ErrorEdges covers helper-level failures
// that Validate normally wraps.
func TestTokenValidator_FetchHelpers_ErrorEdges(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{}
	if _, err := v.fetchScopes(context.Background()); err == nil {
		t.Fatalf("expected nil REST fetchScopes error")
	}
	if _, err := v.fetchRateLimit(context.Background()); err == nil {
		t.Fatalf("expected nil REST fetchRateLimit error")
	}

	mock := &restMock{rateCode: 204}
	v.REST = newREST(t, mock)
	if _, err := v.fetchRateLimit(context.Background()); err == nil {
		t.Fatalf("expected non-2xx rate limit error")
	}
}

// TestTokenValidator_FetchRateLimitTransportError covers REST transport
// failures before HTTP status handling.
func TestTokenValidator_FetchRateLimitTransportError(t *testing.T) {
	t.Parallel()
	v := &TokenValidator{
		REST: newREST(t, staticRoundTripper{
			status: http.StatusInternalServerError,
			err:    errors.New("transport"),
		}),
		Token: config.NewToken("ghp_mock_pat_valid"),
	}
	if _, err := v.fetchRateLimit(context.Background()); err == nil {
		t.Fatalf("expected transport error")
	}
	_, err := v.Validate(context.Background())
	var retryable *xerrors.RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("Validate err = %T, want RetryableError", err)
	}
}

// TestDecodeRateLimit_InvalidJSON covers malformed rate_limit payloads.
func TestDecodeRateLimit_InvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := decodeRateLimit([]byte(`{`)); err == nil {
		t.Fatalf("expected decode error")
	}
}
