// Package traffic owns the M4 "traffic" plugin. It aggregates per-
// repository view counts from REST /repos/{owner}/{repo}/traffic/views
// for every repository in base.Computed.RepositoryList. The endpoint
// requires the `repo` OAuth scope; without it the plugin returns
// Skipped=true. Repositories that return 403 (owner-only endpoint) are
// dropped silently and aggregation continues.
package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
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

// Result is the JSON payload published under data.Plugins["traffic"].
type Result struct {
	Skipped       bool                   `json:"skipped,omitempty"`
	SkippedReason string                 `json:"-"`
	Views         map[string]TrafficView `json:"views"`
	Total         TrafficView            `json:"total"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

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
// repository in Computed.RepositoryList. 403 (collaborator
// permissions) is treated as drop-and-continue per contract §12.
func (p *trafficPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.REST == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			Views:         map[string]TrafficView{},
		}, nil
	}
	scopes, err := pc.REST.Scopes(ctx)
	if err != nil {
		//nolint:nilerr // intentional: Scopes failure maps to Skipped
		return &Result{
			Skipped:       true,
			SkippedReason: "could not determine token scopes",
			Views:         map[string]TrafficView{},
		}, nil
	}
	if !hasScope(scopes, "repo") {
		return &Result{
			Skipped:       true,
			SkippedReason: "missing repo scope",
			Views:         map[string]TrafficView{},
		}, nil
	}

	repos := pc.Data.Computed.RepositoryList
	views := make(map[string]TrafficView, len(repos))
	var total TrafficView
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for _, repo := range repos {
		repo := repo
		g.Go(func() error {
			path := fmt.Sprintf("/repos/%s/traffic/views", urlPath(repo.NameWithOwner))
			body, resp, err := pc.REST.Get(gctx, path, nil)
			if err != nil {
				// Transport error — drop and continue per the
				// best-effort contract.
				//nolint:nilerr // intentional: per-repo failure does not abort the aggregation
				return nil
			}
			if resp == nil {
				return nil
			}
			if resp.StatusCode == http.StatusForbidden {
				return nil
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil
			}
			var raw rawTraffic
			if err := json.Unmarshal(body, &raw); err != nil {
				//nolint:nilerr // intentional: decode failure → drop one repo
				return nil
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

	return &Result{
		Views: views,
		Total: total,
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

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
