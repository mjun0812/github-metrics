// Package sponsorships owns the M4 "sponsorships" plugin. It returns
// the sponsorships the viewer is paying for. In M4 the plugin is
// scaffolded; the underlying `viewer.sponsorshipsAsSponsor` GraphQL
// call lands as a follow-up once the schema covers it. The MVP wires
// the type surface so downstream consumers see the slot.
package sponsorships

import (
	"context"
	"slices"
	"strings"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "sponsorships"

// sectionAmount / sectionSponsorships are the two upstream sections
// (org_repo/source/plugins/sponsorships/metadata.yml,
// plugin_sponsorships_sections values). The default is both, in order.
const (
	sectionAmount       = "amount"
	sectionSponsorships = "sponsorships"
)

// amountImageURL is the heart image upstream embeds for the "amount"
// section (index.mjs: hearts_around.png). The render pipeline inlines
// it as base64 so the SVG is self-contained.
const amountImageURL = "https://github.githubassets.com/images/icons/emoji/hearts_around.png"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &sponsorshipsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type sponsorshipsPlugin struct{}

func (p *sponsorshipsPlugin) Name() string { return Name }

func (p *sponsorshipsPlugin) Requires() []plugins.DataKey {
	// sponsorships reads from pc.Data fields populated by base; it does not
	// call Provider directly.
	return []plugins.DataKey{}
}

// Result is the JSON payload published under data.Plugins["sponsorships"].
type Result struct {
	Skipped       bool        `json:"skipped,omitempty"`
	SkippedReason string      `json:"-"`
	Active        []Sponsored `json:"active"`
	Past          []Sponsored `json:"past,omitempty"`
	// Sections honors plugin_sponsorships_sections (default
	// "amount, sponsorships") and drives which partial branches render.
	Sections []string `json:"sections"`
	// Amount is the total spend in US dollars (cents / 100) the viewer
	// has sponsored. Renders in the "amount" section; zero is valid and
	// still renders as "$0.00".
	Amount float64 `json:"amount"`
	// Image is the URL of the heart image shown in the "amount" section.
	// The render pipeline inlines it as a base64 data URI.
	Image string `json:"image,omitempty"`
	// Started is the date of the viewer's first sponsorship, shown as
	// "since <date>" in the amount line. Nil when there are none.
	Started *time.Time `json:"started,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Sponsored carries one entry from the upstream sponsorships list.
type Sponsored struct {
	Login  string     `json:"login"`
	Tier   string     `json:"tier"`
	Since  time.Time  `json:"since"`
	Until  *time.Time `json:"until,omitempty"`
	Avatar string     `json:"avatar,omitempty"`
	Type   string     `json:"type,omitempty"`
}

// Run wires viewer.sponsorshipsAsSponsor (spec 013). With a nil GraphQL
// client the M4 baseline (empty, non-Skipped) is preserved.
func (p *sponsorshipsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	sections := readSections(pc.Inputs)
	base := &Result{Active: []Sponsored{}, Sections: sections}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		base.Skipped = true
		base.SkippedReason = reason
		return base, nil
	}
	if !pluginutil.Truthy(pc.Inputs["plugin_sponsorships"]) {
		return base, nil
	}
	// When the "amount" section is requested we surface the heart image
	// even before the fetch so the zero-state still renders it.
	if slices.Contains(sections, sectionAmount) {
		base.Image = amountImageURL
	}
	if pc.GraphQL == nil {
		return base, nil
	}
	resp, err := pc.GraphQL.ViewerSponsorships(ctx, 12)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	active, past := collectViewerSponsorships(resp)
	base.Active = active
	base.Past = past
	base.Amount = amountFromResponse(resp)
	base.Started = startedFromResponse(resp)
	return base, nil
}

// readSections parses plugin_sponsorships_sections (comma-separated),
// keeping only the recognized upstream values in input order. Falls back
// to the upstream default "amount, sponsorships" when unset/empty.
func readSections(in map[string]any) []string {
	raw := pluginutil.ReadCSVValue(in["plugin_sponsorships_sections"])
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		switch strings.ToLower(s) {
		case sectionAmount:
			out = append(out, sectionAmount)
		case sectionSponsorships:
			out = append(out, sectionSponsorships)
		}
	}
	if len(out) == 0 {
		return []string{sectionAmount, sectionSponsorships}
	}
	return out
}

// amountFromResponse converts totalSponsorshipAmountAsSponsorInCents to
// US dollars. A nil/absent value (the common case) yields 0.0, matching
// upstream's "$0.00" zero-state.
func amountFromResponse(resp *githubapi.ViewerSponsorshipsResponse) float64 {
	if resp == nil || resp.Viewer == nil || resp.Viewer.TotalSponsorshipAmountAsSponsorInCents == nil {
		return 0
	}
	return float64(*resp.Viewer.TotalSponsorshipAmountAsSponsorInCents) / 100
}

// startedFromResponse picks the earliest sponsorship createdAt (the
// query orders DESC, so the last node is the oldest) for the "since"
// suffix. Returns nil when there are no sponsorships.
func startedFromResponse(resp *githubapi.ViewerSponsorshipsResponse) *time.Time {
	if resp == nil || resp.Viewer == nil || resp.Viewer.SponsorshipsAsSponsor == nil {
		return nil
	}
	nodes := resp.Viewer.SponsorshipsAsSponsor.Nodes
	var earliest *time.Time
	for _, n := range nodes {
		if n == nil {
			continue
		}
		t := n.CreatedAt
		if earliest == nil || t.Before(*earliest) {
			tt := t
			earliest = &tt
		}
	}
	return earliest
}

// collectViewerSponsorships splits the sponsorable nodes into active and
// past lists (upstream `past: !active`), preserving query order.
func collectViewerSponsorships(resp *githubapi.ViewerSponsorshipsResponse) (active, past []Sponsored) {
	active, past = []Sponsored{}, []Sponsored{}
	if resp == nil || resp.Viewer == nil || resp.Viewer.SponsorshipsAsSponsor == nil {
		return active, past
	}
	for _, n := range resp.Viewer.SponsorshipsAsSponsor.Nodes {
		if n == nil || n.Sponsorable == nil {
			continue
		}
		var s Sponsored
		switch x := (*n.Sponsorable).(type) {
		case *githubapi.ViewerSponsorshipsViewerUserSponsorshipsAsSponsorSponsorshipConnectionNodesSponsorshipSponsorableUser:
			s = Sponsored{Login: x.Login, Avatar: x.AvatarUrl, Type: "user", Since: n.CreatedAt}
		case *githubapi.ViewerSponsorshipsViewerUserSponsorshipsAsSponsorSponsorshipConnectionNodesSponsorshipSponsorableOrganization:
			s = Sponsored{Login: x.Login, Avatar: x.AvatarUrl, Type: "organization", Since: n.CreatedAt}
		default:
			continue
		}
		if n.IsActive {
			active = append(active, s)
		} else {
			past = append(past, s)
		}
	}
	return active, past
}
