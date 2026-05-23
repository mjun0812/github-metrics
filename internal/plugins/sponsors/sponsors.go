// Package sponsors owns the M4 "sponsors" plugin. The plugin is
// gated by the `read:user` AND `read:org` OAuth scopes; the MVP wires
// the scope-gate path and returns an empty (but non-Skipped) Result
// when both scopes are present. The full GraphQL fetch lands as a
// follow-up.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §9
// Data model: specs/004-m4-github-plugins/data-model.md E-030
package sponsors

import (
	"context"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "sponsors"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &sponsorsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type sponsorsPlugin struct{}

func (p *sponsorsPlugin) Name() string                     { return Name }
func (p *sponsorsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["sponsors"].
type Result struct {
	Skipped       bool      `json:"skipped,omitempty"`
	SkippedReason string    `json:"-"`
	Mode          string    `json:"mode,omitempty"`
	Sections      []string  `json:"sections"`
	Sponsors      []Sponsor `json:"sponsors"`
	Past          []Sponsor `json:"past,omitempty"`
	// 011 v2 additions for upstream parity (Principle II additive).
	// Title is rendered in the section header (`<h2>`); defaults to
	// the user's login when empty.
	Title string `json:"title,omitempty"`
	// User is the login the partial uses in "${user} is being funded by..."
	// strings; sourced from PluginContext.Data.User.Login.
	User string `json:"user,omitempty"`
	// Goal mirrors `plugins.sponsors.goal` when the user has a
	// sponsorship goal set on their profile. Empty Goal => no goal
	// progress bar rendered.
	Goal *Goal `json:"goal,omitempty"`
	// About is the markdown blurb shown in the `about` sub-section.
	About string `json:"about,omitempty"`
	// PastIncluded toggles the past-sponsors block (mjun0812 sets
	// `plugin_sponsors_past: yes`).
	PastIncluded bool `json:"pastIncluded,omitempty"`
	// Size is the per-sponsor avatar width/height in px (upstream
	// default 64).
	Size int `json:"size,omitempty"`
	// Count carries active + past totals (upstream uses these in the
	// "N sponsors are funding..." line).
	Count Count `json:"count,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Sponsor mirrors one entry from the upstream sponsor list. The
// Avatar + Type fields land in v2 for the avatar grid render.
type Sponsor struct {
	Login  string    `json:"login"`
	Tier   string    `json:"tier"`
	Since  time.Time `json:"since"`
	Avatar string    `json:"avatar,omitempty"`
	// Type is "user" or "organization". The organization variant gets
	// a different CSS class on the avatar (rounded square vs circle).
	Type string `json:"type,omitempty"`
}

// Goal mirrors `plugins.sponsors.goal` — funding-goal progress bar
// shown when the user configures a goal on their GitHub Sponsors
// profile.
type Goal struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// Progress is the percent (0-100) of the funding goal currently met.
	Progress int `json:"progress,omitempty"`
}

// Count carries active + past sponsor totals.
type Count struct {
	Active CountBucket `json:"active"`
	Past   CountBucket `json:"past"`
}

// CountBucket is one bucket in the Count breakdown.
type CountBucket struct {
	Total int `json:"total"`
}

// Run requires `read:user` OR `read:org` scope (per upstream
// behavior, either is sufficient because GitHub uses the broader of
// the two). With neither, Skipped=true. With at least one the MVP
// returns an empty (non-Skipped) Result.
func (p *sponsorsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.REST == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			Sponsors:      []Sponsor{},
		}, nil
	}
	scopes, err := pc.REST.Scopes(ctx)
	if err != nil {
		//nolint:nilerr // intentional: Scopes failure maps to Skipped
		return &Result{
			Skipped:       true,
			SkippedReason: "could not determine token scopes",
			Sponsors:      []Sponsor{},
		}, nil
	}
	if !hasScope(scopes, "read:user") && !hasScope(scopes, "read:org") {
		return &Result{
			Skipped:       true,
			SkippedReason: "missing read:user / read:org scope",
			Sponsors:      []Sponsor{},
		}, nil
	}
	// Parse 011 v2 inputs.
	sections := []string{"list"}
	if v, ok := pc.Inputs["plugin_sponsors_sections"]; ok {
		if s, ok := v.(string); ok && s != "" {
			sections = splitCSV(s)
		}
	}
	past := false
	if v, ok := pc.Inputs["plugin_sponsors_past"]; ok {
		past = truthy(v)
	}
	size := 64
	if v, ok := pc.Inputs["plugin_sponsors_size"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			size = n
		}
	}
	title := "Sponsors"
	user := ""
	if pc.Data != nil && pc.Data.User != nil {
		user = pc.Data.User.Login
		if user != "" {
			title = user + "'s sponsors"
		}
	}

	return &Result{
		Mode:         plugins.AggregationMode(pc.Data),
		Sections:     sections,
		Sponsors:     []Sponsor{},
		Title:        title,
		User:         user,
		PastIncluded: past,
		Size:         size,
		Count:        Count{Active: CountBucket{Total: 0}, Past: CountBucket{Total: 0}},
	}, nil
}

// splitCSV splits a comma-separated input value into a trimmed slice.
func splitCSV(s string) []string {
	parts := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// truthy mirrors the helper used in other plugins.
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

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
