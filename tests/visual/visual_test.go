// Package visual hosts the visual regression tests for the adopted
// plugin partials.
//
// The original suite drove a headless browser to make DOM-level
// assertions (querySelectorAll / getBoundingClientRect) against each
// partial's rendered SVG — coverage that byte-equality goldens cannot
// give. #409 Phase D removed the browser renderer in favour of the
// resvg rasterizer, which produces a PNG rather than a queryable DOM,
// so those DOM assertions no longer have a backing engine.
//
// Rebuilding this suite on top of resvg (rasterize each partial, assert
// on decoded pixels / non-empty raster regions) is #409 Phase E. Until
// then this file keeps the package compiling and the `tests/visual`
// import path valid, with a single skipping placeholder so the intent
// is discoverable.
package visual

import "testing"

// TestVisual_Placeholder documents the Phase E deferral. It always
// skips; the real resvg-based visual regression lands with #409 Phase E.
func TestVisual_Placeholder(t *testing.T) {
	t.Skip("visual regression suite is rebuilt on resvg in #409 Phase E; " +
		"the browser-DOM implementation was removed in Phase D (#694)")
}
