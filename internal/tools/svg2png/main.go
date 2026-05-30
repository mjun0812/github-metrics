// Command svg2png rasterizes one or more standalone metrics SVG files
// to PNG using the same chromedp-based renderer the production pipeline
// uses (internal/render.Browser + Resize). It exists purely as a
// developer aid for visual layout comparison between the Go output
// (docs/examples/) and the upstream reference output (docs/org_examples/)
// — both rendered through the identical Chrome path so any difference is
// a real layout difference, not a renderer artifact.
//
// Usage:
//
//	METRICS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
//	  go run ./internal/tools/svg2png --out /tmp/png a.svg b.svg ...
//
// This tool is not shipped in the Docker image and has no bearing on
// the action / CLI binaries.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "svg2png:", err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", ".", "output directory for PNG files")
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		return fmt.Errorf("at least one input SVG file is required")
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		return fmt.Errorf("mkdir out: %w", err)
	}

	browser, err := render.New(render.BrowserOpts{})
	if err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	defer func() { _ = browser.Close() }()

	var failures int
	for _, f := range files {
		if err := rasterize(browser, f, *out); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL %s: %v\n", filepath.Base(f), err)
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d file(s) failed", failures)
	}
	return nil
}

// rasterize renders a single SVG file to <out>/<base>.png.
func rasterize(browser *render.Browser, file, out string) error {
	svg, err := os.ReadFile(file) //nolint:gosec // dev tool: caller-supplied path is intentional
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := browser.Resize(ctx, string(svg), render.ResizeOpts{
		Convert: "png",
		Padding: []string{"0, 8 + 11%"},
	})
	if err != nil {
		return fmt.Errorf("resize: %w", err)
	}
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	dst := filepath.Join(out, base+".png")
	if err := os.WriteFile(dst, res.Body, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fmt.Printf("  OK %s -> %s (%dx%d, %d bytes)\n", filepath.Base(file), dst, res.Width, res.Height, len(res.Body))
	return nil
}
