package partials

import (
	"strings"

	"github.com/mjun0812/github-metrics/internal/format"
)

// EscapeXML escapes the five XML predefined entities so dynamic strings
// (login, display name, repository name, etc.) can be safely embedded
// in the SVG output. The function is intentionally re-entrant: an
// already-escaped fragment will be re-escaped, which keeps the rule
// simple — callers must never pre-escape.
func EscapeXML(s string) string {
	if s == "" {
		return ""
	}
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&#34;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

// FormatCount renders integer magnitudes with the k/m/b/t short form
// used by the upstream classic template. It is a thin wrapper around
// format.Format so callers in this package can stay terse.
func FormatCount(n int64) string {
	return format.Format(n, format.Options{})
}
