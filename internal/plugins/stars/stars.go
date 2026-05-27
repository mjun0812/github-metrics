// Package stars owns the M4 "stars" plugin. It fetches the user's
// most-recently starred repositories via GraphQL
// `user.starredRepositories(orderBy: STARRED_AT)`.
package stars

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "stars"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &starsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type starsPlugin struct{}

func (p *starsPlugin) Name() string                     { return Name }
func (p *starsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["stars"].
type Result struct {
	Skipped       bool          `json:"skipped,omitempty"`
	SkippedReason string        `json:"-"`
	List          []StarredRepo `json:"list"`
	Limit         int           `json:"limit"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// StarredRepo carries one entry from the upstream starred list.
type StarredRepo struct {
	NameWithOwner string    `json:"nameWithOwner"`
	Description   string    `json:"description"`
	Stars         int       `json:"stars"`
	StarredAt     time.Time `json:"starredAt"`
}

// Run issues a single GraphQL call to fetch the user's most recently
// starred repositories (Limit defaults to 4).
func (p *starsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{Skipped: true, SkippedReason: reason, List: []StarredRepo{}}, nil
	}
	// Per the contract a plugin runs only when plugin_<name> is truthy.
	// Engine glue does not gate today, so each plugin self-gates.
	if !truthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled", List: []StarredRepo{}}, nil
	}
	if pc.GraphQL == nil {
		return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable", List: []StarredRepo{}}, nil
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login", List: []StarredRepo{}}, nil
	}
	limit := readIntDefault(pc.Inputs, "plugin_stars_limit", 4)
	if limit <= 0 {
		limit = 4
	}

	resp, err := pc.GraphQL.UserStarredRepositories(ctx, login, limit)
	if err != nil {
		return nil, xerrors.NewRetryableError(fmt.Errorf("stars: %w", err))
	}
	if resp == nil || resp.User == nil || resp.User.StarredRepositories == nil {
		return &Result{List: []StarredRepo{}, Limit: limit}, nil
	}

	list := make([]StarredRepo, 0, len(resp.User.StarredRepositories.Edges))
	for _, edge := range resp.User.StarredRepositories.Edges {
		if edge == nil || edge.Node == nil {
			continue
		}
		desc := ""
		if edge.Node.Description != nil {
			desc = *edge.Node.Description
		}
		list = append(list, StarredRepo{
			NameWithOwner: edge.Node.NameWithOwner,
			Description:   desc,
			Stars:         edge.Node.StargazerCount,
			StarredAt:     edge.StarredAt,
		})
	}
	return &Result{List: list, Limit: limit}, nil
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
