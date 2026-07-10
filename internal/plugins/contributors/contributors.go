// Package contributors owns the M4 "contributors" plugin. Per the
// contract this plugin targets the repository-template account kind,
// which M4 does not yet support (M7 territory). In M4 it always
// returns Skipped=true with an explanatory reason.
package contributors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "contributors"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &contributorsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type contributorsPlugin struct{}

func (p *contributorsPlugin) Name() string                     { return Name }
func (p *contributorsPlugin) Metadata() *config.PluginMetadata { return nil }

func (p *contributorsPlugin) Requires() []plugins.DataKey {
	// contributors is repo-mode only and reads pc.Data.Computed.RepositoryList;
	// it does not call Provider directly.
	return []plugins.DataKey{}
}

// Result is the JSON payload published under data.Plugins["contributors"].
type Result struct {
	Skipped       bool          `json:"skipped,omitempty"`
	SkippedReason string        `json:"-"`
	Mode          string        `json:"mode,omitempty"`
	Contributions bool          `json:"contributions"`
	List          []Contributor `json:"list"`
	Sections      []string      `json:"sections"`
	Base          string        `json:"base"`
	Head          string        `json:"head"`
	// StatsPending is true when GET /repos/{owner}/{repo}/stats/contributors
	// returned 202 Accepted ("warming up"). The partial uses this to
	// render a "stats pending" indicator instead of misleading ++0 --0
	// chips when contributions mode is enabled.
	StatsPending bool `json:"stats_pending,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Contributor mirrors one entry from the upstream contributors list.
type Contributor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
	Commits   int    `json:"commits"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Run surfaces a repo-mode contributor summary when the engine has
// populated data.Repo (M7 repository template). User and organization
// modes continue to return Skipped — they have no meaningful single
// base/head commit range.
func (p *contributorsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)
	if r := pc.Data.RepoRef(); r != nil {
		// Repo-mode: keep the existing minimal fallback from
		// base.FetchRepo, then replace it with detailed REST stats when
		// /stats/contributors is ready.
		mode := plugins.AggregationMode(pc.Data)
		list := fallbackList(r, in.ignored)
		statsPending := false
		if pc.REST != nil {
			loaded, status := fetchContributorStats(ctx, pc, r.Owner, r.Name, in.ignored)
			switch status {
			case statsStatusOK:
				list = loaded
			case statsStatusPending:
				if fallback, ok := fetchContributorList(ctx, pc, r.Owner, r.Name, in.ignored); ok {
					list = fallback
				}
				statsPending = true
			case statsStatusFailed:
				if fallback, ok := fetchContributorList(ctx, pc, r.Owner, r.Name, in.ignored); ok {
					list = fallback
				}
				pc.Data.AppendError(fmt.Errorf("contributors: /stats/contributors unavailable for %s/%s (stats fetch failed)", r.Owner, r.Name))
			}
		}
		return &Result{
			Mode:          mode,
			Contributions: in.contributions,
			List:          list,
			Sections:      in.sections,
			Base:          defaultString(in.base, r.DefaultBranch),
			Head:          defaultString(in.head, "HEAD"),
			StatsPending:  statsPending,
		}, nil
	}
	reason, _ := plugins.RequireRepoMode(pc, Name)
	if reason == "" {
		reason = "plugin contributors is only supported in repository mode (current mode: user)"
	}
	return &Result{
		Skipped:       true,
		SkippedReason: reason,
		Mode:          plugins.ModeUser,
		Contributions: in.contributions,
		List:          []Contributor{},
		Sections:      []string{},
	}, nil
}

type rawContributor struct {
	Login         string `json:"login"`
	AvatarURL     string `json:"avatar_url"`
	Contributions int    `json:"contributions"`
}

// contributorsMaxPages caps the /repos/{owner}/{repo}/contributors
// fallback pagination so a repository with thousands of contributors
// cannot spin an unbounded fetch loop. 10 pages * per_page=100 = 1000
// contributors.
const contributorsMaxPages = 10

func fetchContributorList(ctx context.Context, pc *plugins.PluginContext, owner, repo string, ignored map[string]struct{}) ([]Contributor, bool) {
	if pc == nil || pc.REST == nil {
		return nil, false
	}
	out := make([]Contributor, 0)
	page := 1
	for p := 0; p < contributorsMaxPages; p++ {
		path := fmt.Sprintf(
			"/repos/%s/%s/contributors?per_page=100&anon=true&page=%d",
			url.PathEscape(owner), url.PathEscape(repo), page,
		)
		body, resp, err := pc.REST.Get(ctx, path, nil)
		if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
			return nil, false
		}
		var rows []rawContributor
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, false
		}
		for _, row := range rows {
			if row.Login == "" || ignoredLogin(row.Login, ignored) {
				continue
			}
			out = append(out, Contributor{
				Login:     row.Login,
				AvatarURL: row.AvatarURL,
				Commits:   row.Contributions,
			})
		}
		next := pluginutil.NextPageFromLink(resp.Header.Get("Link"))
		if next <= 0 {
			break
		}
		page = next
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Login < out[j].Login
	})
	return out, len(out) > 0
}

type inputs struct {
	base          string
	head          string
	contributions bool
	sections      []string
	ignored       map[string]struct{}
}

func parseInputs(raw map[string]any) inputs {
	return inputs{
		base:          readString(raw, "plugin_contributors_base", ""),
		head:          readString(raw, "plugin_contributors_head", "master"),
		contributions: pluginutil.ReadBool(raw, "plugin_contributors_contributions"),
		sections:      readStringSlice(raw, "plugin_contributors_sections", []string{"contributors"}),
		ignored:       readStringSet(raw, "plugin_contributors_ignored"),
	}
}

func fallbackList(r *plugins.Repo, ignored map[string]struct{}) []Contributor {
	if r == nil || r.Contributors <= 0 || ignoredLogin(r.Owner, ignored) {
		return []Contributor{}
	}
	return []Contributor{{
		Login:     r.Owner,
		AvatarURL: r.OwnerAvatar,
		Commits:   r.Activity.RecentCommits,
	}}
}

type rawContributorStat struct {
	Total int `json:"total"`
	Weeks []struct {
		Additions int `json:"a"`
		Deletions int `json:"d"`
		Commits   int `json:"c"`
	} `json:"weeks"`
	Author *struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"author"`
}

// statsStatus describes the outcome of /stats/contributors. We
// distinguish between OK (full payload available), Pending (GitHub
// returned 202 Accepted while it warms the cache — caller should
// keep the minimal stub but tell the partial to render a "stats
// pending" indicator instead of misleading zero diff chips), and
// Failed (transport / parse error → silently keep the minimal stub).
type statsStatus int

const (
	statsStatusFailed statsStatus = iota
	statsStatusOK
	statsStatusPending
)

// statsPendingMaxAttempts bounds how many times we re-request
// /stats/contributors while GitHub returns 202 Accepted (cache warm-up).
// GitHub recomputes contributor statistics asynchronously the first time
// they are requested and answers 202 with an empty body until the cache
// is ready; mirroring upstream metrics we poll a handful of times before
// giving up and surfacing the "stats pending" placeholder.
const statsPendingMaxAttempts = 5

// statsPendingBackoff is the wait between 202 retries. Upstream uses a
// few seconds; we keep it modest so a warm cache (the common case) costs
// at most one extra round trip and a cold cache resolves within a couple
// of attempts without blocking the run for long.
const statsPendingBackoff = 2 * time.Second

// sleepFn is indirected so unit tests can drive the 202 retry loop
// without real wall-clock delays. Production uses time.Sleep.
var sleepFn = func(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func fetchContributorStats(ctx context.Context, pc *plugins.PluginContext, owner, repo string, ignored map[string]struct{}) ([]Contributor, statsStatus) {
	path := fmt.Sprintf("/repos/%s/%s/stats/contributors", url.PathEscape(owner), url.PathEscape(repo))

	var body []byte
	for attempt := 0; ; attempt++ {
		var resp *http.Response
		var err error
		body, resp, err = pc.REST.Get(ctx, path, nil)
		if err != nil || resp == nil {
			return nil, statsStatusFailed
		}
		// 202 Accepted: GitHub is asynchronously computing the
		// statistics and returns an empty body. Poll a bounded number
		// of times before giving up so a cold cache (which warms within
		// seconds) still yields real per-contributor commits/diffs
		// instead of the misleading "stats pending" placeholder.
		if resp.StatusCode == http.StatusAccepted {
			if attempt >= statsPendingMaxAttempts-1 {
				return nil, statsStatusPending
			}
			sleepFn(ctx, statsPendingBackoff)
			if ctx.Err() != nil {
				return nil, statsStatusPending
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, statsStatusFailed
		}
		break
	}

	var rows []rawContributorStat
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, statsStatusFailed
	}
	out := make([]Contributor, 0, len(rows))
	for _, row := range rows {
		if row.Author == nil || row.Author.Login == "" || ignoredLogin(row.Author.Login, ignored) {
			continue
		}
		c := Contributor{
			Login:     row.Author.Login,
			AvatarURL: row.Author.AvatarURL,
			Commits:   row.Total,
		}
		for _, week := range row.Weeks {
			if c.Commits == 0 {
				c.Commits += week.Commits
			}
			c.Additions += week.Additions
			c.Deletions += week.Deletions
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Login < out[j].Login
	})
	return out, statsStatusOK
}

func ignoredLogin(login string, ignored map[string]struct{}) bool {
	_, ok := ignored[strings.ToLower(strings.TrimSpace(login))]
	return ok
}

func readString(in map[string]any, key, def string) string {
	if in == nil {
		return def
	}
	if v, ok := in[key]; ok {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" {
			return s
		}
	}
	return def
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func readStringSlice(in map[string]any, key string, def []string) []string {
	if in == nil {
		return append([]string(nil), def...)
	}
	switch v := in[key].(type) {
	case []string:
		return cleanStrings(v, def)
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, fmt.Sprint(item))
		}
		return cleanStrings(values, def)
	case string:
		return cleanStrings(strings.Split(v, ","), def)
	default:
		return append([]string(nil), def...)
	}
}

func readStringSet(in map[string]any, key string) map[string]struct{} {
	values := readStringSlice(in, key, nil)
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[strings.ToLower(v)] = struct{}{}
	}
	return out
}

func cleanStrings(values, def []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	if len(out) == 0 && def != nil {
		return append([]string(nil), def...)
	}
	return out
}
