//go:build chromedp

package render

import (
	"context"
	"testing"
)

// TestBrowser_RecycleEvery exercises the recycle-after-N invariant:
// with RecycleEvery=3, the 4th NewTab call should trigger one
// allocator re-init (RecycledCount transitions from 0 -> 1).
func TestBrowser_RecycleEvery(t *testing.T) {
	// Not parallel — chromium child processes.
	b, err := New(BrowserOpts{RecycleEvery: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	if got := b.RecycledCount(); got != 0 {
		t.Fatalf("initial RecycledCount = %d, want 0", got)
	}

	for i := 0; i < 3; i++ {
		_, cancel, err := b.NewTab(context.Background())
		if err != nil {
			t.Fatalf("NewTab #%d: %v", i, err)
		}
		cancel()
	}
	// After 3 successful tabs the counter is at the threshold but
	// recycle has not run yet; the next NewTab should trigger it.
	if got := b.RecycledCount(); got != 0 {
		t.Errorf("RecycledCount before trigger = %d, want 0", got)
	}

	_, cancel, err := b.NewTab(context.Background())
	if err != nil {
		t.Fatalf("NewTab #4 (recycle trigger): %v", err)
	}
	cancel()

	if got := b.RecycledCount(); got != 1 {
		t.Errorf("RecycledCount after trigger = %d, want 1", got)
	}
}
