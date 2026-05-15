//go:build chromedp

package render

import (
	"context"
	"errors"
	"os"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TestBrowser_New_DefaultPath starts a Browser with no explicit
// ExecPath, relying on chromedp / METRICS_CHROME_PATH auto-detection.
// The test is build-tag-gated so contributors without chromium do not
// hit it.
func TestBrowser_New_DefaultPath(t *testing.T) {
	// We do not parallelize because chromedp instances each spawn a
	// chromium process and parallel runs amplify CI flake.
	b, err := New(BrowserOpts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
}

// TestBrowser_New_FailsOnMissingBinary verifies the eager
// stat-check path: a non-existent ExecPath returns *InputError so
// callers can short-circuit instead of waiting on a chromedp launch
// timeout.
func TestBrowser_New_FailsOnMissingBinary(t *testing.T) {
	t.Parallel()
	_, err := New(BrowserOpts{ExecPath: "/nonexistent/chromium"})
	if err == nil {
		t.Fatal("expected error for missing chromium binary, got nil")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *InputError", err)
	}
}

// TestBrowser_NewTab_AfterClose ensures the post-Close guard returns
// an *InputError instead of crashing on the cancelled allocator.
func TestBrowser_NewTab_AfterClose(t *testing.T) {
	b, err := New(BrowserOpts{})
	if err != nil {
		t.Skipf("chromium not available: %v", err)
	}
	if cerr := b.Close(); cerr != nil {
		t.Fatalf("first Close: %v", cerr)
	}
	// Second Close is idempotent.
	if cerr := b.Close(); cerr != nil {
		t.Fatalf("second Close: %v", cerr)
	}
	_, _, tabErr := b.NewTab(context.Background())
	if tabErr == nil {
		t.Fatal("expected error from NewTab on closed Browser")
	}
	var ie *xerrors.InputError
	if !errors.As(tabErr, &ie) {
		t.Errorf("err type = %T, want *InputError", tabErr)
	}
}

// TestBrowser_NewTab_ReturnsCancelFunc exercises the happy path: a
// returned cancel func should not panic when called and Close should
// still work afterwards.
func TestBrowser_NewTab_ReturnsCancelFunc(t *testing.T) {
	if os.Getenv("METRICS_CHROME_PATH") == "" {
		t.Skip("METRICS_CHROME_PATH not set; chromedp tab smoke test skipped")
	}
	b, err := New(BrowserOpts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	_, cancel, err := b.NewTab(context.Background())
	if err != nil {
		t.Fatalf("NewTab: %v", err)
	}
	cancel() // must not panic
}
