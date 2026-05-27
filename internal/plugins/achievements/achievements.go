// Package achievements owns the M4 "achievements" plugin. It converts
// base-aggregated statistics into rank-graded badges per upstream's
// metrics achievements panel.
package achievements

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "achievements"

const (
	displayDetailed = "detailed"
	displayCompact  = "compact"
)

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &achievementsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type achievementsPlugin struct{}

func (p *achievementsPlugin) Name() string                     { return Name }
func (p *achievementsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["achievements"].
type Result struct {
	Skipped       bool              `json:"skipped,omitempty"`
	SkippedReason string            `json:"-"`
	Display       string            `json:"display"`
	List          []Achievement     `json:"list"`
	Ranks         map[string]string `json:"ranks"`
}

// IsSkipped lets the classic dispatcher detect the skipped path
// uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Achievement is one rank-graded entry.
type Achievement struct {
	ID          string `json:"id"`
	Rank        string `json:"rank"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Value       int    `json:"value"`
}

// rankSpec lists the per-achievement thresholds. The thresholds (S/A/
// B/C) mirror upstream's "achievements/source/index.mjs" rank.json
// defaults; concrete values may drift over time but the shape is fixed.
type rankSpec struct {
	id          string
	title       string
	description string
	icon        string
	tiers       map[string]int // rank -> minimum value for that rank
}

var rankTable = []rankSpec{
	{
		id:          "commits",
		title:       "Worker",
		description: "Total commits across owned repositories",
		icon:        "git-commit",
		tiers:       map[string]int{"S": 5000, "A": 1000, "B": 500, "C": 100},
	},
	{
		id:          "repositories",
		title:       "Member",
		description: "Public repositories owned",
		icon:        "repo",
		tiers:       map[string]int{"S": 100, "A": 50, "B": 25, "C": 5},
	},
	{
		id:          "stars",
		title:       "Stargazer",
		description: "Total stars received across owned repositories",
		icon:        "star",
		tiers:       map[string]int{"S": 5000, "A": 1000, "B": 100, "C": 10},
	},
	{
		id:          "followers",
		title:       "Influencer",
		description: "GitHub followers",
		icon:        "people",
		tiers:       map[string]int{"S": 5000, "A": 1000, "B": 100, "C": 10},
	},
	{
		id:          "issues",
		title:       "Polyglot",
		description: "Issues opened across owned repositories",
		icon:        "issue-opened",
		tiers:       map[string]int{"S": 1000, "A": 500, "B": 100, "C": 10},
	},
	{
		id:          "pull-requests",
		title:       "Engineer",
		description: "Pull requests opened across owned repositories",
		icon:        "git-pull-request",
		tiers:       map[string]int{"S": 1000, "A": 500, "B": 100, "C": 10},
	},
}

var rankOrder = []string{"S", "A", "B", "C"}

type inputs struct {
	display   string
	threshold string
	only      map[string]struct{}
	ignored   map[string]struct{}
	limit     int
}

func (p *achievementsPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			Display:       in.display,
			List:          []Achievement{},
			Ranks:         map[string]string{},
		}, nil
	}
	c := pc.Data.Computed
	repos := c.Repositories
	if repos.Count == 0 && c.TotalCommits == 0 && len(c.RepositoryList) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "base data unavailable",
			Display:       in.display,
			List:          []Achievement{},
			Ranks:         map[string]string{},
		}, nil
	}

	values := map[string]int{
		"commits":       c.TotalCommits,
		"repositories":  repos.Count,
		"stars":         repos.Stargazers,
		"followers":     0,
		"issues":        repos.Issues,
		"pull-requests": repos.PullRequests,
	}

	list := make([]Achievement, 0, len(rankTable))
	ranks := make(map[string]string, len(rankTable))
	for _, spec := range rankTable {
		if !shouldInclude(spec.id, in) {
			continue
		}
		val := values[spec.id]
		rank := rankOf(val, spec.tiers)
		ranks[spec.id] = rank
		if !meetsThreshold(rank, in.threshold) {
			continue
		}
		list = append(list, Achievement{
			ID:          spec.id,
			Rank:        rank,
			Title:       spec.title,
			Description: spec.description,
			Icon:        spec.icon,
			Value:       val,
		})
	}
	sort.SliceStable(list, func(i, j int) bool {
		ri, rj := rankWeight(list[i].Rank), rankWeight(list[j].Rank)
		if ri != rj {
			return ri > rj
		}
		return list[i].ID < list[j].ID
	})
	if in.limit > 0 && len(list) > in.limit {
		list = list[:in.limit]
	}
	return &Result{Display: in.display, List: list, Ranks: ranks}, nil
}

func shouldInclude(id string, in inputs) bool {
	if _, drop := in.ignored[id]; drop {
		return false
	}
	if len(in.only) > 0 {
		if _, ok := in.only[id]; !ok {
			return false
		}
	}
	return true
}

// rankOf returns the highest rank whose threshold val meets.
func rankOf(val int, tiers map[string]int) string {
	for _, r := range rankOrder {
		if val >= tiers[r] {
			return r
		}
	}
	return "X"
}

func meetsThreshold(rank, threshold string) bool {
	return rankWeight(rank) >= rankWeight(threshold)
}

func rankWeight(r string) int {
	switch r {
	case "S":
		return 4
	case "A":
		return 3
	case "B":
		return 2
	case "C":
		return 1
	case "X":
		return 0
	}
	return 0
}

func parseInputs(in map[string]any) inputs {
	out := inputs{
		display:   displayDetailed,
		threshold: "C",
		only:      map[string]struct{}{},
		ignored:   map[string]struct{}{},
	}
	if v, ok := in["plugin_achievements_display"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.display = normalizeDisplay(s)
		}
	}
	if v, ok := in["plugin_achievements_threshold"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.threshold = strings.ToUpper(strings.TrimSpace(s))
		}
	}
	for _, s := range readCSV(in, "plugin_achievements_only") {
		out.only[s] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_achievements_ignored") {
		out.ignored[s] = struct{}{}
	}
	if v, ok := readInt(in, "plugin_achievements_limit"); ok {
		out.limit = v
	}
	return out
}

func normalizeDisplay(display string) string {
	switch strings.ToLower(strings.TrimSpace(display)) {
	case displayCompact:
		return displayCompact
	default:
		return displayDetailed
	}
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
