// Package habits owns the M4 "habits" plugin. It derives "when do
// you commit" stats from the user's recent events feed. The full
// upstream behavior also analyzes commit diffs (lines / indent /
// chars per line) — the M4 MVP wires the day/hour histograms from
// PushEvents only, and exposes the input surface for the diff-based
// facts as a follow-up (commit-diff fetching lands alongside US3).
package habits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "habits"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &habitsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type habitsPlugin struct{}

func (p *habitsPlugin) Name() string                     { return Name }
func (p *habitsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["habits"].
type Result struct {
	Skipped       bool        `json:"skipped,omitempty"`
	SkippedReason string      `json:"-"`
	Days          int         `json:"days"`
	FactsEnabled  bool        `json:"factsEnabled"`
	ChartsEnabled bool        `json:"chartsEnabled"`
	Facts         HabitFacts  `json:"facts"`
	Charts        HabitCharts `json:"charts"`
	From          int         `json:"from"`
	Trim          bool        `json:"trim"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// HabitFacts holds the diff-derived statistics. In M4 MVP these are
// zero-valued because commit-diff fetching is deferred.
type HabitFacts struct {
	LinesPerCommit float64 `json:"linesPerCommit"`
	CharsPerLine   float64 `json:"charsPerLine"`
	CommitsPerDay  float64 `json:"commitsPerDay"`
	IndentStyle    string  `json:"indentStyle"`
}

// HabitCharts holds the per-hour and per-weekday push event histogram.
type HabitCharts struct {
	Hours [24]int `json:"hours"`
	Days  [7]int  `json:"days"`
}

type rawEvent struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// Run pages /users/{login}/events looking for PushEvent activity, then
// bins each event into the 24-hour and 7-day histograms. Returns
// Skipped when no events surface so the partial stays out of the SVG.
func (p *habitsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	factsEnabled := readBoolDefault(pc.Inputs, "plugin_habits_facts", true)
	chartsEnabled := readBoolDefault(pc.Inputs, "plugin_habits_charts", true)
	skipped := func(reason string) *Result {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			FactsEnabled:  factsEnabled,
			ChartsEnabled: chartsEnabled,
		}
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return skipped(reason), nil
	}
	if pc.REST == nil {
		return skipped("REST client unavailable"), nil
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return skipped("no login"), nil
	}
	from := readIntDefault(pc.Inputs, "plugin_habits_from", 200)
	days := readIntDefault(pc.Inputs, "plugin_habits_days", 14)
	trim := readBool(pc.Inputs, "plugin_habits_trim")

	events, err := fetchPushEvents(ctx, pc, login, from)
	if err != nil {
		// Treat REST failure as Skipped rather than retryable per
		// contract §2 "Skipped=true, SkippedReason='no recent commits'"
		// for any absence of usable input. Engine still surfaces
		// transport errors via Data.Errors plumbing if needed.
		//nolint:nilerr // intentional: REST failure maps to Skipped
		return skipped("events fetch failed"), nil
	}
	if len(events) == 0 {
		return skipped("no recent commits"), nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	var charts HabitCharts
	commitsInWindow := 0
	for _, e := range events {
		if e.CreatedAt.IsZero() {
			continue
		}
		hour := e.CreatedAt.UTC().Hour()
		// time.Weekday: Sunday=0 ... Saturday=6.
		// Upstream maps Mon-first; remap so Monday=0 ... Sunday=6.
		wd := (int(e.CreatedAt.UTC().Weekday()) + 6) % 7
		charts.Hours[hour]++
		charts.Days[wd]++
		if e.CreatedAt.After(cutoff) {
			commitsInWindow++
		}
	}

	facts := HabitFacts{
		IndentStyle: "spaces", // upstream-compat default; real detection lands with diff fetching
	}
	if days > 0 {
		facts.CommitsPerDay = float64(commitsInWindow) / float64(days)
	}

	return &Result{
		Days:          days,
		FactsEnabled:  factsEnabled,
		ChartsEnabled: chartsEnabled,
		Facts:         facts,
		Charts:        charts,
		From:          from,
		Trim:          trim,
	}, nil
}

func fetchPushEvents(ctx context.Context, pc *plugins.PluginContext, login string, limit int) ([]rawEvent, error) {
	const perPage = 100
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	maxPages := (limit + perPage - 1) / perPage
	out := make([]rawEvent, 0, limit)
	for page := 1; page <= maxPages && len(out) < limit; page++ {
		path := fmt.Sprintf("/users/%s/events?per_page=%d&page=%d",
			url.PathEscape(login), perPage, page)
		body, resp, err := pc.REST.Get(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("habits: nil response")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("habits: status %d", resp.StatusCode)
		}
		var batch []rawEvent
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("habits: decode: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, e := range batch {
			if e.Type == "PushEvent" {
				out = append(out, e)
				if len(out) >= limit {
					break
				}
			}
		}
		if len(batch) < perPage {
			break
		}
	}
	return out, nil
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

func readBoolDefault(in map[string]any, key string, def bool) bool {
	v, ok := in[key]
	if !ok {
		return def
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
