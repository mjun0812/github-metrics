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
	"sort"
	"strconv"
	"strings"
	"time"

	enry "github.com/go-enry/go-enry/v2"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
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

func (p *habitsPlugin) Requires() []plugins.DataKey {
	// habits reads from pc.Data fields populated by base; it does not
	// call Provider directly.
	return []plugins.DataKey{}
}

// Result is the JSON payload published under data.Plugins["habits"].
type Result struct {
	Skipped       bool          `json:"skipped,omitempty"`
	SkippedReason string        `json:"-"`
	Days          int           `json:"days"`
	FactsEnabled  bool          `json:"factsEnabled"`
	ChartsEnabled bool          `json:"chartsEnabled"`
	Facts         HabitFacts    `json:"facts"`
	Charts        HabitCharts   `json:"charts"`
	Linguist      HabitLinguist `json:"linguist"`
	From          int           `json:"from"`
	Trim          bool          `json:"trim"`
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

// HabitLinguist holds the "Language activity" breakdown derived from the
// files touched by recent PushEvent commits. Mirrors upstream's
// `plugins.habits.linguist` ({available, ordered}). Available is false
// when no analyzable files were found, which keeps the section out of the
// SVG exactly like upstream's `linguist.available` gate.
type HabitLinguist struct {
	Available bool            `json:"available"`
	Ordered   []LanguageShare `json:"ordered"`
}

// LanguageShare is one (language, share) pair in the Language activity
// chart. Share is the 0..1 fraction of analyzed bytes attributed to the
// language, matching upstream's `linguist.ordered` entries.
type LanguageShare struct {
	Name  string  `json:"name"`
	Share float64 `json:"share"`
}

type rawEvent struct {
	Type      string       `json:"type"`
	CreatedAt time.Time    `json:"created_at"`
	Repo      rawEventRepo `json:"repo"`
	Payload   rawEventLoad `json:"payload"`
}

type rawEventRepo struct {
	Name string `json:"name"`
}

type rawEventLoad struct {
	Commits []rawEventCommit `json:"commits"`
	Before  string           `json:"before"`
	Head    string           `json:"head"`
}

func (p rawEventLoad) commitSHAs() []string {
	out := make([]string, 0, len(p.Commits))
	for _, c := range p.Commits {
		if c.SHA != "" {
			out = append(out, c.SHA)
		}
	}
	return out
}

type rawEventCommit struct {
	SHA string `json:"sha"`
}

type rawCommitFile struct {
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Run pages /users/{login}/events looking for PushEvent activity, then
// bins each event into the 24-hour and 7-day histograms. Returns
// Skipped when no events surface so the partial stays out of the SVG.
func (p *habitsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	factsEnabled := pluginutil.ReadBoolDefault(pc.Inputs, "plugin_habits_facts", true)
	chartsEnabled := pluginutil.ReadBoolDefault(pc.Inputs, "plugin_habits_charts", true)
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
	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return skipped("no login"), nil
	}
	from := pluginutil.ReadIntDefault(pc.Inputs, "plugin_habits_from", 200)
	days := pluginutil.ReadIntDefault(pc.Inputs, "plugin_habits_days", 14)
	trim := pluginutil.ReadBool(pc.Inputs, "plugin_habits_trim")
	langLimit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_habits_languages_limit", 8)
	langThreshold := readPercent(pc.Inputs, "plugin_habits_languages_threshold")

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
	windowEvents := make([]rawEvent, 0, len(events))
	for _, e := range events {
		if e.CreatedAt.IsZero() {
			continue
		}
		hour := e.CreatedAt.UTC().Hour()
		// time.Weekday: Sunday=0 ... Saturday=6, matching upstream's
		// getDay() indexing and partial.go's Sun-first dayNames.
		wd := int(e.CreatedAt.UTC().Weekday())
		charts.Hours[hour]++
		charts.Days[wd]++
		if e.CreatedAt.After(cutoff) {
			commitsInWindow++
			windowEvents = append(windowEvents, e)
		}
	}

	facts := HabitFacts{
		IndentStyle: "spaces", // upstream-compat default; real detection lands with diff fetching
	}
	if days > 0 {
		facts.CommitsPerDay = float64(commitsInWindow) / float64(days)
	}

	// Language activity ("linguist"): mirrors upstream habits/index.mjs,
	// which reuses the languages `recent` analyzer to attribute the files
	// touched by in-window PushEvent commits to a language, then renders
	// the share breakdown. Only computed when charts are enabled, matching
	// upstream's `if (charts && extras(...))` gate.
	var linguist HabitLinguist
	if chartsEnabled {
		linguist = computeLinguist(ctx, pc, windowEvents, langLimit, langThreshold)
	}

	return &Result{
		Days:          days,
		FactsEnabled:  factsEnabled,
		ChartsEnabled: chartsEnabled,
		Facts:         facts,
		Charts:        charts,
		Linguist:      linguist,
		From:          from,
		Trim:          trim,
	}, nil
}

// computeLinguist walks the files changed by the supplied in-window
// PushEvents, attributes their additions+deletions byte volume to a
// language via go-enry, and returns the ordered share breakdown.
//
// It mirrors upstream's recent-language analyzer: byte counts per
// language, normalized to a 0..1 share of the analyzed total, sorted
// descending, filtered by threshold, then truncated to limit. Available
// is reported true only when at least one analyzable file surfaced, so
// the partial drops the section (like upstream's `linguist.available`)
// when nothing usable was fetched.
//
// Per-commit fetch failures are best-effort: a single miss is recorded on
// Data.Errors and skipped rather than aborting the whole walk.
func computeLinguist(ctx context.Context, pc *plugins.PluginContext, events []rawEvent, limit int, threshold float64) HabitLinguist {
	totals := map[string]int{}
	accumulate := func(files []rawCommitFile) {
		for _, f := range files {
			lang := enry.GetLanguage(f.Filename, nil)
			if lang == "" {
				continue
			}
			bytes := f.Additions + f.Deletions
			if bytes <= 0 {
				continue
			}
			totals[lang] += bytes
		}
	}

	for _, e := range events {
		if e.Repo.Name == "" {
			continue
		}
		if shas := e.Payload.commitSHAs(); len(shas) > 0 {
			for _, sha := range shas {
				files, err := fetchCommitFiles(ctx, pc, e.Repo.Name, sha)
				if err != nil {
					pc.Data.AppendError(fmt.Errorf("habits: commit %s/%s: %w", e.Repo.Name, sha, err))
					continue
				}
				accumulate(files)
			}
			continue
		}
		files, err := fetchPushFiles(ctx, pc, e.Repo.Name, e.Payload.Before, e.Payload.Head)
		if err != nil {
			pc.Data.AppendError(fmt.Errorf("habits: push %s: %w", e.Repo.Name, err))
			continue
		}
		accumulate(files)
	}

	totalBytes := 0
	for _, n := range totals {
		totalBytes += n
	}
	if totalBytes == 0 {
		return HabitLinguist{Available: false, Ordered: []LanguageShare{}}
	}

	ordered := make([]LanguageShare, 0, len(totals))
	for name, n := range totals {
		ordered = append(ordered, LanguageShare{
			Name:  name,
			Share: float64(n) / float64(totalBytes),
		})
	}
	// Sort by share desc, then name asc for deterministic ties.
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Share != ordered[j].Share {
			return ordered[i].Share > ordered[j].Share
		}
		return ordered[i].Name < ordered[j].Name
	})
	// Apply threshold filter (upstream: `value > threshold`).
	if threshold > 0 {
		filtered := ordered[:0:0]
		for _, ls := range ordered {
			if ls.Share > threshold {
				filtered = append(filtered, ls)
			}
		}
		ordered = filtered
	}
	// Apply display limit (upstream: `slice(0, limit || Infinity)`).
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}

	return HabitLinguist{Available: true, Ordered: ordered}
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

// rawCommit is the subset of /repos/{repo}/commits/{sha} we consume.
type rawCommit struct {
	Files []rawCommitFile `json:"files"`
}

// rawCompare is the subset of the /compare response we consume.
type rawCompare struct {
	Files []rawCommitFile `json:"files"`
}

// fetchCommitFiles loads the per-file change list for a single commit.
func fetchCommitFiles(ctx context.Context, pc *plugins.PluginContext, repo, sha string) ([]rawCommitFile, error) {
	path := fmt.Sprintf("/repos/%s/commits/%s", repo, sha)
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
	var c rawCommit
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("habits: decode commit: %w", err)
	}
	return c.Files, nil
}

// fetchPushFiles resolves the files changed by a push via the compare API
// (/repos/{repo}/compare/{before}...{head}), which aggregates the
// additions/deletions across the pushed range without depending on
// payload.commits (which GitHub frequently omits from the events feed).
// New-branch pushes carry an all-zero `before`, so fall back to the head
// commit alone. Mirrors languages.recent's fetchPushFiles.
func fetchPushFiles(ctx context.Context, pc *plugins.PluginContext, repo, before, head string) ([]rawCommitFile, error) {
	if head == "" {
		return nil, nil
	}
	if pluginutil.IsZeroSHA(before) {
		return fetchCommitFiles(ctx, pc, repo, head)
	}
	path := fmt.Sprintf("/repos/%s/compare/%s...%s", repo, before, head)
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
	var c rawCompare
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("habits: decode compare: %w", err)
	}
	return c.Files, nil
}

// readPercent parses a "N%" string input into a 0..1 fraction, mirroring
// upstream's `Number(threshold.replace(/%$/, "")) / 100`. Missing /
// unparsable values yield 0 (no threshold).
func readPercent(in map[string]any, key string) float64 {
	v, ok := in[key]
	if !ok {
		return 0
	}
	s, ok := v.(string)
	if !ok {
		// Numeric inputs are treated as already-percent values.
		switch x := v.(type) {
		case float64:
			return x / 100
		case int:
			return float64(x) / 100
		}
		return 0
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if s == "" {
		return 0
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return n / 100
}
