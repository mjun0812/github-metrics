// Package languages — recent.go implements the "languages.recent"
// sub-mode of the M4 languages plugin. It walks PushEvents from
// /users/<login>/events, fetches per-commit file lists from
// /repos/<owner>/<name>/commits/<sha>, runs each filename through
// go-enry, and aggregates byte counts per language using the same
// favorites/other split as standard mode.
//
// The runtime is unconditionally compiled (no build tag) so the plugin
// registry sees a "languages.recent" entry on every build. The heavy
// test fixtures sit behind //go:build heavy in recent_heavy_test.go.
package languages

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

	enry "github.com/go-enry/go-enry/v2"
	enrydata "github.com/go-enry/go-enry/v2/data"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// RecentName is the canonical plugin slug for the recent sub-mode.
const RecentName = "languages.recent"

// RecentPlugin is the singleton registered with the global plugin
// registry. It runs the recent-mode aggregation against /users/X/events.
var RecentPlugin plugins.Plugin = &recentPlugin{}

func init() {
	plugins.Register(RecentPlugin)
}

type recentPlugin struct{}

func (p *recentPlugin) Name() string                     { return RecentName }
func (p *recentPlugin) Metadata() *config.PluginMetadata { return nil }

// RecentResult is the JSON payload published under
// data.Plugins["languages.recent"]. Field set mirrors data-model E-011.
type RecentResult struct {
	Skipped       bool                   `json:"skipped,omitempty"`
	SkippedReason string                 `json:"-"`
	Favorites     []plugins.LanguageStat `json:"favorites"`
	Other         plugins.LanguageStat   `json:"other"`
	Days          int                    `json:"days"`
	Load          int                    `json:"load"`
	Repos         []string               `json:"repos"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *RecentResult) IsSkipped() bool { return r != nil && r.Skipped }

type recentInputs struct {
	days       int
	load       int
	categories map[string]struct{}
	sections   []string
	// pluginInputs is reused to keep alias / ignored handling in sync
	// with standard mode.
	standard inputs
}

func (p *recentPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseRecentInputs(pc.Inputs)
	// Gate on `plugin_languages` truthy + sections containing
	// "recently-used". The plugin shares a top-level slug with standard
	// mode, so it only fires when both the languages dispatcher and the
	// recently-used section flag are on.
	if !truthy(pc.Inputs["plugin_languages"]) {
		return &RecentResult{
			Skipped:       true,
			SkippedReason: "plugin_languages not enabled",
			Favorites:     []plugins.LanguageStat{},
			Days:          in.days,
			Load:          in.load,
			Repos:         []string{},
		}, nil
	}
	if !containsString(in.sections, "recently-used") {
		return &RecentResult{
			Skipped:       true,
			SkippedReason: "recently-used section not requested",
			Favorites:     []plugins.LanguageStat{},
			Days:          in.days,
			Load:          in.load,
			Repos:         []string{},
		}, nil
	}
	if !extrasEnabled(pc.Inputs, "extras.metrics.run.linguist") {
		return &RecentResult{
			Skipped:       true,
			SkippedReason: "linguist disabled via extras",
			Favorites:     []plugins.LanguageStat{},
			Days:          in.days,
			Load:          in.load,
			Repos:         []string{},
		}, nil
	}
	if pc.REST == nil {
		return &RecentResult{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			Favorites:     []plugins.LanguageStat{},
			Days:          in.days,
			Load:          in.load,
			Repos:         []string{},
		}, nil
	}
	login := recentLoginFromInputs(pc.Inputs)
	if login == "" {
		return &RecentResult{
			Skipped:       true,
			SkippedReason: "no login",
			Favorites:     []plugins.LanguageStat{},
			Days:          in.days,
			Load:          in.load,
			Repos:         []string{},
		}, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -in.days)

	pushes, err := fetchPushEvents(ctx, pc, login, in.load)
	if err != nil {
		wrapped := fmt.Errorf("languages.recent: %w", err)
		if isTransientFetchError(err) {
			return nil, xerrors.NewRetryableError(wrapped)
		}
		return nil, wrapped
	}

	// Per-language accumulator. The standard-mode alias / ignored maps
	// apply here so users get one consistent view across modes.
	type acc struct {
		size  int
		count int
		color string
	}
	totals := map[string]*acc{}
	seenRepos := map[string]struct{}{}
	repos := []string{}
	// accumulate folds a commit/push file list into the per-language byte
	// totals, applying the standard-mode alias / ignored / category
	// filters so recent mode stays consistent with most-used mode.
	accumulate := func(files []rawCommitFile) {
		for _, f := range files {
			lang := enry.GetLanguage(f.Filename, nil)
			if lang == "" {
				continue
			}
			lang = canonicalLanguage(lang, in.standard.aliases)
			if _, drop := in.standard.ignored[lang]; drop {
				continue
			}
			if !categoryAllowed(lang, in.categories) {
				continue
			}
			bytes := f.Additions + f.Deletions
			if bytes <= 0 {
				continue
			}
			a, ok := totals[lang]
			if !ok {
				a = &acc{}
				totals[lang] = a
			}
			a.size += bytes
			a.count++
			if a.color == "" {
				a.color = colorFor(lang, in.standard.colors)
			}
		}
	}

	for _, pe := range pushes {
		if pe.CreatedAt.Before(cutoff) {
			continue
		}
		if _, seen := seenRepos[pe.Repo.Name]; !seen {
			seenRepos[pe.Repo.Name] = struct{}{}
			repos = append(repos, pe.Repo.Name)
		}
		// Prefer the explicit per-commit list when the events payload
		// still carries it. GitHub increasingly omits payload.commits
		// (leaving only before/head); in that case resolve the pushed
		// range through the compare API, mirroring the gh-metrics fork
		// of upstream metrics.
		if shas := pe.Payload.commitSHAs(); len(shas) > 0 {
			for _, sha := range shas {
				files, err := fetchCommitFiles(ctx, pc, pe.Repo.Name, sha)
				if err != nil {
					// Best-effort: a single commit miss should not abort
					// the whole walk. Record once on Data.Errors so
					// callers can see the degraded path.
					pc.Data.AppendError(fmt.Errorf("languages.recent: commit %s/%s: %w", pe.Repo.Name, sha, err))
					continue
				}
				accumulate(files)
			}
			continue
		}
		files, err := fetchPushFiles(ctx, pc, pe.Repo.Name, pe.Payload.Before, pe.Payload.Head)
		if err != nil {
			// Best-effort: a single push miss should not abort the walk.
			pc.Data.AppendError(fmt.Errorf("languages.recent: push %s (%s..%s): %w",
				pe.Repo.Name, shortSHA(pe.Payload.Before), shortSHA(pe.Payload.Head), err))
			continue
		}
		accumulate(files)
	}

	totalBytes := 0
	for _, a := range totals {
		totalBytes += a.size
	}
	if totalBytes == 0 {
		// PushEvent 0 件 / no analyzable files → empty Favorites, NOT
		// skipped per contract §1.6.
		return &RecentResult{
			Favorites: []plugins.LanguageStat{},
			Days:      in.days,
			Load:      len(pushes),
			Repos:     repos,
		}, nil
	}

	stats := make([]plugins.LanguageStat, 0, len(totals))
	for name, a := range totals {
		stats = append(stats, plugins.LanguageStat{
			Name:  name,
			Color: a.color,
			Size:  a.size,
			Count: a.count,
			Value: float64(a.size) / float64(totalBytes),
		})
	}
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Size != stats[j].Size {
			return stats[i].Size > stats[j].Size
		}
		return stats[i].Name < stats[j].Name
	})

	limit := in.standard.limit
	if limit <= 0 || limit > len(stats) {
		limit = len(stats)
	}
	favorites := append([]plugins.LanguageStat(nil), stats[:limit]...)
	other := plugins.LanguageStat{Name: "Other", Color: "#cccccc"}
	if in.standard.other && limit < len(stats) {
		for _, s := range stats[limit:] {
			other.Size += s.Size
			other.Count += s.Count
		}
		other.Value = float64(other.Size) / float64(totalBytes)
	}

	return &RecentResult{
		Favorites: favorites,
		Other:     other,
		Days:      in.days,
		Load:      len(pushes),
		Repos:     repos,
	}, nil
}

// --- REST plumbing ---------------------------------------------------

type rawPushEvent struct {
	Type      string         `json:"type"`
	Repo      rawPushRepo    `json:"repo"`
	CreatedAt time.Time      `json:"created_at"`
	Payload   rawPushPayload `json:"payload"`
}

type rawPushRepo struct {
	Name string `json:"name"`
}

type rawPushPayload struct {
	// Commits is the legacy per-commit list. GitHub increasingly omits it
	// from the events feed, so recent analysis falls back to Before/Head
	// plus the compare API when it is absent.
	Commits []rawPushCommit `json:"commits"`
	Before  string          `json:"before"`
	Head    string          `json:"head"`
}

func (p rawPushPayload) commitSHAs() []string {
	out := make([]string, 0, len(p.Commits))
	for _, c := range p.Commits {
		if c.SHA != "" {
			out = append(out, c.SHA)
		}
	}
	return out
}

type rawPushCommit struct {
	SHA string `json:"sha"`
}

type rawCommitFile struct {
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type rawCommit struct {
	Files []rawCommitFile `json:"files"`
}

func fetchPushEvents(ctx context.Context, pc *plugins.PluginContext, login string, load int) ([]rawPushEvent, error) {
	const perPage = 100
	out := make([]rawPushEvent, 0, load)
	for page := 1; len(out) < load; page++ {
		path := fmt.Sprintf("/users/%s/events?per_page=%d&page=%d",
			url.PathEscape(login), perPage, page)
		body, resp, err := pc.REST.Get(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("languages.recent: nil response")
		}
		// GitHub's user events feed only paginates ~300 events deep
		// (per_page=100 × 3 pages); requesting page 4+ returns HTTP 422
		// (Unprocessable Entity). Upstream metrics (languages
		// analyzer/recent.mjs) swallows that page-limit error and
		// proceeds with the events already fetched ("no more page to
		// load"). Mirror that: stop paginating on 422 and analyse
		// whatever we gathered so far instead of failing the whole
		// plugin. Other non-2xx (e.g. 5xx) still surface as errors so the
		// retryable path keeps working.
		if resp.StatusCode == 422 {
			break
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &recentFetchStatusError{status: resp.StatusCode}
		}
		var batch []rawPushEvent
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("languages.recent: decode events: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, e := range batch {
			if e.Type == "PushEvent" {
				out = append(out, e)
				if len(out) >= load {
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

func fetchCommitFiles(ctx context.Context, pc *plugins.PluginContext, repo, sha string) ([]rawCommitFile, error) {
	path := fmt.Sprintf("/repos/%s/commits/%s", repo, sha)
	body, resp, err := pc.REST.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("languages.recent: nil response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &recentFetchStatusError{status: resp.StatusCode}
	}
	var c rawCommit
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("languages.recent: decode commit: %w", err)
	}
	return c.Files, nil
}

// zeroSHA is git's all-zero object id. GitHub sends it as a push's
// `before` when a brand-new branch is created (nothing to diff against).
const zeroSHA = "0000000000000000000000000000000000000000"

func isZeroSHA(sha string) bool { return sha == "" || sha == zeroSHA }

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// rawCompare is the subset of the /compare response we consume: the
// aggregated file list across the pushed range.
type rawCompare struct {
	Files []rawCommitFile `json:"files"`
}

// fetchPushFiles resolves the files changed by a push via the compare API
// (/repos/{repo}/compare/{before}...{head}), which aggregates
// additions/deletions across every commit in the pushed range. This
// avoids depending on payload.commits, which GitHub frequently omits from
// the events feed. New-branch pushes carry an all-zero `before` with
// nothing to diff from, so fall back to the head commit alone.
func fetchPushFiles(ctx context.Context, pc *plugins.PluginContext, repo, before, head string) ([]rawCommitFile, error) {
	if head == "" {
		return nil, nil
	}
	if isZeroSHA(before) {
		return fetchCommitFiles(ctx, pc, repo, head)
	}
	path := fmt.Sprintf("/repos/%s/compare/%s...%s", repo, before, head)
	body, resp, err := pc.REST.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("languages.recent: nil response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &recentFetchStatusError{status: resp.StatusCode}
	}
	var c rawCompare
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("languages.recent: decode compare: %w", err)
	}
	return c.Files, nil
}

// --- input parsing ---------------------------------------------------

func parseRecentInputs(in map[string]any) recentInputs {
	out := recentInputs{
		days:       14,
		load:       100,
		categories: map[string]struct{}{"programming": {}},
		sections:   []string{"most-used"},
		standard:   parseInputs(in),
	}
	if v, ok := readInt(in, "plugin_languages_recent_days"); ok {
		out.days = v
	}
	if v, ok := readInt(in, "plugin_languages_recent_load"); ok {
		out.load = v
	}
	if cats := readCSV(in, "plugin_languages_recent_categories"); len(cats) > 0 {
		out.categories = map[string]struct{}{}
		for _, c := range cats {
			out.categories[strings.ToLower(c)] = struct{}{}
		}
	}
	if v, ok := in["plugin_languages_sections"]; ok {
		out.sections = readCSVValue(v)
		if len(out.sections) == 0 {
			out.sections = []string{"most-used"}
		}
	}
	return out
}

func recentLoginFromInputs(in map[string]any) string {
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

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// extrasEnabled returns true when the input key is absent OR truthy.
// Upstream's extras feature flags default to enabled; only an explicit
// false value disables them.
func extrasEnabled(in map[string]any, key string) bool {
	if in == nil {
		return true
	}
	v, ok := in[key]
	if !ok {
		return true
	}
	return truthy(v)
}

func categoryAllowed(lang string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	info, err := enry.GetLanguageInfo(lang)
	if err != nil {
		return false
	}
	cat := strings.ToLower(typeToString(info.Type))
	_, ok := allowed[cat]
	return ok
}

func typeToString(t enrydata.Type) string {
	switch t {
	case enrydata.TypeProgramming:
		return "programming"
	case enrydata.TypeMarkup:
		return "markup"
	case enrydata.TypeProse:
		return "prose"
	case enrydata.TypeData:
		return "data"
	default:
		return "unknown"
	}
}

func colorFor(lang string, overrides map[string]string) string {
	if c, ok := overrides[lang]; ok {
		return c
	}
	info, err := enry.GetLanguageInfo(lang)
	if err != nil {
		return ""
	}
	return info.Color
}

// --- error classification --------------------------------------------

type recentFetchStatusError struct {
	status int
}

func (e *recentFetchStatusError) Error() string {
	return fmt.Sprintf("languages.recent: status %d", e.status)
}

func isTransientFetchError(err error) bool {
	var statusErr *recentFetchStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if msg := err.Error(); strings.Contains(msg, "giving up after") {
		return true
	}
	return false
}
