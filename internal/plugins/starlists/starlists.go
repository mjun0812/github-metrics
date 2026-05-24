// Package starlists owns the M4 "starlists" plugin. It scrapes the
// user's starred-lists landing page + each list's detail page via
// chromedp because the GitHub API does not expose starlists.
//
// Same Navigator-based test seam as the topics plugin: production
// chromedp scraping is isolated behind the Navigator interface so non-
// chromedp tests can exercise the skipped/short-circuit paths.
package starlists

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

// Name is the canonical plugin slug.
const Name = "starlists"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &starlistsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type starlistsPlugin struct{}

func (p *starlistsPlugin) Name() string                     { return Name }
func (p *starlistsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["starlists"].
// Field set mirrors data-model E-024.
type Result struct {
	Skipped       bool       `json:"skipped,omitempty"`
	SkippedReason string     `json:"-"`
	List          []Starlist `json:"list"`
	Languages     bool       `json:"languages,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Starlist is one starlist entry. When the user requested per-list
// language analysis (_languages=true), Languages holds the aggregated
// breakdown across the list's repositories.
type Starlist struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Count       int                    `json:"count"`
	Languages   []plugins.LanguageStat `json:"languages,omitempty"`
	URL         string                 `json:"-"` // internal: used by language analysis
	Repos       []string               `json:"-"` // internal: repos extracted from detail page
}

// Navigator abstracts the chromedp interactions so non-chromedp tests
// can inject a fake. The Fetch method returns the list overview; the
// FetchRepos method drills into a single list to enumerate its repos
// for language analysis.
type Navigator interface {
	FetchLists(ctx context.Context, url string) ([]Starlist, error)
	FetchRepos(ctx context.Context, listURL string) ([]string, error)
}

// NavigatorKey is the inputs map slot tests use to inject a fake
// Navigator. Production code never sets this — the plugin falls back to
// pc.Render.
const NavigatorKey = "_test_starlists_navigator"

type starlistsInputs struct {
	limit     int
	languages bool
}

func (p *starlistsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)

	// Gate on the user-facing `plugin_starlists` input so the plugin
	// stays silent (no Data.Errors entries) when never requested.
	if !truthy(pc.Inputs["plugin_starlists"]) {
		return &Result{
			Skipped:       true,
			SkippedReason: "plugin_starlists not enabled",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	if !extrasEnabled(pc.Inputs, "extras.metrics.run.puppeteer.scrapping") {
		return &Result{
			Skipped:       true,
			SkippedReason: "puppeteer scrapping disabled via extras",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	nav := pickNavigator(pc)
	if nav == nil {
		pc.Data.AppendError(xerrors.NewRetryableError(
			fmt.Errorf("starlists: chromedp not available"),
		))
		return &Result{
			Skipped:       true,
			SkippedReason: "chromedp not available",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{
			Skipped:       true,
			SkippedReason: "no login",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	url := fmt.Sprintf("https://github.com/stars/%s/lists", login)
	list, err := nav.FetchLists(ctx, url)
	if err != nil {
		return nil, xerrors.NewRetryableError(fmt.Errorf("starlists: %w", err))
	}

	if in.limit > 0 && len(list) > in.limit {
		list = list[:in.limit]
	}

	if in.languages {
		analyzeLanguages(ctx, pc, list, nav)
	}

	// Stable sort by name for deterministic output.
	sort.SliceStable(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	return &Result{
		List:      list,
		Languages: in.languages,
	}, nil
}

// analyzeLanguages drills into each starlist via Navigator.FetchRepos
// and joins the resulting repo names against base.Computed.RepositoryList
// to compute per-list language byte totals. Repos that aren't in
// Computed.RepositoryList contribute nothing (the GraphQL fetch for
// arbitrary external repos is out of scope for M4 — see contract §4.4).
func analyzeLanguages(ctx context.Context, pc *plugins.PluginContext, list []Starlist, nav Navigator) {
	repoIndex := map[string][]plugins.LanguageStat{}
	for _, r := range pc.Data.Computed.RepositoryList {
		repoIndex[strings.ToLower(r.NameWithOwner)] = r.Languages
	}
	for i := range list {
		select {
		case <-ctx.Done():
			return
		default:
		}
		repos, err := nav.FetchRepos(ctx, list[i].URL)
		if err != nil {
			pc.Data.AppendError(fmt.Errorf("starlists: list %q: %w", list[i].Name, err))
			continue
		}
		list[i].Repos = repos

		totals := map[string]*plugins.LanguageStat{}
		for _, repo := range repos {
			for _, lang := range repoIndex[strings.ToLower(repo)] {
				if lang.Name == "" {
					continue
				}
				agg, ok := totals[lang.Name]
				if !ok {
					agg = &plugins.LanguageStat{Name: lang.Name, Color: lang.Color}
					totals[lang.Name] = agg
				}
				agg.Size += lang.Size
				agg.Count++
			}
		}
		// Convert to slice, sorted by Size desc, name asc.
		stats := make([]plugins.LanguageStat, 0, len(totals))
		var total int
		for _, s := range totals {
			stats = append(stats, *s)
			total += s.Size
		}
		sort.SliceStable(stats, func(a, b int) bool {
			if stats[a].Size != stats[b].Size {
				return stats[a].Size > stats[b].Size
			}
			return stats[a].Name < stats[b].Name
		})
		if total > 0 {
			for j := range stats {
				stats[j].Value = float64(stats[j].Size) / float64(total)
			}
		}
		list[i].Languages = stats
	}
}

// pickNavigator returns the test-injected Navigator if present.
// Otherwise it returns a browserNavigator when pc.Render is a real
// *render.Browser; nil when it is nil or a *FakeRenderer.
func pickNavigator(pc *plugins.PluginContext) Navigator {
	if pc.Inputs != nil {
		if v, ok := pc.Inputs[NavigatorKey]; ok {
			if n, ok := v.(Navigator); ok {
				return n
			}
		}
	}
	browser, ok := pc.Render.(*render.Browser)
	if !ok || browser == nil {
		return nil
	}
	return &browserNavigator{browser: browser}
}

func parseInputs(in map[string]any) starlistsInputs {
	out := starlistsInputs{
		limit:     4,
		languages: false,
	}
	if v, ok := readInt(in, "plugin_starlists_limit"); ok {
		out.limit = v
	}
	if v, ok := in["plugin_starlists_languages"]; ok {
		out.languages = truthy(v)
	}
	return out
}

func loginFromInputs(in map[string]any) string {
	if in == nil {
		return ""
	}
	if v, ok := in["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := in["login"].(string); ok {
		return v
	}
	return ""
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

func extrasEnabled(in map[string]any, key string) bool {
	if in == nil {
		return true
	}
	v, ok := in[key]
	if !ok {
		return true
	}
	return truthy(v)
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
