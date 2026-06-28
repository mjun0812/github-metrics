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
// `chrome_*` section key (regardless of truthiness). Presence — not
// truthiness — gates the legacy CSV fallback so a caller that pins
// every chrome_* to `no` gets an empty section set instead of the v2
// "default all" behaviour.
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

// LegacyDefaultAllSections reports whether the inputs map triggers the
// v2 "all sections by default" backwards-compat path. True iff neither
// any `chrome_*` key nor the legacy `base` CSV input is declared.
// Plugin auto-enable helpers in internal/plugins/{base,header} consult
// this so the v2 default behaviour is preserved end-to-end: not only
// does ResolveBaseSections return the full section set, the relevant
// plugin Run calls also fire, so the partials have data to render.
func LegacyDefaultAllSections(in map[string]any) bool {
	if AnyChromeInputPresent(in) {
		return false
	}
	if _, present := ReadBaseInput(in); present {
		return false
	}
	return true
}

// resolveBaseDefaultLogged is set when the v2 "absent → all sections"
// fallback emits its deprecation warning at most once per process so
// repeated render calls do not spam the log.
var resolveBaseDefaultLogged sync.Once

// resolveBaseLegacyLogged guards the `base=CSV` deprecation warning to
// at most one emission per process.
var resolveBaseLegacyLogged sync.Once

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

// AllBaseSections is the legacy `base` CSV default — the comma-
// separated string the v2 "absent → all" fallback expands to before
// translation into the section set. (`introduction` is upstream-
// defined but gated separately and is not part of the v2 default.)
const AllBaseSections = "header, activity, community, repositories, metadata"

// ResolveBaseSections returns the set of enabled base section names
// derived from the user's inputs. Priority (highest first):
//
//  1. If any `chrome_<section>` key is declared in inputs, build the
//     set from those whose value is truthy.
//  2. Else if the legacy `base` CSV input is set, translate it into
//     the equivalent section set (a single deprecation warning is
//     emitted per process, listing the translated keys).
//  3. Else default to the v2 "all sections" set with a deprecation
//     warning explaining the v3 default flip.
//
// Step 2 only translates the section set used by the static
// dispatcher (classic.go / repository.go partialEnabledByBase). The
// per-partial gates inside internal/plugins/{base,header}/render.go
// (basePartialEnabled, activityEnabled, repositoriesEnabled) DO NOT
// consult the legacy CSV — they read chrome_* booleans directly.
// Direct engine callers that pass `Inputs: {"base": "header"}` will
// therefore get the right static dispatch decision but the partial
// bodies will short-circuit to "". Production callers (action.Run /
// action.RunCLI) avoid this by pre-translating via
// internal/action.TranslateLegacyChromeInputs before calling the
// engine; programmatic embedders MUST do the same.
func ResolveBaseSections(in map[string]any) map[string]struct{} {
	// 1. New canonical path: chrome_* booleans win when any are set.
	if AnyChromeInputPresent(in) {
		out := map[string]struct{}{}
		for _, k := range chromeSectionKeys {
			if TruthyInput(in, ChromeSectionInputKey(k)) {
				out[k] = struct{}{}
			}
		}
		return out
	}

	// 2. Legacy CSV path: warn once, translate to section set.
	if raw, present := ReadBaseInput(in); present {
		sections := splitBaseCSV(raw)
		translated := make([]string, 0, len(sections))
		for _, s := range chromeSectionKeys {
			if _, ok := sections[s]; ok {
				translated = append(translated, ChromeSectionInputKey(s)+"=yes")
			}
		}
		resolveBaseLegacyLogged.Do(func() {
			slog.Warn(
				"`base` input is deprecated; use `chrome_<section>=yes` (removed in v3.0)",
				"base", raw,
				"translated", strings.Join(translated, ", "),
			)
		})
		return sections
	}

	// 3. v2 backwards-compat default: warn once, return the v2
	// "all sections" set. v3 will flip this to an empty set.
	resolveBaseDefaultLogged.Do(func() {
		slog.Warn(
			"no `chrome_*` or `base` input set; defaulting to all sections (v3 will default to none — set `chrome_header=yes` etc. explicitly)",
		)
	})
	return splitBaseCSV(AllBaseSections)
}

// splitBaseCSV converts a `base` CSV value to the section set, used by
// both the deprecation alias path in ResolveBaseSections and the v2
// default fallback.
func splitBaseCSV(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		s := strings.ToLower(strings.TrimSpace(part))
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}

// ReadBaseInput extracts the `base` input. Returns (value, true) when
// the key is present even if the value is "" — callers need to tell
// "user set base to empty" from "user did not set base".
func ReadBaseInput(in map[string]any) (string, bool) {
	if in == nil {
		return "", false
	}
	v, ok := in["base"]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case []string:
		return strings.Join(x, ","), true
	case []any:
		parts := make([]string, 0, len(x))
		for _, p := range x {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ","), true
	}
	return "", false
}

// FooterOpts toggles per-template differences in MetadataFooter.
type FooterOpts struct {
	// IncludePrivateNotice prepends a "These metrics include private
	// contributions" span when pc.Data.Account == plugins.AccountUser.
	// classic enables this; repository does not (its account is repo).
	IncludePrivateNotice bool
}

// MetadataFooter renders the metadata block when the `metadata`
// section is in the resolved set (i.e. `chrome_metadata=yes`, or one
// of the legacy aliases the action layer translated), or when the
// legacy expanded `base.metadata` input is truthy. The block is
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
