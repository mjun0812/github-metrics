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

//go:embed all:plugins all:templates octicons/data.json version.txt
var fsys embed.FS

// FS returns the embedded filesystem rooted at the assets/ directory.
// The template loaders (classic / repository) read templates/<name>
// from it via fs.Sub; octicon data is exposed separately through
// OcticonData.
func FS() embed.FS { return fsys }

// OcticonData returns the embedded primer/octicons data.json that
// gen-octicons produces. internal/render.ReplaceOcticons reads it
// once at first use.
func OcticonData() ([]byte, error) {
	return fsys.ReadFile("octicons/data.json")
}
