// Package render implements the resvg-backed SVG rasterization, the
// pure-Go svg.Hash, the SVG decoration pipeline (octicon / CSS purge /
// XML format), and the small Renderer interface that engine.Compute
// consumes. Rasterization shells out to the `resvg` binary (resolved
// via METRICS_RESVG_PATH), so the default `go test ./...` run stays
// usable without it — the resvg-dependent tests skip when the binary is
// absent.
package render
