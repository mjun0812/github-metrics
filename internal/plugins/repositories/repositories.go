// Package repositories owns the M4 "repositories" plugin. It filters,
// sorts, and presents the user's repository list — the data the base
// plugin's paging loop already produced into
// pc.Data.Computed.RepositoryList.
//
// MVP scope (Phase 3 US1):
//   - Featured: filter by `_forks` / `_skipped` and sort by `_order`
//     (stars / forks / watchers).
//   - Random: deterministic Fisher-Yates over Featured when
//     `_random=true` (seed via `_random_seed`).
//   - Starred (012): fetched from `/users/<login>/starred` when
//     `_starred=true`. Falls back to the Featured-copy placeholder
//     when REST is unavailable (test paths / no token).
//
// Inputs accepted for upstream-compat but NOT yet wired in this MVP:
//   - `plugin_repositories_affiliations`: parsed but ignored. Base
//     surfaces OWNER repositories regardless. Future US2 work will
//     route this through a dedicated GraphQL query.
//   - `plugin_repositories_pinned`: when set, the corresponding Result
//     section is populated by reusing Featured as a placeholder. The
//     dedicated `user.pinnedItems` GraphQL fragment lands in the 013 PR.
package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "repositories"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &repositoriesPlugin{}

func init() {
	plugins.Register(Plugin)
}

type repositoriesPlugin struct{}

func (p *repositoriesPlugin) Name() string                     { return Name }
func (p *repositoriesPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["repositories"].
type Result struct {
	Skipped       bool                 `json:"skipped,omitempty"`
	SkippedReason string               `json:"-"`
	Featured      []plugins.Repository `json:"featured"`
	Pinned        []plugins.Repository `json:"pinned,omitempty"`
	Starred       []plugins.Repository `json:"starred,omitempty"`
	Random        []plugins.Repository `json:"random,omitempty"`
}

// IsSkipped lets the classic dispatcher (and any duck-typed consumer)
// detect the skipped path uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

type inputs struct {
	order        string
	pinned       bool
	starred      bool
	random       bool
	includeForks bool
	skipped      map[string]struct{}
	affiliations map[string]struct{}
	limit        int
	randomSeed   int64
}

func (p *repositoriesPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			Featured:      []plugins.Repository{},
		}, nil
	}
	repos := pc.Data.Computed.RepositoryList
	if len(repos) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no repositories",
			Featured:      []plugins.Repository{},
		}, nil
	}
	in := parseInputs(pc.Inputs)

	// The `affiliations` input is accepted by parseInputs for upstream-
	// compat but does not gate filtering in this MVP — base only
	// surfaces OWNER repositories today, so post-filtering would be a
	// no-op. The dedicated US2 GraphQL fragment will wire this up; the
	// package doc lists this carve-out.
	filtered := make([]plugins.Repository, 0, len(repos))
	for _, r := range repos {
		if !in.includeForks && r.IsFork {
			continue
		}
		if _, drop := in.skipped[r.NameWithOwner]; drop {
			continue
		}
		filtered = append(filtered, r)
	}

	sortRepositories(filtered, in.order)
	featured := filtered
	if in.limit > 0 && len(featured) > in.limit {
		featured = featured[:in.limit]
	}

	res := &Result{Featured: featured}
	if in.random {
		res.Random = randomSubset(featured, in.limit, in.randomSeed)
	}
	if in.pinned {
		// Spec 013: real viewer.pinnedItems GraphQL fetch. When the
		// GraphQL client is unavailable (test paths / dryrun-no-token CLI)
		// fall back to the Featured copy for M4 baseline compatibility.
		if pc.GraphQL == nil {
			res.Pinned = featured
		} else {
			pinned, perr := fetchPinned(ctx, pc)
			if perr != nil {
				pc.Data.AppendError(xerrors.NewRetryableError(perr))
				res.Pinned = nil
			} else {
				res.Pinned = pinned
			}
		}
	}
	if in.starred {
		res.Starred = resolveStarred(ctx, pc, featured, in.limit)
	}
	return res, nil
}

// starredFetchTimeout caps the single REST request to `/users/<login>/starred`.
// Mirrors the per-request timeout used by `languages.indepth`.
const starredFetchTimeout = 30 * time.Second

// resolveStarred returns the value to assign to Result.Starred when the
// `plugin_repositories_starred` input is truthy. Three paths:
//
//  1. REST + login available → live fetch from `/users/<login>/starred`.
//     On HTTP failure: log via *RetryableError on Data.Errors and return
//     nil (the section is suppressed in the JSON via omitempty).
//  2. REST nil but login present → fall back to the M4 baseline placeholder
//     (Featured copy) for test paths and no-token CLI invocations.
//  3. Login empty → return an empty slice (no network call).
func resolveStarred(ctx context.Context, pc *plugins.PluginContext, featured []plugins.Repository, limit int) []plugins.Repository {
	login := ""
	if pc.Data != nil && pc.Data.User != nil {
		login = pc.Data.User.Login
	}
	if login == "" {
		return []plugins.Repository{}
	}
	if pc.REST == nil {
		// FR-006: keep the M4 baseline placeholder for non-REST paths
		// (unit tests that don't construct a REST client, dryrun-no-token
		// flows, etc.) so the partial still renders something.
		return featured
	}

	reqCtx, cancel := context.WithTimeout(ctx, starredFetchTimeout)
	defer cancel()
	starred, err := fetchStarred(reqCtx, pc.REST, login, limit)
	if err != nil {
		pc.Data.AppendError(xerrors.NewRetryableError(fmt.Errorf("repositories.starred: %w", err)))
		return nil
	}
	return starred
}

// starredREST is the subset of *githubapi.REST that fetchStarred needs.
// Defining the interface here (instead of taking *REST directly) keeps
// the helper unit-testable without spinning up a mock transport — see
// the table-test cases that supply an in-memory stub.
type starredREST interface {
	Get(ctx context.Context, path string, extra http.Header) ([]byte, *http.Response, error)
}

// Compile-time assertion: *githubapi.REST satisfies starredREST.
var _ starredREST = (*githubapi.REST)(nil)

// rawStarredRepo mirrors the subset of the GitHub REST repository payload
// (`GET /users/{login}/starred`) that maps onto plugins.Repository.
// Fields not on this list (license, homepage, default_branch, etc.) are
// deliberately ignored — the partial doesn't render them.
type rawStarredRepo struct {
	NameWithOwner string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	Stars         int    `json:"stargazers_count"`
	Forks         int    `json:"forks_count"`
	Watchers      int    `json:"watchers_count"`
	Language      string `json:"language"`
}

// fetchStarred calls the REST starred endpoint and returns the mapped
// repository slice, truncated to limit. Returns an error when the
// transport fails or the response status is non-2xx; the caller wraps
// it in a *RetryableError before recording.
func fetchStarred(ctx context.Context, rest starredREST, login string, limit int) ([]plugins.Repository, error) {
	const perPage = 100
	path := fmt.Sprintf("/users/%s/starred?per_page=%d&sort=created&direction=desc", login, perPage)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("nil response for %s", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	var raw []rawStarredRepo
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if limit > 0 && len(raw) > limit {
		raw = raw[:limit]
	}
	out := make([]plugins.Repository, 0, len(raw))
	for _, r := range raw {
		vis := "public"
		if r.Private {
			vis = "private"
		}
		entry := plugins.Repository{
			NameWithOwner: r.NameWithOwner,
			Description:   r.Description,
			URL:           r.HTMLURL,
			Visibility:    vis,
			IsFork:        r.Fork,
			Stars:         r.Stars,
			Forks:         r.Forks,
			Watchers:      r.Watchers,
		}
		if r.Language != "" {
			entry.Language = &plugins.LanguageStat{Name: r.Language}
		}
		out = append(out, entry)
	}
	return out, nil
}

func sortRepositories(s []plugins.Repository, order string) {
	switch strings.ToLower(order) {
	case "forks":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Forks != s[j].Forks {
				return s[i].Forks > s[j].Forks
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	case "watchers":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Watchers != s[j].Watchers {
				return s[i].Watchers > s[j].Watchers
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	case "stars", "stargazers", "":
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Stars != s[j].Stars {
				return s[i].Stars > s[j].Stars
			}
			return s[i].NameWithOwner < s[j].NameWithOwner
		})
	}
}

// randomSubset returns at most `limit` repositories drawn from src via
// fisher-yates on a copy. seed=0 picks a deterministic seed=1 so tests
// can pin the order without exporting a knob.
func randomSubset(src []plugins.Repository, limit int, seed int64) []plugins.Repository {
	if seed == 0 {
		seed = 1
	}
	out := make([]plugins.Repository, len(src))
	copy(out, src)
	// Deterministic shuffle for test reproducibility; not crypto.
	// #nosec G404 -- math/rand is intentional for seeded fisher-yates.
	r := rand.New(rand.NewSource(seed))
	for i := len(out) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func parseInputs(in map[string]any) inputs {
	out := inputs{
		order:        "stars",
		limit:        6,
		affiliations: map[string]struct{}{},
		skipped:      map[string]struct{}{},
	}
	if v, ok := in["plugin_repositories_order"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.order = s
		}
	}
	if v, ok := in["plugin_repositories_pinned"]; ok {
		out.pinned = truthy(v)
	}
	if v, ok := in["plugin_repositories_starred"]; ok {
		out.starred = truthy(v)
	}
	if v, ok := in["plugin_repositories_random"]; ok {
		out.random = truthy(v)
	}
	if v, ok := in["plugin_repositories_forks"]; ok {
		out.includeForks = truthy(v)
	}
	for _, s := range readCSV(in, "plugin_repositories_affiliations") {
		out.affiliations[strings.ToUpper(s)] = struct{}{}
	}
	for _, s := range readCSV(in, "plugin_repositories_skipped") {
		out.skipped[s] = struct{}{}
	}
	if v, ok := readInt(in, "plugin_repositories_batch"); ok {
		out.limit = v
	}
	if v, ok := readInt(in, "plugin_repositories_random_seed"); ok {
		out.randomSeed = int64(v)
	}
	return out
}

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

// fetchPinned executes the spec-013 viewer.pinnedItems GraphQL query
// and maps each Repository node to a plugins.Repository. Gist nodes
// (the other PinnableItem variant) are skipped — the union path is
// covered by the schema for exhaustiveness but the projection only
// emits repository fields.
func fetchPinned(ctx context.Context, pc *plugins.PluginContext) ([]plugins.Repository, error) {
	resp, err := pc.GraphQL.ViewerPinnedItems(ctx, 6)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Viewer == nil || resp.Viewer.PinnedItems == nil {
		return []plugins.Repository{}, nil
	}
	out := make([]plugins.Repository, 0, len(resp.Viewer.PinnedItems.Nodes))
	for _, n := range resp.Viewer.PinnedItems.Nodes {
		if n == nil {
			continue
		}
		repo, ok := pinnableToRepository(n)
		if !ok {
			continue
		}
		out = append(out, repo)
	}
	return out, nil
}

func pinnableToRepository(node githubapi.ViewerPinnedItemsViewerUserPinnedItemsPinnableItemConnectionNodesPinnableItem) (plugins.Repository, bool) {
	r, ok := node.(*githubapi.ViewerPinnedItemsViewerUserPinnedItemsPinnableItemConnectionNodesRepository)
	if !ok {
		return plugins.Repository{}, false
	}
	desc := ""
	if r.Description != nil {
		desc = *r.Description
	}
	repo := plugins.Repository{
		NameWithOwner: r.NameWithOwner,
		Description:   desc,
		URL:           r.Url,
		IsFork:        r.IsFork,
		Stars:         r.StargazerCount,
		Forks:         r.ForkCount,
	}
	if r.IsPrivate {
		repo.Visibility = "PRIVATE"
	} else {
		repo.Visibility = "PUBLIC"
	}
	if r.PrimaryLanguage != nil {
		color := ""
		if r.PrimaryLanguage.Color != nil {
			color = *r.PrimaryLanguage.Color
		}
		repo.Language = &plugins.LanguageStat{Name: r.PrimaryLanguage.Name, Color: color}
	}
	return repo, true
}
