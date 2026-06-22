// Package reactions owns the M4 "reactions" plugin. It aggregates the
// per-emoji reaction content on the user's issues + issue comments via
// GraphQL and renders the upstream 8-emoji gauge panel.
//
// Upstream (lowlighter/metrics) additionally covers discussions and
// discussion comments; those arms require extra scopes / a separate
// query surface and are deferred. The two issue-side sources reproduce
// the same gauge layout and per-content aggregation.
package reactions

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "reactions"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &reactionsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type reactionsPlugin struct{}

func (p *reactionsPlugin) Name() string                     { return Name }
func (p *reactionsPlugin) Metadata() *config.PluginMetadata { return nil }

// Reaction is the per-content aggregate published for one emoji
// category. Mirrors upstream's `list[content]` entry.
type Reaction struct {
	// Value is the raw number of reactions with this content.
	Value int `json:"value"`
	// Percentage is value / total reactions (absolute share).
	Percentage float64 `json:"percentage"`
	// Score drives the gauge arc: value / total in "absolute" display
	// mode, or value / max in "relative" display mode.
	Score float64 `json:"score"`
}

// Result is the JSON payload published under data.Plugins["reactions"].
type Result struct {
	Skipped       bool   `json:"skipped,omitempty"`
	SkippedReason string `json:"-"`
	// List is the per-content aggregate keyed by the GraphQL
	// ReactionContent enum (HEART, THUMBS_UP, ...).
	List map[string]Reaction `json:"list"`
	// Total is the total number of reactions collected.
	Total int `json:"total"`
	// Comments is the number of comments scanned (drives the header).
	Comments int `json:"comments"`
	// Details mirrors upstream `plugin_reactions_details`: the ordered
	// detail fields to render under each gauge (count / percentage).
	Details []string `json:"details,omitempty"`
	// Days is the comment maximum age (0 = disabled).
	Days int `json:"days"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Run issues a single GraphQL call to collect per-emoji reaction content
// across the user's issues and issue comments, then aggregates them into
// the upstream per-content list with value / percentage / score.
func (p *reactionsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{Skipped: true, SkippedReason: reason}, nil
	}
	if !pluginutil.TruthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled"}, nil
	}
	if pc.GraphQL == nil {
		return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable"}, nil
	}
	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login"}, nil
	}
	// Upstream defaults: plugin_reactions_limit=200 (issue comments),
	// plugin_reactions_limit_issues=100, plugin_reactions_days=0.
	commentsLimit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_reactions_limit", 200)
	issuesLimit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_reactions_limit_issues", 100)
	// GitHub's GraphQL connections reject first > 100 with
	// EXCESSIVE_PAGINATION, which fails the entire UserReactions query
	// and blanks the card. The upstream plugin_reactions_limit default
	// is 200, so clamp both connection sizes to GitHub's 100 ceiling. (#472)
	if commentsLimit > 100 {
		commentsLimit = 100
	}
	if issuesLimit > 100 {
		issuesLimit = 100
	}
	days := pluginutil.ReadIntDefault(pc.Inputs, "plugin_reactions_days", 0)
	display := readDisplay(pc.Inputs)
	details := readDetails(pc.Inputs)

	resp, err := pc.GraphQL.UserReactions(ctx, login, issuesLimit, commentsLimit)
	if err != nil {
		return nil, xerrors.NewRetryableError(fmt.Errorf("reactions: %w", err))
	}

	r := &Result{
		List:    map[string]Reaction{},
		Days:    days,
		Details: details,
	}
	if resp == nil || resp.User == nil {
		return r, nil
	}

	// Collect raw reaction contents and count scanned comments.
	counts := map[string]int{}
	comments := 0
	if resp.User.IssueComments != nil {
		for _, n := range resp.User.IssueComments.Nodes {
			if n == nil {
				continue
			}
			comments++
			if n.Reactions == nil {
				continue
			}
			for _, react := range n.Reactions.Nodes {
				if react == nil {
					continue
				}
				counts[string(react.Content)]++
			}
		}
	}
	if resp.User.Issues != nil {
		for _, n := range resp.User.Issues.Nodes {
			if n == nil {
				continue
			}
			comments++
			if n.Reactions == nil {
				continue
			}
			for _, react := range n.Reactions.Nodes {
				if react == nil {
					continue
				}
				counts[string(react.Content)]++
			}
		}
	}
	r.Comments = comments

	// Total reactions and per-content max (for relative display mode).
	total := 0
	max := 0
	for _, v := range counts {
		total += v
		if v > max {
			max = v
		}
	}
	r.Total = total

	for key, value := range counts {
		entry := Reaction{Value: value}
		if total > 0 {
			entry.Percentage = float64(value) / float64(total)
		}
		switch {
		case display == "relative" && max > 0:
			entry.Score = float64(value) / float64(max)
		case total > 0:
			entry.Score = float64(value) / float64(total)
		}
		r.List[key] = entry
	}

	return r, nil
}

// readDisplay returns the normalized plugin_reactions_display value
// ("absolute" or "relative"); defaults to "absolute" like upstream.
func readDisplay(in map[string]any) string {
	v, ok := in["plugin_reactions_display"].(string)
	if !ok {
		return "absolute"
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "relative":
		return "relative"
	default:
		return "absolute"
	}
}

// readDetails parses plugin_reactions_details (comma-separated array of
// "count" / "percentage", in priority order). Unknown values are
// dropped; at most two entries are kept (primary + secondary).
func readDetails(in map[string]any) []string {
	v, ok := in["plugin_reactions_details"]
	if !ok {
		return nil
	}
	var raw []string
	switch x := v.(type) {
	case string:
		raw = append(raw, strings.Split(x, ",")...)
	case []string:
		raw = x
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				raw = append(raw, s)
			}
		}
	case bool:
		// Legacy boolean form: enabling details defaults to percentage.
		if x {
			return []string{"percentage"}
		}
		return nil
	}
	out := make([]string, 0, 2)
	for _, p := range raw {
		s := strings.ToLower(strings.TrimSpace(p))
		if s == "count" || s == "percentage" {
			out = append(out, s)
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
