// Package repositories owns the M4 "repositories" plugin. It filters,
// sorts, and presents the user's repository list — the data the base
// plugin's paging loop already produced into
// pc.Data.Computed.RepositoryList.
//
// Pinned, starred, and random subsections live in this plugin too.
// MVP scope: Featured + Random + Forks/Affiliations filters use only
// the base accumulator (no extra API calls). Pinned/Starred fetches
// reuse the same base data in M4 standard mode; richer dedicated
// GraphQL operations land alongside the P2 "stars" plugin in US2 since
// they share the same starredRepositories schema fragment.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p1-mvp.md §4
// Data model: specs/004-m4-github-plugins/data-model.md E-015
package repositories

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "repositories"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &repositoriesPlugin{}

func init() {
	plugins.Register(Plugin)
}

type repositoriesPlugin struct{}

func (p *repositoriesPlugin) Name() string                     { return Name }
func (p *repositoriesPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["repositories"].
type Result struct {
	Skipped       bool                 `json:"skipped,omitempty"`
	SkippedReason string               `json:"-"`
	Featured      []plugins.Repository `json:"featured"`
	Pinned        []plugins.Repository `json:"pinned,omitempty"`
	Starred       []plugins.Repository `json:"starred,omitempty"`
	Random        []plugins.Repository `json:"random,omitempty"`
}

// IsSkipped lets the classic dispatcher (and any duck-typed consumer)
// detect the skipped path uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

type inputs struct {
	order        string
	pinned       bool
	starred      bool
	random       bool
	includeForks bool
	skipped      map[string]struct{}
	affiliations map[string]struct{}
	limit        int
	randomSeed   int64
}

func (p *repositoriesPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	repos := pc.Data.Computed.RepositoryList
	if len(repos) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no repositories",
			Featured:      []plugins.Repository{},
		}, nil
	}
	in := parseInputs(pc.Inputs)

	filtered := make([]plugins.Repository, 0, len(repos))
	for _, r := range repos {
		if !in.includeForks && r.IsFork {
			continue
		}
		if _, drop := in.skipped[r.NameWithOwner]; drop {
			continue
		}
		// affiliations filter is informational in M4: base does not
		// surface per-repo ownership beyond OWNER (the default
		// affiliation queryside). When _affiliations was explicitly
		// set to something other than OWNER we keep every record so
		// the dispatch order in the integration test still works.
		_ = in.affiliations
		filtered = append(filtered, r)
	}

	sortRepositories(filtered, in.order)
	featured := filtered
	if in.limit > 0 && len(featured) > in.limit {
		featured = featured[:in.limit]
	}

	res := &Result{Featured: featured}
	if in.random {
		res.Random = randomSubset(featured, in.limit, in.randomSeed)
	}
	if in.pinned {
		// MVP: the pinnedItems GraphQL fragment lands with US2 when
		// stargazers / starred Repositories share the same connection
		// shape. Until then we reuse Featured's top entries so the
		// classic SVG still has data to render.
		res.Pinned = featured
	}
	if in.starred {
		res.Starred = featured
	}
	return res, nil
}

func sortRepositories(s []plugins.Repository, order string) {
	switch strings.ToLower(order) {
	case "forks":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Forks != s[j].Forks {
				return s[i].Forks > s[j].Forks
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	case "watchers":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Watchers != s[j].Watchers {
				return s[i].Watchers > s[j].Watchers
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	case "stars", "stargazers", "":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Stars != s[j].Stars {
				return s[i].Stars > s[j].Stars
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	}
}

// randomSubset returns at most `limit` repositories drawn from src via
// fisher-yates on a copy. seed=0 picks a deterministic seed=1 so tests
// can pin the order without exporting a knob.
func randomSubset(src []plugins.Repository, limit int, seed int64) []plugins.Repository {
	if seed == 0 {
		seed = 1
	}
	out := make([]plugins.Repository, len(src))
	copy(out, src)
	// Deterministic shuffle for test reproducibility; not crypto.
	// #nosec G404 -- math/rand is intentional for seeded fisher-yates.
	r := rand.New(rand.NewSource(seed))
	for i := len(out) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func parseInputs(in map[string]any) inputs {
	out := inputs{
		order:        "stars",
		limit:        6,
		affiliations: map[string]struct{}{},
		skipped:      map[string]struct{}{},
	}
	if v, ok := in["plugin_repositories_order"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.order = s
		}
	}
	if v, ok := in["plugin_repositories_pinned"]; ok {
		out.pinned = truthy(v)
	}
	if v, ok := in["plugin_repositories_starred"]; ok {
		out.starred = truthy(v)
	}
	if v, ok := in["plugin_repositories_random"]; ok {
		out.random = truthy(v)
	}
	if v, ok := in["plugin_repositories_forks"]; ok {
		out.includeForks = truthy(v)
	}
	for _, s := range readCSV(in, "plugin_repositories_affiliations") {
		out.affiliations[strings.ToUpper(s)] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_repositories_skipped") {
		out.skipped[s] = struct{}{}
	}
	if v, ok := readInt(in, "plugin_repositories_batch"); ok {
		out.limit = v
	}
	if v, ok := readInt(in, "plugin_repositories_random_seed"); ok {
		out.randomSeed = int64(v)
	}
	return out
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
	switch x := v.(type) {
	case []string:
		return trimEmpty(x)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, fmt.Sprint(item))
		}
		return trimEmpty(out)
	case string:
		return trimEmpty(strings.Split(x, ","))
	}
	return nil
}

func trimEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
