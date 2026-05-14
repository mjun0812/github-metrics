// Package assets bundles plugin/template metadata, GraphQL queries,
// and other static files into the binary via go:embed. The companion
// scripts/sync-assets.sh script materializes the directory tree from
// the upstream lowlighter/metrics checkout; this file is the only
// runtime entry point.
//
// The "all:" prefix is required so directories whose names begin with
// "_" (rare upstream cases) are also included.
package assets

import "embed"

//go:embed all:plugins all:templates version.txt
var fsys embed.FS

// FS returns the embedded filesystem rooted at the assets/ directory.
// Callers (typically [config.LoadMetadata]) walk plugins/ and
// templates/ from this root.
func FS() embed.FS { return fsys }
