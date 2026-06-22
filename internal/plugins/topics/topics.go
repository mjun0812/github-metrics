// Package topics owns the M4 "topics" plugin. It fetches the user's
// starred-topics page via plain HTTPS + goquery — the GitHub API does
// not expose the dataset, but `https://github.com/stars/{user}/topics`
// is fully server-rendered HTML (no React hydration, no
// include-fragment), so a single GET is enough. No headless browser
// required.
//
// The Navigator interface is preserved as a test seam: production code
// uses httpNavigator; non-network tests substitute a fake. The
// upstream chromedp-backed implementation is gone.
package topics

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "topics"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &topicsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type topicsPlugin struct{}

func (p *topicsPlugin) Name() string                     { return Name }
func (p *topicsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["topics"].
// Field set mirrors data-model E-023.
type Result struct {
	Skipped       bool    `json:"skipped,omitempty"`
	SkippedReason string  `json:"-"`
	List          []Topic `json:"list"`
	Mode          string  `json:"mode"`
	Limit         int     `json:"limit"`
	Sort          string  `json:"sort"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Topic mirrors one entry in the starred-topics list.
type Topic struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	URL         string `json:"url"`
	// StarredAt is opaque metadata used only for sorting when the user
	// requested sort="starred-at". Not part of the public JSON shape;
	// upstream does not expose it.
	StarredAt string `json:"-"`
}

// Navigator abstracts the HTTP fetch + HTML parse so non-network tests
// can inject a fake. Production code uses httpNavigator from
// http_navigator.go.
type Navigator interface {
	Fetch(ctx context.Context, url string) ([]Topic, error)
}

// NavigatorKey is the inputs map slot tests use to inject a fake
// Navigator. Production code never sets this — the plugin falls back to
// pc.Render.
const NavigatorKey = "_test_topics_navigator"

type topicsInputs struct {
	mode  string
	limit int
	sort  string
}

func (p *topicsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	in := parseInputs(pc.Inputs)

	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	// Gate: the user must enable the plugin via `plugin_topics`. Without
	// this gate the plugin would always run and pollute Result.Errors
	// with "chromedp not available" entries even when topics was never
	// requested.
	if !pluginutil.Truthy(pc.Inputs["plugin_topics"]) {
		return &Result{
			Skipped:       true,
			SkippedReason: "plugin_topics not enabled",
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	if !pluginutil.ExtrasEnabled(pc.Inputs, "extras.metrics.run.puppeteer.scrapping") {
		return &Result{
			Skipped:       true,
			SkippedReason: "puppeteer scrapping disabled via extras",
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	nav := pickNavigator(pc)
	// pickNavigator always returns a non-nil Navigator: tests can inject
	// one via NavigatorKey, otherwise we fall back to a stdlib-backed
	// httpNavigator. (No external dependency on chromedp / browsers any
	// more — the GitHub stars-topics page is fully SSR.)

	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{
			Skipped:       true,
			SkippedReason: "no login",
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	url := fmt.Sprintf("https://github.com/stars/%s/topics", login)
	list, err := nav.Fetch(ctx, url)
	if err != nil {
		wrapped := fmt.Errorf("topics: %w", err)
		// Network errors (timeouts, transient 5xx, context cancellation)
		// are surfaced as *RetryableError so the engine can retry or
		// expose them on Result.Errors per contract §3.6.
		return nil, xerrors.NewRetryableError(wrapped)
	}

	switch in.sort {
	case "starred-at":
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].StarredAt > list[j].StarredAt
		})
	default:
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
	}
	if in.limit > 0 && len(list) > in.limit {
		list = list[:in.limit]
	}

	return &Result{
		List:  list,
		Mode:  in.mode,
		Limit: in.limit,
		Sort:  in.sort,
	}, nil
}

// pickNavigator returns the test-injected Navigator if present.
// Otherwise it returns a stdlib-backed httpNavigator. Result is never
// nil — the topics page does not depend on chromium / a logged-in
// session, so the plugin no longer has a "navigator unavailable" path.
func pickNavigator(pc *plugins.PluginContext) Navigator {
	if pc != nil && pc.Inputs != nil {
		if v, ok := pc.Inputs[NavigatorKey]; ok {
			if n, ok := v.(Navigator); ok {
				return n
			}
		}
	}
	// Prefer pc.HTTPClient (it threads the project's retryable transport
	// + standard UA) when present. Otherwise fall back to the bare
	// http.DefaultClient.
	var client httpDoer
	if pc != nil && pc.HTTPClient != nil {
		client = pc.HTTPClient.HTTPClient()
	} else {
		client = http.DefaultClient
	}
	return NewHTTPNavigator(client, "")
}

func parseInputs(in map[string]any) topicsInputs {
	out := topicsInputs{
		mode:  "icons",
		limit: 15,
		sort:  "name",
	}
	if v, ok := in["plugin_topics_mode"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.mode = s
		}
	}
	if v, ok := pluginutil.ReadInt(in, "plugin_topics_limit"); ok {
		out.limit = v
	}
	if v, ok := in["plugin_topics_sort"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.sort = s
		}
	}
	return out
}
