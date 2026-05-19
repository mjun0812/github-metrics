// Package languages owns the M4 "languages" plugin in its standard
// (most-used) mode. It aggregates per-repository language byte
// breakdowns the base plugin paged into pc.Data.Computed.RepositoryList
// and surfaces them as Favorites + Other + Mostly, mirroring upstream's
// data.plugins.languages JSON shape.
//
// Recent-mode and indepth-mode (P3, build tag heavy) ship in separate
// files (recent.go / indepth.go). Standard mode never issues additional
// API calls — it relies entirely on the data base already collected.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p1-mvp.md §1
// Data model: specs/004-m4-github-plugins/data-model.md E-010
package languages

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "languages"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &languagesPlugin{}

func init() {
	plugins.Register(Plugin)
}

type languagesPlugin struct{}

func (p *languagesPlugin) Name() string                     { return Name }
func (p *languagesPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload the plugin publishes under
// data.Plugins["languages"]. Field set mirrors upstream
// data.plugins.languages (constitution 原則 II).
type Result struct {
	Skipped       bool                   `json:"skipped,omitempty"`
	SkippedReason string                 `json:"-"`
	Mode          string                 `json:"mode,omitempty"`
	Favorites     []plugins.LanguageStat `json:"favorites"`
	Other         plugins.LanguageStat   `json:"other"`
	Sections      []string               `json:"sections"`
	Mostly        plugins.LanguageStat   `json:"mostly"`
	Colors        map[string]string      `json:"colors"`
	// Details mirrors upstream `plugins.languages.details` — a list of
	// per-language detail columns to render. Subset of
	// {"lines", "bytes-size", "percentage"}. mjun0812 uses all three.
	// (011 v2 additive extension per Principle II.)
	Details []string `json:"details,omitempty"`
	// Unique is the distinct-language count surfaced in the count
	// header — mirrors upstream `plugins.languages.unique`. Computed
	// across all analyzed repositories before favorites truncation.
	Unique int `json:"unique,omitempty"`
}

// IsSkipped lets the classic dispatcher (and any duck-typed consumer)
// detect the skipped path uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// inputs collects the resolved plugin_languages_* options after applying
// defaults. Keeping them on one struct keeps the aggregation loop tidy.
type inputs struct {
	limit     int
	threshold float64
	other     bool
	ignored   map[string]struct{}
	skipped   map[string]struct{}
	aliases   map[string]string
	colors    map[string]string
	sections  []string
	details   []string
}

// Run aggregates language bytes across base.Computed.RepositoryList.
// Returns a *Result; never returns a non-nil error in standard mode
// (the contract reserves *RetryableError for plugins that hit the
// network).
func (p *languagesPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)
	repos := pc.Data.Computed.RepositoryList
	if len(repos) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no repositories",
			Sections:      in.sections,
			Favorites:     []plugins.LanguageStat{},
			Colors:        map[string]string{},
		}, nil
	}

	// Per-language accumulators. counts tracks how many repositories
	// surfaced each language (after aliasing) so the upstream `count`
	// field stays accurate when aliases collapse two languages into one.
	type acc struct {
		size  int
		count int
		color string
	}
	totals := map[string]*acc{}
	for _, repo := range repos {
		if _, drop := in.skipped[repo.NameWithOwner]; drop {
			continue
		}
		seen := map[string]struct{}{}
		for _, lang := range repo.Languages {
			name := canonicalLanguage(lang.Name, in.aliases)
			if _, drop := in.ignored[name]; drop {
				continue
			}
			if name == "" {
				continue
			}
			a, ok := totals[name]
			if !ok {
				a = &acc{}
				totals[name] = a
			}
			a.size += lang.Size
			color := lang.Color
			if override, ok := in.colors[name]; ok {
				color = override
			}
			if a.color == "" {
				a.color = color
			}
			if _, alreadyCounted := seen[name]; !alreadyCounted {
				a.count++
				seen[name] = struct{}{}
			}
		}
	}

	totalBytes := 0
	for _, a := range totals {
		totalBytes += a.size
	}
	if totalBytes == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no repositories",
			Sections:      in.sections,
			Favorites:     []plugins.LanguageStat{},
			Colors:        map[string]string{},
		}, nil
	}

	stats := make([]plugins.LanguageStat, 0, len(totals))
	for name, a := range totals {
		stats = append(stats, plugins.LanguageStat{
			Name:  name,
			Color: a.color,
			Size:  a.size,
			Count: a.count,
			Value: float64(a.size) / float64(totalBytes),
		})
	}
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Size != stats[j].Size {
			return stats[i].Size > stats[j].Size
		}
		return stats[i].Name < stats[j].Name
	})

	limit := in.limit
	if limit < 0 {
		limit = 0
	}
	if limit > len(stats) {
		limit = len(stats)
	}
	favorites := append([]plugins.LanguageStat(nil), stats[:limit]...)

	other := plugins.LanguageStat{
		Name:  "Other",
		Color: "#cccccc",
	}
	if in.other && limit < len(stats) {
		for _, s := range stats[limit:] {
			other.Size += s.Size
			other.Count += s.Count
		}
		other.Value = float64(other.Size) / float64(totalBytes)
	}

	mostly := plugins.LanguageStat{}
	if len(favorites) > 0 {
		mostly = favorites[0]
	}
	if in.other && other.Value > in.threshold && other.Value > mostly.Value {
		mostly = other
	}

	colors := map[string]string{}
	for _, s := range favorites {
		if s.Color != "" {
			colors[s.Name] = s.Color
		}
	}
	if other.Color != "" {
		colors[other.Name] = other.Color
	}

	// Upstream index.mjs:33-34 — when indepth mode is disabled, "lines"
	// is filtered out of details. The EJS template then doesn't render
	// the lines column (which would otherwise show "0 lines" since no
	// linguist line counts exist outside indepth mode).
	details := append([]string(nil), in.details...)
	if !truthy(pc.Inputs["plugin_languages_indepth"]) {
		filtered := details[:0]
		for _, d := range details {
			if d != "lines" {
				filtered = append(filtered, d)
			}
		}
		details = filtered
	}

	return &Result{
		Mode:      plugins.AggregationMode(pc.Data),
		Favorites: favorites,
		Other:     other,
		Sections:  in.sections,
		Mostly:    mostly,
		Colors:    colors,
		Details:   details,
		Unique:    len(totals),
	}, nil
}

// canonicalLanguage applies the aliases map, returning the resolved
// language name. Aliases are case-insensitive on the source key per
// upstream behavior.
func canonicalLanguage(name string, aliases map[string]string) string {
	if name == "" {
		return ""
	}
	if mapped, ok := aliases[name]; ok {
		return mapped
	}
	lower := strings.ToLower(name)
	if mapped, ok := aliases[lower]; ok {
		return mapped
	}
	return name
}

// parseInputs collapses pc.Inputs into a typed view with defaults
// applied per contracts/plugin-p1-mvp.md §1.1.
func parseInputs(in map[string]any) inputs {
	out := inputs{
		limit:    8,
		other:    true,
		sections: []string{"most-used"},
		ignored:  map[string]struct{}{},
		skipped:  map[string]struct{}{},
		aliases:  map[string]string{},
		colors:   map[string]string{},
	}
	if v, ok := readInt(in, "plugin_languages_limit"); ok {
		out.limit = v
	}
	if v, ok := in["plugin_languages_threshold"]; ok {
		out.threshold = parseThreshold(fmt.Sprint(v))
	}
	if v, ok := in["plugin_languages_other"]; ok {
		out.other = truthy(v)
	}
	for _, s := range readCSV(in, "plugin_languages_ignored") {
		out.ignored[s] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_languages_skipped") {
		out.skipped[s] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_languages_aliases") {
		from, to, ok := splitPair(s, ":")
		if !ok {
			continue
		}
		out.aliases[from] = to
		out.aliases[strings.ToLower(from)] = to
	}
	for _, s := range readCSV(in, "plugin_languages_colors") {
		from, to, ok := splitPair(s, ":")
		if !ok {
			continue
		}
		out.colors[from] = to
	}
	if v, ok := in["plugin_languages_sections"]; ok {
		out.sections = readCSVValue(v)
		if len(out.sections) == 0 {
			out.sections = []string{"most-used"}
		}
	}
	// 011 v2: plugin_languages_details — mjun0812 uses
	// "bytes-size, percentage, lines".
	if v, ok := in["plugin_languages_details"]; ok {
		out.details = readCSVValue(v)
	}
	return out
}

// parseThreshold accepts "0%", "5%", "0.05" and returns a 0..1 value.
func parseThreshold(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	pct := strings.HasSuffix(s, "%")
	if pct {
		s = strings.TrimSuffix(s, "%")
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	if pct {
		return v / 100.0
	}
	return v
}

func readInt(in map[string]any, key string) (int, bool) {
	v, ok := in[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func readCSV(in map[string]any, key string) []string {
	v, ok := in[key]
	if !ok {
		return nil
	}
	return readCSVValue(v)
}

func readCSVValue(v any) []string {
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		parts := strings.Split(x, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

func splitPair(s, sep string) (string, string, bool) {
	i := strings.Index(s, sep)
	if i < 0 {
		return "", "", false
	}
	left := strings.TrimSpace(s[:i])
	right := strings.TrimSpace(s[i+len(sep):])
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}
