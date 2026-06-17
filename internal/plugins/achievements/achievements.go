// Package achievements owns the M4 "achievements" plugin. It converts
// base-aggregated statistics into rank-graded badges per upstream's
// metrics achievements panel.
//
// The 18-entry rank table mirrors upstream
// `org_repo/source/plugins/achievements/list/users.mjs`. The six
// upstream achievements not adopted are Manager (deprecated Projects
// classic), Verified / Explorer (HTML scraping), Automator /
// Infographile / Octonaut (secret / env-specific badges).
//
// Each entry's value source is a `func(*plugins.Data) int` so the
// table stays parseable and the lookup happens at run time against
// whichever Data fields the base plugin has already populated.
package achievements

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

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

// rankSpec lists the per-achievement thresholds. The thresholds vector
// is `[C, B, A, S, X-max]` taken verbatim from upstream
// `users.mjs`'s `rank(value, [...])` invocations: a value at or above
// `thresholds[3]` ranks S, `[2]..[3)` ranks A, `[1]..[2)` ranks B,
// `[0]..[1)` ranks C, and anything below `thresholds[0]` (i.e. 0)
// ranks X.
type rankSpec struct {
	id          string
	title       string
	icon        string
	thresholds  [4]int // C, B, A, S minimums
	value       func(*plugins.Data) int
	description func(value int) string
}

// rankTable enumerates the 18 adopted achievements in upstream order.
// Thresholds and descriptions mirror users.mjs verbatim.
var rankTable = []rankSpec{
	{
		id:         "developer",
		title:      "Developer",
		icon:       "repo",
		thresholds: [4]int{1, 20, 50, 100},
		value: func(d *plugins.Data) int {
			return d.Computed.Repositories.Count
		},
		description: func(v int) string {
			return fmt.Sprintf("Published %d public %s", v, pluralY(v, "repositor"))
		},
	},
	{
		id:         "forker",
		title:      "Forker",
		icon:       "repo-forked",
		thresholds: [4]int{1, 5, 10, 20},
		value: func(d *plugins.Data) int {
			n := 0
			for _, r := range d.Computed.RepositoryList {
				if r.IsFork {
					n++
				}
			}
			return n
		},
		description: func(v int) string {
			return fmt.Sprintf("Forked %d public %s", v, pluralY(v, "repositor"))
		},
	},
	{
		id:         "contributor",
		title:      "Contributor",
		icon:       "git-pull-request",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.PullRequestsOpened
		},
		description: func(v int) string {
			return fmt.Sprintf("Opened %d pull %s", v, plural(v, "request"))
		},
	},
	{
		id:         "reviewer",
		title:      "Reviewer",
		icon:       "eye",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.PullRequestsReviewed
		},
		description: func(v int) string {
			return fmt.Sprintf("Reviewed %d pull %s", v, plural(v, "request"))
		},
	},
	{
		id:         "packager",
		title:      "Packager",
		icon:       "package",
		thresholds: [4]int{1, 5, 10, 20},
		value: func(d *plugins.Data) int {
			return d.Computed.Repositories.Packages
		},
		description: func(v int) string {
			return fmt.Sprintf("Created %d %s", v, plural(v, "package"))
		},
	},
	{
		id:         "gister",
		title:      "Gister",
		icon:       "code",
		thresholds: [4]int{1, 20, 50, 100},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Gists
		},
		description: func(v int) string {
			return fmt.Sprintf("Published %d %s", v, plural(v, "gist"))
		},
	},
	{
		id:         "worker",
		title:      "Worker",
		icon:       "organization",
		thresholds: [4]int{1, 2, 4, 8},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Organizations
		},
		description: func(v int) string {
			return fmt.Sprintf("Member of %d %s", v, plural(v, "organization"))
		},
	},
	{
		id:         "stargazer",
		title:      "Stargazer",
		icon:       "star",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Starred
		},
		description: func(v int) string {
			return fmt.Sprintf("Starred %d %s", v, pluralY(v, "repositor"))
		},
	},
	{
		id:         "follower",
		title:      "Follower",
		icon:       "person-add",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Following
		},
		description: func(v int) string {
			return fmt.Sprintf("Following %d %s", v, plural(v, "user"))
		},
	},
	{
		id:         "influencer",
		title:      "Influencer",
		icon:       "people",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Followers
		},
		description: func(v int) string {
			return fmt.Sprintf("Followed by %d %s", v, plural(v, "user"))
		},
	},
	{
		id:         "maintainer",
		title:      "Maintainer",
		icon:       "tools",
		thresholds: [4]int{1, 1000, 5000, 10000},
		value: func(d *plugins.Data) int {
			n := 0
			for _, r := range d.Computed.RepositoryList {
				if r.Stars > n {
					n = r.Stars
				}
			}
			return n
		},
		description: func(v int) string {
			return fmt.Sprintf("Most popular repository has %d %s", v, plural(v, "star"))
		},
	},
	{
		id:         "inspirer",
		title:      "Inspirer",
		icon:       "repo-forked",
		thresholds: [4]int{1, 100, 500, 1000},
		value: func(d *plugins.Data) int {
			n := 0
			for _, r := range d.Computed.RepositoryList {
				if r.Forks > n {
					n = r.Forks
				}
			}
			return n
		},
		description: func(v int) string {
			return fmt.Sprintf("Most forked repository has %d %s", v, plural(v, "fork"))
		},
	},
	{
		id:         "polyglot",
		title:      "Polyglot",
		icon:       "code-square",
		thresholds: [4]int{1, 4, 8, 16},
		value: func(d *plugins.Data) int {
			seen := map[string]struct{}{}
			for _, r := range d.Computed.RepositoryList {
				for _, l := range r.Languages {
					if l.Name == "" {
						continue
					}
					seen[l.Name] = struct{}{}
				}
			}
			return len(seen)
		},
		description: func(v int) string {
			return fmt.Sprintf("Has used %d programming %s across owned repositories", v, plural(v, "language"))
		},
	},
	{
		id:         "member",
		title:      "Member",
		icon:       "calendar",
		thresholds: [4]int{1, 3, 5, 10},
		value: func(d *plugins.Data) int {
			if d.User == nil || d.User.CreatedAt.IsZero() {
				return 0
			}
			years := time.Since(d.User.CreatedAt).Hours() / (24 * 365.25)
			if years <= 0 {
				return 0
			}
			return int(math.Floor(years))
		},
		description: func(v int) string {
			return fmt.Sprintf("Registered %d %s ago", v, plural(v, "year"))
		},
	},
	{
		id:         "sponsor",
		title:      "Sponsor",
		icon:       "heart",
		thresholds: [4]int{1, 3, 5, 10},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.Sponsoring
		},
		description: func(v int) string {
			return fmt.Sprintf("Sponsored %d %s", v, plural(v, "user/organization"))
		},
	},
	{
		id:         "deployer",
		title:      "Deployer",
		icon:       "rocket",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			return d.Computed.Repositories.Deployments
		},
		description: func(v int) string {
			return fmt.Sprintf("Performed %d %s across owned repositories", v, plural(v, "deployment"))
		},
	},
	{
		id:         "chatter",
		title:      "Chatter",
		icon:       "comment-discussion",
		thresholds: [4]int{1, 200, 500, 1000},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.DiscussionsStarted + d.User.DiscussionsComments
		},
		description: func(v int) string {
			return fmt.Sprintf("Started or commented on %d %s", v, plural(v, "discussion"))
		},
	},
	{
		id:         "helper",
		title:      "Helper",
		icon:       "comment",
		thresholds: [4]int{1, 20, 50, 100},
		value: func(d *plugins.Data) int {
			if d.User == nil {
				return 0
			}
			return d.User.DiscussionAnswers
		},
		description: func(v int) string {
			return fmt.Sprintf("Marked as answer on %d %s", v, plural(v, "discussion"))
		},
	},
}

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
	if c.Repositories.Count == 0 && c.TotalCommits == 0 && len(c.RepositoryList) == 0 && pc.Data.User == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "base data unavailable",
			Display:       in.display,
			List:          []Achievement{},
			Ranks:         map[string]string{},
		}, nil
	}

	list := make([]Achievement, 0, len(rankTable))
	ranks := make(map[string]string, len(rankTable))
	for _, spec := range rankTable {
		if !shouldInclude(spec.id, in) {
			continue
		}
		val := spec.value(pc.Data)
		rank := rankOf(val, spec.thresholds)
		ranks[spec.id] = rank
		if !meetsThreshold(rank, in.threshold) {
			continue
		}
		list = append(list, Achievement{
			ID:          spec.id,
			Rank:        rank,
			Title:       spec.title,
			Description: spec.description(val),
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

// rankOf returns the highest rank whose threshold val meets. The
// thresholds vector lists the C/B/A/S minimums in ascending order
// (mirroring upstream `rank(value, [C, B, A, S, X-max])`).
func rankOf(val int, thresholds [4]int) string {
	switch {
	case val >= thresholds[3]:
		return "S"
	case val >= thresholds[2]:
		return "A"
	case val >= thresholds[1]:
		return "B"
	case val >= thresholds[0]:
		return "C"
	default:
		return "X"
	}
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

// plural mirrors upstream `imports.s(value)`: append "s" unless value == 1.
func plural(v int, word string) string {
	if v == 1 {
		return word
	}
	return word + "s"
}

// pluralY mirrors upstream `imports.s(value, "y")`: switch the trailing
// "y" suffix to "ies" when value != 1. The base is supplied without
// the "y" / "ies" tail so callers can write `pluralY(v, "repositor")`.
func pluralY(v int, base string) string {
	if v == 1 {
		return base + "y"
	}
	return base + "ies"
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
