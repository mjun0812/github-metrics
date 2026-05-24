// Package reactions owns the M4 "reactions" plugin. It aggregates
// reaction counts on the user's issues + issue comments via GraphQL.
// Upstream also covers discussion comments — that arm lands as a
// follow-up since GitHub's discussion API surface requires additional
// scopes and a separate query.
package reactions

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
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

// Result is the JSON payload published under data.Plugins["reactions"].
type Result struct {
	Skipped       bool           `json:"skipped,omitempty"`
	SkippedReason string         `json:"-"`
	Issues        int            `json:"issues"`
	Comments      int            `json:"comments"`
	Discussions   int            `json:"discussions"`
	Total         int            `json:"total"`
	Days          int            `json:"days"`
	Details       map[string]int `json:"details,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Run issues a single GraphQL call to sum reaction totalCount across
// the user's issues and issue comments.
func (p *reactionsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if !truthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled"}, nil
	}
	if pc.GraphQL == nil {
		return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable"}, nil
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login"}, nil
	}
	issuesLimit := readIntDefault(pc.Inputs, "plugin_reactions_limit_issues", 100)
	commentsLimit := readIntDefault(pc.Inputs, "plugin_reactions_limit_comments", 100)
	days := readIntDefault(pc.Inputs, "plugin_reactions_days", 30)

	resp, err := pc.GraphQL.UserReactions(ctx, login, issuesLimit, commentsLimit)
	if err != nil {
		return nil, xerrors.NewRetryableError(fmt.Errorf("reactions: %w", err))
	}
	if resp == nil || resp.User == nil {
		return &Result{Days: days}, nil
	}

	var issuesTotal, commentsTotal int
	if resp.User.Issues != nil {
		for _, n := range resp.User.Issues.Nodes {
			if n == nil || n.Reactions == nil {
				continue
			}
			issuesTotal += n.Reactions.TotalCount
		}
	}
	if resp.User.IssueComments != nil {
		for _, n := range resp.User.IssueComments.Nodes {
			if n == nil || n.Reactions == nil {
				continue
			}
			commentsTotal += n.Reactions.TotalCount
		}
	}

	r := &Result{
		Issues:   issuesTotal,
		Comments: commentsTotal,
		Days:     days,
	}
	r.Total = r.Issues + r.Comments + r.Discussions
	if readBool(pc.Inputs, "plugin_reactions_details") {
		// MVP: real per-emoji breakdown requires per-issue subfield
		// fetching. Surface an empty map so the JSON key appears.
		r.Details = map[string]int{}
	}
	return r, nil
}

func truthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
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

func loginFromInputs(in map[string]any) string {
	if v, ok := in["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := in["login"].(string); ok {
		return v
	}
	return ""
}

func readIntDefault(in map[string]any, key string, def int) int {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return def
		}
		return n
	}
	return def
}

func readBool(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	}
	return false
}
