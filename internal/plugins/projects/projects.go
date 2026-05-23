// Package projects owns the M4 "projects" plugin. The plugin is
// gated by the `read:project` OAuth scope; when the active token does
// not advertise the scope, Run returns Skipped=true. In M4 the actual
// GraphQL fetch is deferred — the scope-gate path is wired and tests
// exercise it, but a real `user.projects` GraphQL query lands as a
// follow-up.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §8
// Data model: specs/004-m4-github-plugins/data-model.md E-029
package projects

import (
	"context"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "projects"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &projectsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type projectsPlugin struct{}

func (p *projectsPlugin) Name() string                     { return Name }
func (p *projectsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["projects"].
type Result struct {
	Skipped       bool      `json:"skipped,omitempty"`
	SkippedReason string    `json:"-"`
	Mode          string    `json:"mode,omitempty"`
	List          []Project `json:"list"`
	Limit         int       `json:"limit"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Project mirrors one upstream project entry.
type Project struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Closed      bool      `json:"closed,omitempty"`
}

// Run checks for the `read:project` scope via REST.Scopes() and
// short-circuits to Skipped if missing. With the scope present the
// MVP returns an empty List (the real GraphQL fetch lands in a
// follow-up).
func (p *projectsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.REST == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			List:          []Project{},
		}, nil
	}
	scopes, err := pc.REST.Scopes(ctx)
	if err != nil {
		// Scope probe failure → Skipped (no plugin work possible
		// without knowing the scope set).
		//nolint:nilerr // intentional: Scopes failure maps to Skipped
		return &Result{
			Skipped:       true,
			SkippedReason: "could not determine token scopes",
			List:          []Project{},
		}, nil
	}
	if !hasScope(scopes, "read:project") {
		return &Result{
			Skipped:       true,
			SkippedReason: "missing read:project scope",
			List:          []Project{},
		}, nil
	}
	// Spec 013: GraphQL fetch wiring.
	limit := 4
	if v, ok := pc.Inputs["plugin_projects_limit"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			limit = n
		}
	}
	base := &Result{Mode: plugins.AggregationMode(pc.Data), List: []Project{}, Limit: limit}
	if pc.GraphQL == nil || !truthy(pc.Inputs["plugin_projects"]) {
		return base, nil
	}
	resp, err := pc.GraphQL.ViewerProjects(ctx, limit)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	base.List = collectProjects(resp)
	return base, nil
}

func collectProjects(resp *githubapi.ViewerProjectsResponse) []Project {
	if resp == nil || resp.Viewer == nil || resp.Viewer.ProjectsV2 == nil {
		return []Project{}
	}
	nodes := resp.Viewer.ProjectsV2.Nodes
	out := make([]Project, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		desc := ""
		if n.ShortDescription != nil {
			desc = *n.ShortDescription
		}
		out = append(out, Project{
			Name:        n.Title,
			Description: desc,
			URL:         n.Url,
			UpdatedAt:   n.UpdatedAt,
			Closed:      n.Closed,
		})
	}
	return out
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// truthy mirrors the shared helper across plugins; spec 013 uses it to
// gate the GraphQL fetch on the `plugin_projects` input.
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1" || x == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}
