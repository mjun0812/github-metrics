package action

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// BannerInfo carries the data the startup banner displays. The pre-v3.0
// "Mode" row (action vs cli) was removed when the dispatch was unified
// (#646) — there is now exactly one entry pipeline, so the row no longer
// conveyed useful information.
type BannerInfo struct {
	Version     string   // semver tag or "dev"
	Template    string   // e.g. "classic"
	Plugins     []string // enabled plugin slugs (sorted, deprecated entries suffixed " (deprecated)")
	TokenMasked string   // masked token string (config.Token.String() output)
	GoVersion   string   // runtime.Version()
	OSArch      string   // "darwin/arm64" etc.
}

// PrintBanner writes the English-fixed startup banner to w. Format
// follows docs/design/13-appendix.md §E semantics: a top-bordered
// table block with key/value columns, suitable for grep'ing in
// GitHub Actions run logs and pinning via snapshot tests (SC-003).
//
// The banner is intentionally written directly to w (bypassing slog)
// so handler choice (JSON / text) does not mangle the ASCII layout.
func PrintBanner(w io.Writer, info BannerInfo) {
	plugins := append([]string(nil), info.Plugins...)
	sort.Strings(plugins)

	pluginsCell := "(none)"
	if len(plugins) > 0 {
		pluginsCell = strings.Join(plugins, ", ")
	}

	template := info.Template
	if template == "" {
		template = "classic"
	}
	tokenCell := info.TokenMasked
	if tokenCell == "" {
		tokenCell = "(not provided)"
	}

	const ruler = "──────────────────────────────────────────────────────────────"

	_, _ = fmt.Fprintln(w, ruler)
	_, _ = fmt.Fprintln(w, "── metrics-cli — startup banner ──")
	_, _ = fmt.Fprintf(w, "Version            │ %s\n", info.Version)
	_, _ = fmt.Fprintf(w, "Template           │ %s\n", template)
	_, _ = fmt.Fprintf(w, "Plugins            │ %s\n", pluginsCell)
	_, _ = fmt.Fprintf(w, "Token              │ %s\n", tokenCell)
	if info.GoVersion != "" {
		_, _ = fmt.Fprintf(w, "Runtime            │ %s, %s\n", info.GoVersion, info.OSArch)
	}
	_, _ = fmt.Fprintln(w, ruler)
}
