//go:build chromedp

package render

import (
	"context"
	"testing"
	"time"
)

// BenchmarkResize_FixedSVG measures the chromedp-driven Resize
// against the same SC-003 budget the M3 spec calls out: ≤ 2.5 s/op.
// The benchmark is build-tag-gated to keep the default suite fast.
func BenchmarkResize_FixedSVG(b *testing.B) {
	br, err := New(BrowserOpts{})
	if err != nil {
		b.Skipf("chromium unavailable: %v", err)
	}
	b.Cleanup(func() { _ = br.Close() })

	const in = `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="auto" viewBox="0 0 480 640">` +
		`<text x="16" y="32">bench</text>` +
		`<g id="metrics-end" transform="translate(0,640)"/>` +
		`</svg>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := br.Resize(ctx, in, ResizeOpts{
			Convert:     "svg",
			SettleDelay: 100 * time.Millisecond,
		})
		cancel()
		if err != nil {
			b.Fatalf("Resize: %v", err)
		}
	}
	avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
	if avgNs > 2_500_000_000 {
		b.Fatalf("Resize exceeded 2.5 s/op SC-003 sub-budget: %.2f s/op", avgNs/1e9)
	}
}
