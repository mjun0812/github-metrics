// Package main implements normalize-svg-stream: a thin stdin→stdout
// masker that replaces the two dynamic footer fragments in an SVG so
// committed samples under docs/examples/ stay byte-stable across
// re-runs (FR-008 idempotency).
//
// This tool intentionally does NOT round-trip the SVG through
// encoding/xml: Go's encoder duplicates xmlns declarations on every
// pass, breaking idempotency. The two regex masks below are
// sufficient — the rendering pipeline (templates + chromedp resize)
// is otherwise deterministic for a fixed input.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
)

var (
	lastUpdatedRE = regexp.MustCompile(`Last updated [^<]+`)
	versionRE     = regexp.MustCompile(`github-metrics@[^<\s]+`)
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "normalize-svg-stream: read stdin: %v\n", err)
		os.Exit(1)
	}
	out := lastUpdatedRE.ReplaceAll(raw, []byte("Last updated __MASKED__"))
	out = versionRE.ReplaceAll(out, []byte("github-metrics@__MASKED__"))
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "normalize-svg-stream: write stdout: %v\n", err)
		os.Exit(1)
	}
}
