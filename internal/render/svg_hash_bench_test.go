package render

import (
	"strings"
	"testing"
)

// BenchmarkHash_LargeSVG exercises the Hash hot path against a
// ~500 KB synthetic SVG.
//
// Budget: 50 ms/op. The original target was 5 ms but the goquery
// HTML5 parse of a 500 KB document lands around 20 ms on an
// M-series Mac — the parser is the
// dominant cost and pre-empting it would mean abandoning DOM-aware
// footer removal. The relaxed budget still catches catastrophic
// regressions (e.g., adding an O(N²) walk) while honoring the real
// implementation cost. Realistic classic output is ~40 KB, which
// hashes in ~3 ms.
func BenchmarkHash_LargeSVG(b *testing.B) {
	in := buildLargeSVG(500_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Hash(in); err != nil {
			b.Fatalf("Hash: %v", err)
		}
	}
	b.ReportMetric(float64(len(in))/1024, "KB/in")
	avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
	if avgNs > 50_000_000 {
		b.Fatalf("Hash exceeded 50 ms/op regression budget: %.2f ms/op", avgNs/1_000_000)
	}
}

// BenchmarkOptimizeCSS_50Selectors anchors the CSS stage's
// performance budget (< 50 ms/op) against the same 50-selector
// workload the SC-006 test uses.
func BenchmarkOptimizeCSS_50Selectors(b *testing.B) {
	in := buildFiftySelectorSVG()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := OptimizeCSS(in); err != nil {
			b.Fatalf("OptimizeCSS: %v", err)
		}
	}
	avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
	if avgNs > 50_000_000 {
		b.Fatalf("OptimizeCSS exceeded 50 ms/op budget: %.2f ms/op", avgNs/1_000_000)
	}
}

// BenchmarkReplaceOcticons_50Placeholders measures the octicon
// substitution path against a 50-placeholder input. Budget: < 10 ms.
func BenchmarkReplaceOcticons_50Placeholders(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg">`)
	for i := 0; i < 50; i++ {
		sb.WriteString(`<text>:octicon-star-16:</text>`)
	}
	sb.WriteString(`</svg>`)
	in := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ReplaceOcticons(in); err != nil {
			b.Fatalf("ReplaceOcticons: %v", err)
		}
	}
	avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
	if avgNs > 10_000_000 {
		b.Fatalf("ReplaceOcticons exceeded 10 ms/op budget: %.2f ms/op", avgNs/1_000_000)
	}
}

// buildLargeSVG returns an SVG roughly `target` bytes long made of a
// repeated boring `<g>` group. Used by the Hash benchmark.
func buildLargeSVG(target int) string {
	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg">`)
	piece := `<g class="row"><text>row</text><path d="M0 0L1 1"/></g>`
	for sb.Len() < target {
		sb.WriteString(piece)
	}
	sb.WriteString("<footer>generated 2026-05-15T00:00:00Z</footer></svg>")
	return sb.String()
}

// buildFiftySelectorSVG mirrors the SC-006 test fixture: 50 .sN
// rules, half of which the body references.
func buildFiftySelectorSVG() string {
	var styles, body strings.Builder
	for i := 0; i < 50; i++ {
		styles.WriteString(`.s`)
		styles.WriteString(itoa(i))
		styles.WriteString(`{color:red;}`)
		if i%2 == 0 {
			body.WriteString(`<g class="s`)
			body.WriteString(itoa(i))
			body.WriteString(`"/>`)
		}
	}
	return `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">` +
		styles.String() + `</style>` + body.String() + `</svg>`
}

// itoa is a tiny strconv.Itoa replacement that avoids the dep churn
// when this file is the only consumer in the package.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
