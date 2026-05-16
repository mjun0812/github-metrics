// Package activity owns the M4 "activity" plugin. It walks the user's
// public events feed (REST /users/<login>/events) and surfaces a typed
// timeline of recent activity for the classic SVG.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p1-mvp.md §2
// Data model: specs/004-m4-github-plugins/data-model.md E-013
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "activity"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &activityPlugin{}

func init() {
	plugins.Register(Plugin)
}

type activityPlugin struct{}

func (p *activityPlugin) Name() string                     { return Name }
func (p *activityPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload the plugin publishes under
// data.Plugins["activity"]. Field set mirrors upstream
// data.plugins.activity.
type Result struct {
	Skipped       bool            `json:"skipped,omitempty"`
	SkippedReason string          `json:"-"`
	Events        []ActivityEvent `json:"events"`
	Days          int             `json:"days"`
}

// IsSkipped lets the classic dispatcher detect the skipped path
// uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// ActivityEvent mirrors one entry from the upstream events array.
type ActivityEvent struct {
	Type       string    `json:"type"`
	Repo       string    `json:"repo"`
	Date       time.Time `json:"date"`
	Visibility string    `json:"visibility"`
}

// rawEvent matches the GitHub REST `/users/{login}/events` payload shape
// for the fields this plugin consumes.
type rawEvent struct {
	Type      string    `json:"type"`
	Repo      rawRepo   `json:"repo"`
	CreatedAt time.Time `json:"created_at"`
	Public    bool      `json:"public"`
}

type rawRepo struct {
	Name string `json:"name"`
}

type inputs struct {
	limit      int
	load       int
	days       int
	visibility string
	skipped    map[string]struct{} // event types to drop
	ignored    map[string]struct{} // repo names to drop
}

func (p *activityPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.REST == nil {
		return &Result{Skipped: true, SkippedReason: "REST client unavailable"}, nil
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login"}, nil
	}
	in := parseInputs(pc.Inputs)
	cutoff := time.Now().UTC().AddDate(0, 0, -in.days)

	raws, err := fetchEvents(ctx, pc, login, in.load)
	if err != nil {
		// Transient failures surface as *RetryableError so the engine
		// records them on Result.Errors per contract §2.5.
		return nil, xerrors.NewRetryableError(fmt.Errorf("activity: %w", err))
	}

	events := make([]ActivityEvent, 0, len(raws))
	for _, raw := range raws {
		if _, drop := in.skipped[raw.Type]; drop {
			continue
		}
		if _, drop := in.ignored[raw.Repo.Name]; drop {
			continue
		}
		vis := visibilityOf(raw.Public)
		if !matchesVisibility(vis, in.visibility) {
			continue
		}
		if !raw.CreatedAt.IsZero() && raw.CreatedAt.Before(cutoff) {
			continue
		}
		events = append(events, ActivityEvent{
			Type:       raw.Type,
			Repo:       raw.Repo.Name,
			Date:       raw.CreatedAt,
			Visibility: vis,
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Date.After(events[j].Date)
	})
	if len(events) > in.limit {
		events = events[:in.limit]
	}

	return &Result{
		Events: events,
		Days:   in.days,
	}, nil
}

// fetchEvents pages /users/{login}/events?per_page=100 until either
// `load` events have been collected or the events API exhausts. Returns
// the raw events in the order GitHub returned them (newest-first).
func fetchEvents(ctx context.Context, pc *plugins.PluginContext, login string, load int) ([]rawEvent, error) {
	const perPage = 100
	out := make([]rawEvent, 0, load)
	for page := 1; len(out) < load; page++ {
		path := fmt.Sprintf("/users/%s/events?per_page=%d&page=%d",
			url.PathEscape(login), perPage, page)
		body, resp, err := pc.REST.Get(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("activity: nil response")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("activity: events status %d", resp.StatusCode)
		}
		var batch []rawEvent
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("activity: decode events: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		out = append(out, batch...)
		if len(batch) < perPage {
			break
		}
	}
	if len(out) > load {
		out = out[:load]
	}
	return out, nil
}

func loginFromInputs(in map[string]any) string {
	if in == nil {
		return ""
	}
	if v, ok := in["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := in["login"].(string); ok {
		return v
	}
	return ""
}

func parseInputs(in map[string]any) inputs {
	out := inputs{
		limit:      100,
		load:       300,
		days:       14,
		visibility: "public",
		skipped:    map[string]struct{}{},
		ignored:    map[string]struct{}{},
	}
	if v, ok := readInt(in, "plugin_activity_limit"); ok {
		out.limit = v
	}
	if v, ok := readInt(in, "plugin_activity_load"); ok {
		out.load = v
	}
	if v, ok := readInt(in, "plugin_activity_days"); ok {
		out.days = v
	}
	if v, ok := in["plugin_activity_visibility"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.visibility = s
		}
	}
	for _, s := range readCSV(in, "plugin_activity_skipped") {
		out.skipped[s] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_activity_ignored") {
		out.ignored[s] = struct{}{}
	}
	return out
}

func visibilityOf(public bool) string {
	if public {
		return "public"
	}
	return "private"
}

func matchesVisibility(eventVis, want string) bool {
	switch want {
	case "all", "":
		return true
	case "public", "private":
		return eventVis == want
	}
	return true
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

// Compile-time guard: ensure http.Header import stays used (some
// helpers below may go through pc.REST.Get with a nil header).
var _ http.Header
