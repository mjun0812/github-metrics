// Package render implements the chromedp-backed SVG resize, the
// pure-Go svg.Hash, the SVG decoration pipeline (octicon / CSS purge /
// XML format), and the small Renderer interface that engine.Compute
// consumes. Chromedp imports live behind a build tag so the default
// `go test ./...` run stays usable without a chromium binary.
package render
