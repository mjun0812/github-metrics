// Package languages owns the M4 "languages" plugin in its standard
// (most-used) mode. It aggregates per-repository language byte
// breakdowns the base plugin paged into pc.Data.Computed.RepositoryList
// and surfaces them as Favorites + Other + Mostly, mirroring upstream's
// data.plugins.languages JSON shape.
//
// Recent-mode and indepth-mode (P3, build tag heavy) ship in separate
// files (recent.go / indepth.go). Standard mode never issues additional
// API calls — it relies entirely on the data base already collected.
package languages

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "languages"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &languagesPlugin{}

func init() {
	plugins.Register(Plugin)
}

type languagesPlugin struct{}

func (p *languagesPlugin) Name() string { return Name }

func (p *languagesPlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{plugins.KeyRepositories}
}

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
func (p *languagesPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)
	// Upstream parity (#537): every plugin defaults to off — see
	// org_repo/source/app/metrics/index.mjs:143-148. The repository
	// template's fixed partial order
	// (assets/templates/repository/partials/_.json) includes
	// "languages", and internal/dataprovider's repository-mode
	// Provider synthesizes a single-repo RepositoryList regardless of
	// chrome/base inputs, so without this gate every repo-mode
	// sample still leaked a `<g data-section="languages">`
	// (plugin-base-repo / plugin-people-repo* /
	// plugin-contributors-repo-contributions / metrics-repository).
	// Gating here also keeps the JSON output
	// upstream-parity by omitting languages from data.plugins when
	// the toggle is off.
	if !pluginutil.Truthy(pc.Inputs["plugin_languages"]) {
		return &Result{
			Skipped:       true,
			SkippedReason: "plugin_languages not enabled",
			Sections:      in.sections,
			Favorites:     []plugins.LanguageStat{},
			Colors:        map[string]string{},
		}, nil
	}
	repos := resolveRepositoryList(ctx, pc)
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
	// Upstream `repositories_forks: no` is the default (org_repo/source/
	// plugins/base/metadata.yml line 88). Without this filter, language
	// stats from forked repos (e.g. a fork of a large EJS codebase)
	// pollute the user's distribution with code they didn't write.
	// Mirror upstream's default by skipping forks unless the caller
	// explicitly opts in via `plugin_repositories_forks` / `repositories_forks`.
	includeForks := pluginutil.Truthy(pc.Inputs["plugin_repositories_forks"]) ||
		pluginutil.Truthy(pc.Inputs["repositories_forks"])

	totals := map[string]*acc{}
	for _, repo := range repos {
		if _, drop := in.skipped[repo.NameWithOwner]; drop {
			continue
		}
		if repo.IsFork && !includeForks {
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
	if !pluginutil.Truthy(pc.Inputs["plugin_languages_indepth"]) {
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
	if v, ok := pluginutil.ReadInt(in, "plugin_languages_limit"); ok {
		out.limit = v
	}
	if v, ok := in["plugin_languages_threshold"]; ok {
		out.threshold = parseThreshold(fmt.Sprint(v))
	}
	if v, ok := in["plugin_languages_other"]; ok {
		out.other = pluginutil.Truthy(v)
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_languages_ignored") {
		out.ignored[s] = struct{}{}
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_languages_skipped") {
		out.skipped[s] = struct{}{}
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_languages_aliases") {
		from, to, ok := splitPair(s, ":")
		if !ok {
			continue
		}
		out.aliases[from] = to
		out.aliases[strings.ToLower(from)] = to
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_languages_colors") {
		from, to, ok := splitPair(s, ":")
		if !ok {
			continue
		}
		out.colors[from] = to
	}
	if v, ok := in["plugin_languages_sections"]; ok {
		out.sections = pluginutil.ReadCSVValue(v)
		if len(out.sections) == 0 {
			out.sections = []string{"most-used"}
		}
	}
	// 011 v2: plugin_languages_details — mjun0812 uses
	// "bytes-size, percentage, lines".
	if v, ok := in["plugin_languages_details"]; ok {
		out.details = pluginutil.ReadCSVValue(v)
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

// resolveRepositoryList reads the paged repository accumulator via the
// shared dataprovider (#603), falling back to
// pc.Data.Computed.RepositoryList for unit tests that build
// PluginContext by hand without wiring a Provider.
//
// In repository-template mode (Account == AccountRepository) the
// Provider's Repositories accessor returns the synthesized
// single-element list wrapping the target repo (see
// dataprovider.synthesizeRepoResult), so callers can dispatch through
// Provider in both modes without a special case.
func resolveRepositoryList(ctx context.Context, pc *plugins.PluginContext) []plugins.Repository {
	if pc == nil {
		return nil
	}
	if pc.Provider != nil {
		if repos, err := pc.Provider.Repositories(ctx); err == nil && repos != nil {
			return repos
		}
	}
	if pc.Data != nil {
		return pc.Data.Computed.RepositoryList
	}
	return nil
}
