// Package sponsors owns the M4 "sponsors" plugin. The plugin is
// gated by the `read:user` AND `read:org` OAuth scopes; the MVP wires
// the scope-gate path and returns an empty (but non-Skipped) Result
// when both scopes are present. The full GraphQL fetch lands as a
// follow-up.
package sponsors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
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
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			Sponsors:      []Sponsor{},
		}, nil
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
	// Parse 011 v2 inputs. Defaults mirror upstream
	// assets/plugins/sponsors/metadata.yml: sections=goal,list,about,
	// size=24, title="Sponsor Me!".
	sections := []string{"goal", "list", "about"}
	if v, ok := pc.Inputs["plugin_sponsors_sections"]; ok {
		if s, ok := v.(string); ok && s != "" {
			sections = splitCSV(s)
		}
	}
	past := false
	if v, ok := pc.Inputs["plugin_sponsors_past"]; ok {
		past = truthy(v)
	}
	size := 24
	if v, ok := pc.Inputs["plugin_sponsors_size"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			size = n
		}
	}
	title := "Sponsor Me!"
	user := ""
	if pc.Data != nil && pc.Data.User != nil {
		user = pc.Data.User.Login
	}
	if v, ok := pc.Inputs["plugin_sponsors_title"]; ok {
		if s, ok := v.(string); ok && s != "" {
			title = s
		}
	}

	base := &Result{
		Mode:         plugins.AggregationMode(pc.Data),
		Sections:     sections,
		Sponsors:     []Sponsor{},
		Past:         []Sponsor{},
		Title:        title,
		User:         user,
		PastIncluded: past,
		Size:         size,
		Count:        Count{Active: CountBucket{Total: 0}, Past: CountBucket{Total: 0}},
	}

	// GraphQL data fetch (spec 013). On nil client OR when the plugin
	// is not enabled via `plugin_sponsors=yes` we return the M4 baseline
	// (empty, non-Skipped) so dependent test suites that don't enable
	// the plugin stay green and don't accumulate Data.Errors entries.
	if pc.GraphQL == nil || !pluginEnabled(pc.Inputs, "plugin_sponsors") {
		return base, nil
	}
	// GitHub's GraphQL API rejects a connection `first: 0` (it must be a
	// positive integer), so we never pass 0. Upstream only fetches the
	// past connection when `plugin_sponsors_past` is enabled; we mirror
	// that by always requesting a valid `first` (>= 1) and discarding the
	// past result when `past` is disabled (see populateFromGraphQL).
	activeFirst := 12
	pastFirst := 1
	if past {
		pastFirst = 12
	}
	resp, err := pc.GraphQL.ViewerSponsors(ctx, activeFirst, pastFirst)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = fmt.Sprintf("GraphQL fetch failed: %v", err)
		if pc.Data != nil {
			pc.Data.AppendError(xerrors.NewRetryableError(err))
		}
		return base, nil
	}
	populateFromGraphQL(base, resp, past)
	return base, nil
}

// populateFromGraphQL maps the ViewerSponsors GraphQL response onto the
// pre-built base Result. It mutates `out` in place. When `past` is false
// the past connection is fetched only to satisfy GitHub's `first >= 1`
// requirement and its results are discarded.
func populateFromGraphQL(out *Result, resp *githubapi.ViewerSponsorsResponse, past bool) {
	if resp == nil || resp.Viewer == nil {
		return
	}
	v := resp.Viewer
	if v.SponsorsListing != nil {
		out.About = v.SponsorsListing.FullDescription
		if g := v.SponsorsListing.ActiveGoal; g != nil {
			goalTitle := ""
			if g.Title != nil {
				goalTitle = *g.Title
			}
			goalDesc := ""
			if g.Description != nil {
				goalDesc = *g.Description
			}
			out.Goal = &Goal{
				Title:       goalTitle,
				Description: goalDesc,
				Progress:    g.PercentComplete,
			}
		}
	}
	if v.Active != nil {
		out.Count.Active.Total = v.Active.TotalCount
		out.Sponsors = collectActive(v.Active.Nodes)
	}
	if past && v.Past != nil {
		pastOnly := collectPast(v.Past.Nodes)
		out.Past = pastOnly
		out.Count.Past.Total = len(pastOnly)
	}
}

func collectActive(nodes []*githubapi.ViewerSponsorsViewerUserActiveSponsorshipConnectionNodesSponsorship) []Sponsor {
	out := make([]Sponsor, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || n.SponsorEntity == nil {
			continue
		}
		switch x := (*n.SponsorEntity).(type) {
		case *githubapi.ViewerSponsorsViewerUserActiveSponsorshipConnectionNodesSponsorshipSponsorEntityUser:
			out = append(out, Sponsor{Login: x.Login, Avatar: x.AvatarUrl, Type: "user", Since: n.CreatedAt})
		case *githubapi.ViewerSponsorsViewerUserActiveSponsorshipConnectionNodesSponsorshipSponsorEntityOrganization:
			out = append(out, Sponsor{Login: x.Login, Avatar: x.AvatarUrl, Type: "organization", Since: n.CreatedAt})
		}
	}
	return out
}

func collectPast(nodes []*githubapi.ViewerSponsorsViewerUserPastSponsorshipConnectionNodesSponsorship) []Sponsor {
	out := make([]Sponsor, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || n.SponsorEntity == nil || n.IsActive {
			continue
		}
		switch x := (*n.SponsorEntity).(type) {
		case *githubapi.ViewerSponsorsViewerUserPastSponsorshipConnectionNodesSponsorshipSponsorEntityUser:
			out = append(out, Sponsor{Login: x.Login, Avatar: x.AvatarUrl, Type: "user", Since: n.CreatedAt})
		case *githubapi.ViewerSponsorsViewerUserPastSponsorshipConnectionNodesSponsorshipSponsorEntityOrganization:
			out = append(out, Sponsor{Login: x.Login, Avatar: x.AvatarUrl, Type: "organization", Since: n.CreatedAt})
		}
	}
	return out
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

// pluginEnabled returns true when the named input is truthy. Used by
// spec-013 wiring to short-circuit GraphQL fetches when the consuming
// workflow has not opted into the plugin (test paths + dryrun CLI).
func pluginEnabled(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	return truthy(v)
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
