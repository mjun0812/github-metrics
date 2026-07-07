// Package starlists owns the M4 "starlists" plugin. It uses GitHub's
// `user.lists` GraphQL field (verified live, May 2026) to fetch
// starlists + their items in one round trip. No headless browser
// required.
//
// Same Navigator-based test seam as the topics plugin: production code
// uses graphqlNavigator from graphql_navigator.go; non-network tests
// substitute a fake.
package starlists

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
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

func (p *starlistsPlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{plugins.KeyRepositories}
}

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
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Count        int                    `json:"count"`
	Languages    []plugins.LanguageStat `json:"languages,omitempty"`
	Repositories []Repository           `json:"repositories,omitempty"`
	URL          string                 `json:"-"` // internal: used by language analysis
	Repos        []string               `json:"-"` // internal: repos extracted from detail page
}

// Repository is one repo card rendered inside a starlist (upstream EJS
// `<div class="repositories">` block). It carries only the fields the
// user.lists GraphQL items already project — name + description; no
// extra per-repo fetch is performed. IsPrivate is kept internal so the
// repositories_skip_private filter can drop private repos before render.
type Repository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"-"`
}

// Navigator abstracts the upstream-data fetch so non-network tests can
// inject a fake. FetchLists returns the list overview; FetchRepos
// enumerates a single list's repos for language analysis. The
// production implementation is graphqlNavigator (single GraphQL query
// answers both — see graphql_navigator.go).
type Navigator interface {
	FetchLists(ctx context.Context, url string) ([]Starlist, error)
	FetchRepos(ctx context.Context, listURL string) ([]string, error)
}

// NavigatorKey is the inputs map slot tests use to inject a fake
// Navigator. Production code never sets this — the plugin falls back to
// pc.Render.
const NavigatorKey = "_test_starlists_navigator"

type starlistsInputs struct {
	limit             int
	limitRepositories int
	languages         bool
	skipPrivate       bool
}

func (p *starlistsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)

	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	// Gate on the user-facing `plugin_starlists` input so the plugin
	// stays silent (no Data.Errors entries) when never requested.
	if !pluginutil.Truthy(pc.Inputs["plugin_starlists"]) {
		return &Result{
			Skipped:       true,
			SkippedReason: "plugin_starlists not enabled",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	if !pluginutil.ExtrasEnabled(pc.Inputs, "extras.metrics.run.puppeteer.scrapping") {
		return &Result{
			Skipped:       true,
			SkippedReason: "puppeteer scrapping disabled via extras",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{
			Skipped:       true,
			SkippedReason: "no login",
			List:          []Starlist{},
			Languages:     in.languages,
		}, nil
	}

	nav := pickNavigator(pc, login, in)
	if nav == nil {
		// The GraphQL client is absent (test harnesses with no API
		// dep). Skip cleanly and record a retryable error so the
		// engine surfaces the degraded path on Result.Errors.
		pc.Data.AppendError(xerrors.NewRetryableError(
			fmt.Errorf("starlists: graphql client not available"),
		))
		return &Result{
			Skipped:       true,
			SkippedReason: "graphql client not available",
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

	for i := range list {
		list[i].Repositories = limitRepositories(list[i].Repositories, in.limitRepositories, in.skipPrivate)
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
	for _, r := range resolveRepositories(ctx, pc) {
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

// limitRepositories drops private repos when skipPrivate is set
// (repositories_skip_private, #656) then truncates to limit. limit <= 0
// means "unlimited" (see parseInputs). Returns nil when nothing
// survives so the partial omits the `<div class="repositories">`
// wrapper entirely.
func limitRepositories(repos []Repository, limit int, skipPrivate bool) []Repository {
	if len(repos) == 0 {
		return nil
	}
	out := repos
	if skipPrivate {
		out = out[:0:0]
		for _, r := range repos {
			if r.IsPrivate {
				continue
			}
			out = append(out, r)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pickNavigator returns the test-injected Navigator if present.
// Otherwise it constructs a graphqlNavigator on top of pc.GraphQL;
// nil when pc.GraphQL is unset (test harnesses without GraphQL
// wiring).
func pickNavigator(pc *plugins.PluginContext, login string, in starlistsInputs) Navigator {
	if pc != nil && pc.Inputs != nil {
		if v, ok := pc.Inputs[NavigatorKey]; ok {
			if n, ok := v.(Navigator); ok {
				return n
			}
		}
	}
	if pc == nil || pc.GraphQL == nil {
		return nil
	}
	listsFirst := in.limit
	if listsFirst <= 0 {
		listsFirst = 10
	}
	// GitHub's GraphQL caps `first` at 100; keep items capped at 50 to
	// stay well inside the per-query node budget. Items rarely exceed
	// that in real starlists.
	return NewGraphQLNavigator(pc.GraphQL, login, listsFirst, 50)
}

// resolveRepositories reads the per-node accumulator via the shared
// dataprovider (#603), falling back to pc.Data.Computed.RepositoryList
// for unit tests that build PluginContext by hand without wiring a
// Provider.
//
// In repository-template mode (Account == AccountRepository) the
// Provider returns the synthesized single-element list wrapping the
// target repo (see dataprovider.synthesizeRepoResult), so callers can
// dispatch through Provider in both modes without a special case.
func resolveRepositories(ctx context.Context, pc *plugins.PluginContext) []plugins.Repository {
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

func parseInputs(in map[string]any) starlistsInputs {
	out := starlistsInputs{
		limit:             4,
		limitRepositories: 2,
		languages:         false,
	}
	if v, ok := pluginutil.ReadInt(in, "plugin_starlists_limit"); ok {
		out.limit = v
	}
	// plugin_starlists_limit_repositories has no "zero: disable" in
	// metadata.yml, so 0 means "unlimited" — matching the sibling
	// plugin_starlists_limit handling below (in.limit > 0 guard).
	if v, ok := pluginutil.ReadInt(in, "plugin_starlists_limit_repositories"); ok {
		out.limitRepositories = v
	}
	if v, ok := in["plugin_starlists_languages"]; ok {
		out.languages = pluginutil.Truthy(v)
	}
	out.skipPrivate = pluginutil.Truthy(in["repositories_skip_private"])
	return out
}
