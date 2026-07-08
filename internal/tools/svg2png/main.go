// Command svg2png rasterizes one or more standalone metrics SVG files
// to PNG using the same resvg-based renderer the production pipeline
// uses (internal/render.Resvg + Resize). Two uses:
//
//   - Developer aid for visual layout comparison between the Go output
//     (docs/examples/) and the upstream reference output
//     (docs/org_examples/) — both rendered through the identical resvg
//     path so any difference is a real layout difference, not a renderer
//     artifact.
//   - The doc-sample regen pipeline (regen-doc-samples.yml /
//     scripts/gen-doc-samples.sh) derives each sample's PNG from its
//     already-rendered SVG with zero API calls, instead of running a
//     second full API fetch (#527). For that reason the production image
//     ships this binary next to metrics-cli (see Dockerfile).
//
// Usage:
//
//	METRICS_RESVG_PATH=/usr/local/bin/resvg \
//	  go run ./internal/tools/svg2png --out /tmp/png a.svg b.svg ...
//
// It has no bearing on the action / CLI binaries' contract.
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

	renderer, err := render.NewResvg(render.ResvgOpts{})
	if err != nil {
		return fmt.Errorf("resvg: %w", err)
	}

	var failures int
	for _, f := range files {
		if err := rasterize(renderer, f, *out); err != nil {
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
func rasterize(renderer *render.Resvg, file, out string) error {
	svg, err := os.ReadFile(file) //nolint:gosec // dev tool: caller-supplied path is intentional
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := renderer.Resize(ctx, string(svg), render.ResizeOpts{Convert: "png"})
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
