// Package traffic owns the M4 "traffic" plugin. It aggregates per-
// repository view counts from REST /repos/{owner}/{repo}/traffic/views
// for every repository in base.Computed.RepositoryList. The endpoint
// requires the `repo` OAuth scope; without it the plugin returns
// Skipped=true. Repositories that fail are dropped and aggregation
// continues; up to three aggregated AppendErrors are recorded (one per
// non-zero bucket: rate limited / forbidden / failed) so the degradation
// is observable via Result.Errors / JSON output.
package traffic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "traffic"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &trafficPlugin{}

func init() {
	plugins.Register(Plugin)
}

type trafficPlugin struct{}

func (p *trafficPlugin) Name() string                     { return Name }
func (p *trafficPlugin) Metadata() *config.PluginMetadata { return nil }

// Requires reports the Provider data sources Run reads. traffic walks
// the repository list via pc.Provider.Repositories and aggregates per-
// repo views/clones from REST.
func (p *trafficPlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{plugins.KeyRepositories}
}

// Result is the JSON payload published under data.Plugins["traffic"].
type Result struct {
	Skipped       bool                   `json:"skipped,omitempty"`
	SkippedReason string                 `json:"-"`
	Views         map[string]TrafficView `json:"views"`
	Total         TrafficView            `json:"total"`
	// HideEmpty controls whether per-repo entries with Count==0 are
	// filtered out at render time. It mirrors the
	// `plugin_traffic_hide_empty` input (default true).
	HideEmpty bool `json:"hide_empty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// TotalViews exposes the aggregate view count without forcing base
// partials to import this package and create an init-time cycle.
func (r *Result) TotalViews() int {
	if r == nil {
		return 0
	}
	return r.Total.Count
}

// TrafficView mirrors the upstream {count, uniques} pair.
type TrafficView struct {
	Count   int `json:"count"`
	Uniques int `json:"uniques"`
}

// rawTraffic matches the JSON shape GitHub returns from
// /repos/{owner}/{repo}/traffic/views.
type rawTraffic struct {
	Count   int `json:"count"`
	Uniques int `json:"uniques"`
}

// Run gates on `repo` scope, then parallel-fetches views for every
// repository in Computed.RepositoryList. Failures are drop-and-continue
// per contract §12; failures are classified into rate-limited /
// forbidden / failed buckets and surfaced via AppendError.
func (p *trafficPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	hideEmpty := pluginutil.ReadBoolDefault(pc.Inputs, "plugin_traffic_hide_empty", true)
	if pc.REST == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			Views:         map[string]TrafficView{},
			HideEmpty:     hideEmpty,
		}, nil
	}
	scopes, err := pc.REST.Scopes(ctx)
	if err != nil {
		//nolint:nilerr // intentional: Scopes failure maps to Skipped
		return &Result{
			Skipped:       true,
			SkippedReason: "could not determine token scopes",
			Views:         map[string]TrafficView{},
			HideEmpty:     hideEmpty,
		}, nil
	}
	if !slices.Contains(scopes, "repo") {
		return &Result{
			Skipped:       true,
			SkippedReason: "missing repo scope",
			Views:         map[string]TrafficView{},
			HideEmpty:     hideEmpty,
		}, nil
	}

	repos := resolveRepositories(ctx, pc)
	views := make(map[string]TrafficView, len(repos))
	var total TrafficView
	var mu sync.Mutex

	// Three drop counters — all aggregated into at most one AppendError each
	// after the loop so a 50-repo list does not spam 50 log entries.
	//
	// Classification:
	//   droppedRateLimit — errors.As(*httpx.RateLimitedError) on the error
	//     path (retry exhaustion or beyond-cap single attempt), or a response
	//     with rate-limit headers classified by httpx.ClassifyRateLimit(resp)
	//     (defense-in-depth for responses that reach the resp != nil path).
	//   droppedForbidden — resp.StatusCode == 403 with no rate-limit headers
	//     (plain permission error, not retried by the httpx layer).
	//   droppedFailed — everything else: transport errors, nil resp, other
	//     non-2xx, and JSON-decode failures.
	var droppedRateLimit atomic.Int64
	var droppedForbidden atomic.Int64
	var droppedFailed atomic.Int64

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for _, repo := range repos {
		repo := repo
		g.Go(func() error {
			path := fmt.Sprintf("/repos/%s/traffic/views", urlPath(repo.NameWithOwner))
			body, resp, err := pc.REST.Get(gctx, path, nil)
			if err != nil {
				// Transport error or rate-limit retry exhaustion.
				// Use errors.As to detect *httpx.RateLimitedError (set by the
				// httpx ErrorHandler on retry exhaustion or beyond-cap responses).
				var rle *httpx.RateLimitedError
				if errors.As(err, &rle) {
					droppedRateLimit.Add(1)
				} else {
					droppedFailed.Add(1)
				}
				return nil //nolint:nilerr // intentional: per-repo failure does not abort the aggregation
			}
			if resp == nil {
				droppedFailed.Add(1)
				return nil
			}
			// Defense-in-depth: classify any response that carries rate-limit
			// headers (e.g. a beyond-cap 403 that flowed through the resp path).
			if httpx.ClassifyRateLimit(resp) != nil {
				droppedRateLimit.Add(1)
				return nil //nolint:nilerr // intentional: *RateLimitedError is not propagated; the drop counter carries the signal
			}
			if resp.StatusCode == http.StatusForbidden {
				// Plain 403: no rate-limit headers (already ruled out above).
				droppedForbidden.Add(1)
				return nil
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				droppedFailed.Add(1)
				return nil
			}
			var raw rawTraffic
			if err := json.Unmarshal(body, &raw); err != nil {
				droppedFailed.Add(1)
				return nil //nolint:nilerr // intentional: decode failure → drop one repo
			}
			mu.Lock()
			views[repo.NameWithOwner] = TrafficView(raw)
			total.Count += raw.Count
			total.Uniques += raw.Uniques
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	// Surface aggregated errors (at most one per non-zero bucket) so operators
	// can distinguish a correct empty state from a degraded one.
	repoCount := len(repos)
	if rl := droppedRateLimit.Load(); rl > 0 {
		pc.Data.AppendError(fmt.Errorf("traffic: views unavailable for %d/%d repos (rate limited)", rl, repoCount))
	}
	if fb := droppedForbidden.Load(); fb > 0 {
		pc.Data.AppendError(fmt.Errorf("traffic: views unavailable for %d/%d repos (forbidden)", fb, repoCount))
	}
	if fl := droppedFailed.Load(); fl > 0 {
		pc.Data.AppendError(fmt.Errorf("traffic: views unavailable for %d/%d repos (failed)", fl, repoCount))
	}

	return &Result{
		Views:     views,
		Total:     total,
		HideEmpty: hideEmpty,
	}, nil
}

// urlPath splits "owner/name" into url.PathEscape("owner")/url.PathEscape("name")
// so that names with slashes (forks etc.) do not corrupt the path.
func urlPath(nameWithOwner string) string {
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) != 2 {
		return url.PathEscape(nameWithOwner)
	}
	return url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}

// resolveRepositories reads the per-node accumulator via the shared
// dataprovider (#603), falling back to pc.Data.Computed.RepositoryList
// for unit tests that build PluginContext by hand without wiring a
// Provider. Returns nil when neither source carries any entries.
func resolveRepositories(ctx context.Context, pc *plugins.PluginContext) []plugins.Repository {
	if pc == nil {
		return nil
	}
	if pc.Provider != nil {
		if repos, err := pc.Provider.Repositories(ctx); err == nil && repos != nil {
			return repos
		}
	}
	if pc.Data != nil {
		return pc.Data.Computed.RepositoryList
	}
	return nil
}
