// Package chrome owns the rendering bits both templates share — the
// metadata footer, the BaseHeader mini-contribution row, the `base`
// section resolver, and the lazy CSS asset loader. classic and
// repository each only customise what is genuinely different (header
// layout, plugin partial set), and lean on this package for the
// envelope.
package chrome

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// chromeSectionKeys is the canonical ordered set of `chrome_*` boolean
// inputs that drive the section gate (#640). The order matches both
// classic and repository partial dispatch.
var chromeSectionKeys = []string{
	"header",
	"activity",
	"community",
	"repositories",
	"metadata",
	"introduction",
}

// ChromeSectionInputKey returns the user-facing input name for a given
// section ("header" → "chrome_header"). Centralises the prefix so all
// readers share one spelling.
func ChromeSectionInputKey(section string) string { return "chrome_" + section }

// AnyChromeInputPresent reports whether the inputs map declares any
// `chrome_*` section key (regardless of truthiness). Used by the
// plugin auto-enable helpers to suppress the legacy `plugin_base`
// master-switch fallback when the caller has opted into the canonical
// per-section namespace.
func AnyChromeInputPresent(in map[string]any) bool {
	if in == nil {
		return false
	}
	for _, k := range chromeSectionKeys {
		if _, ok := in[ChromeSectionInputKey(k)]; ok {
			return true
		}
	}
	return false
}

// TruthyInput reports whether in[key] reads as a truthy toggle
// ("true" / "yes" / "1" / bool true). Mirrors upstream metadata.mjs's
// boolean coercion so the per-plugin `plugin_<slug>` gates behave the
// same in classic and repository templates.
func TruthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "yes" || x == "1"
	default:
		return false
	}
}

// ResolveBaseSections returns the set of enabled base section names
// derived from the user's inputs. Each `chrome_<section>` boolean
// (#640) that reads truthy enables its matching section; absent or
// falsy keys leave the section disabled. The function never falls
// back to a default-all set — opt-in is the only path in v3.
//
// The legacy v2 `base=<csv>` and `plugin_base_<section>` aliases are
// no longer accepted (v3.0, #649); they are silently ignored here.
// Action / CLI callers surface a diagnostic slog.Warn for those keys
// via internal/action.WarnLegacyChromeInputs before reaching this
// layer.
func ResolveBaseSections(in map[string]any) map[string]struct{} {
	out := map[string]struct{}{}
	for _, k := range chromeSectionKeys {
		if TruthyInput(in, ChromeSectionInputKey(k)) {
			out[k] = struct{}{}
		}
	}
	return out
}

// FooterOpts toggles per-template differences in MetadataFooter.
type FooterOpts struct {
	// IncludePrivateNotice prepends a "These metrics include private
	// contributions" span when pc.Data.Account == plugins.AccountUser.
	// classic enables this; repository does not (its account is repo).
	IncludePrivateNotice bool
}

// MetadataFooter renders the metadata block when the `metadata`
// section is in the resolved set (i.e. `chrome_metadata=yes`), or
// when the legacy expanded `base.metadata` input is truthy. The block is
// wrapped in `<section data-section="metadata">` so DOM diffing can
// locate it, and the inner `<footer>` is preserved so the M3
// render.Hash footer-stripping rule still drops the timestamp before
// hashing.
func MetadataFooter(pc *templates.PartialContext, sections map[string]struct{}, opts FooterOpts) string {
	_, enabledByBase := sections["metadata"]
	if !enabledByBase && (pc == nil || pc.Inputs == nil || !TruthyInput(pc.Inputs, "base.metadata")) {
		return ""
	}
	tz := ""
	if pc != nil && pc.Data != nil {
		tz = pc.Data.Config.Timezone.Name
	}
	var b strings.Builder
	b.WriteString(`<section data-section="metadata">`)
	b.WriteString(`<footer>`)
	if opts.IncludePrivateNotice && pc != nil && pc.Data != nil && pc.Data.Account == plugins.AccountUser {
		b.WriteString(`<span>These metrics include private contributions</span>`)
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if tz != "" && tz != "UTC" {
		fmt.Fprintf(&b, `<span>Last updated %s (timezone %s) with mjun0812/github-metrics@%s</span>`,
			stamp, escapeXML(tz), escapeXML(engine.Version()))
	} else {
		fmt.Fprintf(&b, `<span>Last updated %s with mjun0812/github-metrics@%s</span>`,
			stamp, escapeXML(engine.Version()))
	}
	b.WriteString(`</footer>`)
	b.WriteString(`</section>`)
	return b.String()
}

// Contribution-calendar geometry shared between classic and
// repository BaseHeader: a 14-day row of 11×11 day cells on a 15px
// pitch (4px gap), with a GitHub-canonical empty-cell color.
const (
	calendarCellSize  = 11
	calendarCellPitch = 15
	emptyCellColor    = "#ebedf0"
)

// ContributionRow renders the mini contribution calendar as a single
// horizontal row of day cells. Returns "" when no days are present so
// the partial can hide the block. Mirrors upstream `base.header.ejs`,
// which lays the most recent 14 days out left-to-right.
func ContributionRow(days []plugins.ContributionDay) string {
	if len(days) == 0 {
		return ""
	}
	width := len(days) * calendarCellPitch
	var b strings.Builder
	b.WriteString(`<div class="field calendar" data-block="calendar-grid">`)
	fmt.Fprintf(
		&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="16">`,
		width, calendarCellSize, width,
	)
	b.WriteString(`<g>`)
	for i, d := range days {
		color := emptyCellColor
		if d.Color != "" {
			color = d.Color
		}
		fmt.Fprintf(
			&b,
			`<rect class="day" fill=%q x="%d" y="0" width="%d" height="%d" rx="2" ry="2"/>`,
			color, i*calendarCellPitch, calendarCellSize, calendarCellSize,
		)
	}
	b.WriteString(`</g></svg></div>`)
	return b.String()
}

// escapeXML is a tiny local copy of classic/partials.EscapeXML so this
// package can stay above classic/partials in the import graph (and
// classic/partials can in turn depend on chrome for ContributionRow).
// The set of escaped characters matches what classic/partials emits.
func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// Styles caches the classic-template fonts.css + style.css contents.
// Both templates render with the same CSS, so each template embeds
// one of these and calls Load on first render.
type Styles struct {
	mu     sync.Mutex
	loaded bool
	Fonts  string
	Style  string
}

// Load reads fonts.css and style.css from the given filesystem
// (expected to be the classic template's embedded FS) the first time
// it is called. Subsequent calls are no-ops.
func (s *Styles) Load(classicFS fs.FS) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}
	fonts, err := fs.ReadFile(classicFS, "fonts.css")
	if err != nil {
		return fmt.Errorf("chrome: read fonts.css: %w", err)
	}
	style, err := fs.ReadFile(classicFS, "style.css")
	if err != nil {
		return fmt.Errorf("chrome: read style.css: %w", err)
	}
	s.Fonts = string(fonts)
	s.Style = string(style)
	s.loaded = true
	return nil
}
