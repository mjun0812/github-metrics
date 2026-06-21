// Package activity owns the M4 "activity" plugin. It walks the user's
// public events feed (REST /users/<login>/events) and surfaces a typed
// timeline of recent activity for the classic SVG.
package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
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

// Requires reports the Provider data sources Run reads. activity walks
// /users/<login>/events directly via pc.REST and resolves the login
// from pc.Inputs, so it touches no Provider methods.
func (p *activityPlugin) Requires() []plugins.DataKey { return nil }

// Result is the JSON payload the plugin publishes under
// data.Plugins["activity"]. Field set mirrors upstream
// data.plugins.activity.
type Result struct {
	Skipped       bool            `json:"skipped,omitempty"`
	SkippedReason string          `json:"-"`
	Mode          string          `json:"mode,omitempty"`
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
	// Files and Lines carry pull-request diff stats for
	// PullRequestEvent entries, mirroring upstream
	// data.plugins.activity.events[].{files,lines}. They stay nil for
	// event types that do not expose diff stats so the template can
	// decide whether to render the "N files changed ++A --D" line.
	Files *EventFiles `json:"files,omitempty"`
	Lines *EventLines `json:"lines,omitempty"`
}

// EventFiles mirrors upstream event.files (PR change counts).
type EventFiles struct {
	Changed int `json:"changed"`
}

// EventLines mirrors upstream event.lines (PR additions/deletions).
type EventLines struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

// rawEvent matches the GitHub REST `/users/{login}/events` payload shape
// for the fields this plugin consumes.
type rawEvent struct {
	Type      string     `json:"type"`
	Repo      rawRepo    `json:"repo"`
	CreatedAt time.Time  `json:"created_at"`
	Public    bool       `json:"public"`
	Payload   rawPayload `json:"payload"`
}

type rawRepo struct {
	Name string `json:"name"`
}

// rawPayload covers the per-event-type `payload` object. Only the
// PullRequestEvent's pull_request reference is consumed today.
type rawPayload struct {
	PullRequest *rawPullRequest `json:"pull_request"`
}

// rawPullRequest carries the PR diff stats and PR number GitHub embeds
// in a PullRequestEvent payload.
//
// ⚠ The events API (`/users/{login}/events`) embeds only a *summary* of
// the pull request — `additions`, `deletions`, and `changed_files` are
// not populated there (they live on the full PR object). We keep the
// fields here for the rare case GitHub returns them, but rely on
// `Number` to fetch the full PR via `/repos/{owner}/{repo}/pulls/{num}`
// when the embedded summary lacks the diff stats.
type rawPullRequest struct {
	Number       int `json:"number"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedFiles int `json:"changed_files"`
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
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{Skipped: true, SkippedReason: reason, Events: []ActivityEvent{}}, nil
	}
	if pc.REST == nil {
		return &Result{Skipped: true, SkippedReason: "REST client unavailable"}, nil
	}
	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login"}, nil
	}
	in := parseInputs(pc.Inputs)
	cutoff := time.Now().UTC().AddDate(0, 0, -in.days)

	raws, err := fetchEvents(ctx, pc, login, in.load)
	if err != nil {
		// 5xx and network timeouts are transient — surface as
		// *RetryableError so the engine records them on Result.Errors
		// per contract §2.5. Permanent failures (4xx, malformed login)
		// stay as regular errors so the engine does not retry them.
		wrapped := fmt.Errorf("activity: %w", err)
		if isTransientFetchError(err) {
			return nil, xerrors.NewRetryableError(wrapped)
		}
		return nil, wrapped
	}

	// Phase 1: filter + classify events. The PR diff-stat fetch is
	// deferred to Phase 3 (after truncation) so we never spend REST
	// budget on events that get dropped by limit.
	type pendingPR struct {
		index  int // position in `events`
		number int
	}
	events := make([]ActivityEvent, 0, len(raws))
	pending := make([]pendingPR, 0)
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
		ae := ActivityEvent{
			Type:       raw.Type,
			Repo:       raw.Repo.Name,
			Date:       raw.CreatedAt,
			Visibility: vis,
		}
		// PullRequestEvent: capture embedded summary now, defer the
		// /pulls/{number} fallback fetch until after sort + truncate so
		// only the kept events spend REST budget.
		if raw.Type == "PullRequestEvent" && raw.Payload.PullRequest != nil {
			pr := raw.Payload.PullRequest
			if pr.Additions != 0 || pr.Deletions != 0 || pr.ChangedFiles != 0 {
				// Rare: GitHub did populate the embedded stats.
				ae.Files = &EventFiles{Changed: pr.ChangedFiles}
				ae.Lines = &EventLines{Added: pr.Additions, Deleted: pr.Deletions}
			} else if pr.Number > 0 {
				pending = append(pending, pendingPR{index: len(events), number: pr.Number})
			}
		}
		events = append(events, ae)
	}

	// Phase 2: sort + truncate to display limit. Doing this before the
	// PR detail fetch (Phase 3) caps the fallback API spend at `limit`
	// calls — not `load` (300). Without truncation the activity plugin
	// alone would burn ~300 REST calls per run in the worst case.
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Date.After(events[j].Date)
	})
	if len(events) > in.limit {
		events = events[:in.limit]
	}

	// Phase 3: fetch PR diff stats for kept events that lack them, with
	// dedup by (repo, number). The same PR can surface as several
	// PullRequestEvent rows (opened / synchronize / closed) — fetching
	// /pulls/{n} once and reusing the result keeps the REST spend at
	// O(unique PRs in window) rather than O(events). Surface a single
	// aggregated AppendError if any fetch hits rate-limit, mirroring
	// the #531 surface-degradation contract.
	type prKey struct {
		repo   string
		number int
	}
	type prStats struct {
		additions, deletions, changedFiles int
	}
	cache := make(map[prKey]prStats)
	var droppedRateLimit, droppedOther int
	var lastRLE *httpx.RateLimitedError
	for _, p := range pending {
		if p.index >= len(events) {
			continue // event was truncated away by Phase 2.
		}
		key := prKey{repo: events[p.index].Repo, number: p.number}
		stats, cached := cache[key]
		if !cached {
			a, d, c, err := fetchPullRequestStats(ctx, pc, key.repo, key.number)
			if err != nil {
				var rle *httpx.RateLimitedError
				if errors.As(err, &rle) {
					droppedRateLimit++
					lastRLE = rle
				} else {
					droppedOther++
				}
				continue // leave Files/Lines nil; partial skips the line.
			}
			stats = prStats{additions: a, deletions: d, changedFiles: c}
			cache[key] = stats
		}
		events[p.index].Files = &EventFiles{Changed: stats.changedFiles}
		events[p.index].Lines = &EventLines{Added: stats.additions, Deleted: stats.deletions}
	}
	if droppedRateLimit > 0 {
		pc.Data.AppendError(fmt.Errorf("activity: PR diff stats unavailable for %d PR(s) (rate limited): %w",
			droppedRateLimit, lastRLE))
	}
	if droppedOther > 0 {
		pc.Data.AppendError(fmt.Errorf("activity: PR diff stats unavailable for %d PR(s) (fetch failed)",
			droppedOther))
	}

	return &Result{
		Mode:   plugins.AggregationMode(pc.Data),
		Events: events,
		Days:   in.days,
	}, nil
}

// fetchPullRequestStats fetches the diff stats for a single PR via
// `/repos/{owner}/{repo}/pulls/{number}`. The events API embeds only a
// PR summary (base/head/id/number/url) and leaves additions/deletions/
// changed_files at zero — without this fallback the rendered card shows
// "0 files changed ++0 --0" which defeats the activity card's purpose.
//
// Returns (additions, deletions, changedFiles, error). The error is the
// raw transport / decode / status failure so callers can errors.As it
// against *httpx.RateLimitedError to distinguish rate-limit drops from
// other failures (#531 surface-degradation contract).
func fetchPullRequestStats(ctx context.Context, pc *plugins.PluginContext, repo string, number int) (int, int, int, error) {
	if pc == nil || pc.REST == nil {
		return 0, 0, 0, fmt.Errorf("nil REST client")
	}
	if number <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid PR number %d", number)
	}
	owner, name, ok := splitRepo(repo)
	if !ok {
		return 0, 0, 0, fmt.Errorf("invalid repo %q", repo)
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d",
		url.PathEscape(owner), url.PathEscape(name), number)
	body, resp, err := pc.REST.Get(ctx, path, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	if resp == nil {
		return 0, 0, 0, fmt.Errorf("nil response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Pass the response through ClassifyRateLimit so a 403/429 with
		// rate-limit headers surfaces as *httpx.RateLimitedError. Other
		// 4xx/5xx stay as plain fetchStatusError.
		if rle := httpx.ClassifyRateLimit(resp); rle != nil {
			return 0, 0, 0, rle
		}
		return 0, 0, 0, &fetchStatusError{status: resp.StatusCode}
	}
	var pr rawPullRequest
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, 0, 0, fmt.Errorf("decode PR: %w", err)
	}
	return pr.Additions, pr.Deletions, pr.ChangedFiles, nil
}

// splitRepo parses "owner/name" into its parts. Returns ok=false when
// the input is not the canonical two-segment form.
func splitRepo(repo string) (string, string, bool) {
	i := strings.IndexByte(repo, '/')
	if i <= 0 || i == len(repo)-1 {
		return "", "", false
	}
	return repo[:i], repo[i+1:], true
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
			return nil, &fetchStatusError{status: resp.StatusCode}
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

func parseInputs(in map[string]any) inputs {
	out := inputs{
		// Display limit. 5 matches upstream's metadata default
		// (assets/plugins/activity/metadata.yml plugin_activity_limit)
		// — the previous 100 rendered an over-long timeline (every event
		// in the 14-day window) instead of the short recent-activity card.
		limit: 5,
		load:  300,
		days:  14,
		// "all" matches the metadata-declared default
		// (assets/plugins/activity/metadata.yml
		// plugin_activity_visibility) — private events are surfaced
		// whenever the token can see them, like upstream.
		visibility: "all",
		skipped:    map[string]struct{}{},
		ignored:    map[string]struct{}{},
	}
	if v, ok := pluginutil.ReadInt(in, "plugin_activity_limit"); ok {
		out.limit = v
	}
	if v, ok := pluginutil.ReadInt(in, "plugin_activity_load"); ok {
		out.load = v
	}
	if v, ok := pluginutil.ReadInt(in, "plugin_activity_days"); ok {
		out.days = v
	}
	if v, ok := in["plugin_activity_visibility"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.visibility = s
		}
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_activity_skipped") {
		out.skipped[s] = struct{}{}
	}
	for _, s := range pluginutil.ReadCSV(in, "plugin_activity_ignored") {
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

// fetchStatusError carries the HTTP status code surfaced by the events
// endpoint so callers can distinguish transient (5xx) from permanent
// (4xx) failures without scraping error strings.
type fetchStatusError struct {
	status int
}

func (e *fetchStatusError) Error() string {
	return fmt.Sprintf("events status %d", e.status)
}

// isTransientFetchError reports whether err represents a retryable
// failure. A typed *fetchStatusError carries the HTTP status code
// directly (>= 500 = transient, 4xx = permanent). Other shapes — most
// notably retryablehttp's "giving up after N attempt(s)" wrapper that
// surfaces after the httpx layer exhausts its 5xx retries — are
// treated as transient too. Permanent shapes (4xx + non-network
// errors) stay permanent so the engine does not retry them.
func isTransientFetchError(err error) bool {
	var statusErr *fetchStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// retryablehttp surfaces post-exhaustion 5xx as a wrapped error
	// containing "giving up after"; httpx discards the underlying
	// response so we string-match the marker. Same approach base.go
	// uses for its paging-retry detection.
	if msg := err.Error(); strings.Contains(msg, "giving up after") {
		return true
	}
	return false
}
