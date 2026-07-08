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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// StackedSection is one vertically-stacked template section: its
// native-SVG markup and the pixel height it consumes, plus optional
// extra attributes for the wrapping translate `<g>` (e.g. the plugin
// dispatcher's `class="plugin-<slug>" data-plugin="<slug>"` hooks).
type StackedSection struct {
	Markup string
	Height int
	Attrs  string
}

// StackSections lays the sections out top-to-bottom, wrapping each in a
// `<g transform="translate(0,y)">`, and returns the combined body markup
// plus the total stacked height. Used by both templates now that #409
// Phase C dropped the outer foreignObject and each partial self-reports
// its height.
//
// A section with empty markup is skipped. A section that emitted markup
// but did NOT self-report a positive height (Height <= 0) cannot be
// placed deterministically without the old HTML flow, so it is skipped
// with a warning rather than overlapping the next section or injecting
// non-SVG markup. Every in-tree partial self-reports its height; this
// fallback only guards a future external-registry partial that forgets
// to (or a legacy HTML partial such as the repository `introduction`,
// which stays gated off by default).
func StackSections(sections []StackedSection, logger *slog.Logger) (string, int) {
	var body strings.Builder
	y := 0
	for _, s := range sections {
		if s.Markup == "" {
			continue
		}
		if s.Height <= 0 {
			if logger != nil {
				logger.Warn("template: skipping partial that reported no height",
					"attrs", strings.TrimSpace(s.Attrs))
			}
			continue
		}
		fmt.Fprintf(&body, `<g transform="translate(0,%d)"%s>%s</g>`, y, s.Attrs, s.Markup)
		y += s.Height
	}
	return body.String(), y
}

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
// no longer accepted (v3.0, #649 / #652); they are silently ignored
// here.
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

// Native-SVG metadata footer geometry. Mirrors the deleted `footer` CSS
// (`font-size:10px; font-style:italic; color:#666666; text-align:right;
// margin-top:8px; width:440px; margin-left:auto; padding-right:6px`), so
// the footer renders as small right-aligned grey italic text (#409 Phase
// C: the outer foreignObject and its HTML `<footer>` are gone).
const (
	footerFont    = 10.0
	footerFill    = "#666666"
	footerTop     = 8.0           // footer margin-top
	footerLinePad = 3.0           // extra leading per line
	footerRightX  = CardWidth - 6 // right anchor (padding-right:6px)
	footerLineH   = footerFont + footerLinePad
)

// MetadataFooter renders the metadata footer as native SVG when the
// `metadata` section is in the resolved set (i.e. `chrome_metadata=yes`),
// or when the legacy expanded `base.metadata` input is truthy. The block
// is wrapped by WrapSection in `<g data-section="metadata">` so DOM
// diffing can locate it AND so render.Hash can drop the whole (timestamp-
// bearing) section before hashing, keeping data-changed detection stable
// across renders. Returns the markup and the pixel height it consumes
// ("" / 0 when the footer is disabled).
func MetadataFooter(pc *templates.PartialContext, sections map[string]struct{}, opts FooterOpts) (string, int) {
	_, enabledByBase := sections["metadata"]
	if !enabledByBase && (pc == nil || pc.Inputs == nil || !TruthyInput(pc.Inputs, "base.metadata")) {
		return "", 0
	}
	tz := ""
	if pc != nil && pc.Data != nil {
		tz = pc.Data.Config.Timezone.Name
	}

	var rows []string
	if opts.IncludePrivateNotice && pc != nil && pc.Data != nil && pc.Data.Account == plugins.AccountUser {
		rows = append(rows, "These metrics include private contributions")
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if tz != "" && tz != "UTC" {
		rows = append(rows, fmt.Sprintf("Last updated %s (timezone %s) with mjun0812/github-metrics@%s",
			stamp, tz, engine.Version()))
	} else {
		rows = append(rows, fmt.Sprintf("Last updated %s with mjun0812/github-metrics@%s",
			stamp, engine.Version()))
	}

	var b strings.Builder
	for i, ln := range rows {
		baseline := footerTop + float64(i)*footerLineH + footerFont
		b.WriteString(SVGText(footerRightX, baseline, ln, SVGTextOpts{
			Size:   footerFont,
			Fill:   footerFill,
			Italic: true,
			Anchor: "end",
		}))
	}
	height := int(footerTop + float64(len(rows))*footerLineH)
	return WrapSection("metadata", height, b.String()), height
}

// Contribution-calendar geometry shared between classic and
// repository BaseHeader: a 14-day row of 11×11 day cells on a 15px
// pitch (4px gap), with a GitHub-canonical empty-cell color.
const (
	calendarCellSize  = 11
	calendarCellPitch = 15
	emptyCellColor    = "#ebedf0"
)

// calendarLevelColors is the GitHub contribution-graph L0..L4 ramp
// (`--color-calendar-graph-day-{bg,L1..L4}-bg` in style.css). resvg does
// not resolve those CSS variables (#409 decision log #4), so the chart
// partials emit the literal hex via CalendarLevelColor.
var calendarLevelColors = [5]string{emptyCellColor, "#9be9a8", "#40c463", "#30a14e", "#216e39"}

// CalendarLevelColor returns the literal fill for contribution-graph
// level `level` (0 = empty, 1..4 = increasing intensity). Out-of-range
// levels fall back to the empty-cell color.
func CalendarLevelColor(level int) string {
	if level < 0 || level >= len(calendarLevelColors) {
		return emptyCellColor
	}
	return calendarLevelColors[level]
}

// escapeXML is a tiny local copy of classic/partials.EscapeXML so this
// package can stay above classic/partials in the import graph (and
// classic/partials can in turn depend on chrome for its primitives).
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
