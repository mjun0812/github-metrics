// Package topics owns the M4 "topics" plugin. It scrapes the user's
// starred-topics page via chromedp because the GitHub API does not
// expose the dataset directly.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p3-heavy.md §3
// Data model: specs/004-m4-github-plugins/data-model.md E-023
//
// The runtime keeps the chromedp surface behind a small Navigator
// interface so non-chromedp tests can exercise the skipped/short-circuit
// paths without launching chromium. The chromedp-tagged tests substitute
// the production Navigator backed by *render.Browser.
package topics

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
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

// Navigator abstracts the chromedp-backed page scrape so non-chromedp
// tests can inject a fake. Production code uses browserNavigator backed
// by *render.Browser.
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

	// Gate: the user must enable the plugin via `plugin_topics`. Without
	// this gate the plugin would always run and pollute Result.Errors
	// with "chromedp not available" entries even when topics was never
	// requested.
	if !truthy(pc.Inputs["plugin_topics"]) {
		return &Result{
			Skipped:       true,
			SkippedReason: "plugin_topics not enabled",
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	if !extrasEnabled(pc.Inputs, "extras.metrics.run.puppeteer.scrapping") {
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
	if nav == nil {
		// Record a *RetryableError on Data.Errors so the engine surfaces
		// the missing-chromedp degraded path on Result.Errors per
		// contract §3.4 step 2.
		pc.Data.AppendError(xerrors.NewRetryableError(
			fmt.Errorf("topics: chromedp not available"),
		))
		return &Result{
			Skipped:       true,
			SkippedReason: "chromedp not available",
			List:          []Topic{},
			Mode:          in.mode,
			Limit:         in.limit,
			Sort:          in.sort,
		}, nil
	}

	login := loginFromInputs(pc.Inputs)
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
		// chromedp surface errors (timeouts, context cancellation) are
		// always transient — wrap as *RetryableError so the engine can
		// retry or surface them on Result.Errors per contract §3.6.
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
// Otherwise it returns a browserNavigator when pc.Render is a real
// *render.Browser; nil when it is nil or a *FakeRenderer.
func pickNavigator(pc *plugins.PluginContext) Navigator {
	if pc.Inputs != nil {
		if v, ok := pc.Inputs[NavigatorKey]; ok {
			if n, ok := v.(Navigator); ok {
				return n
			}
		}
	}
	browser, ok := pc.Render.(*render.Browser)
	if !ok || browser == nil {
		return nil
	}
	return &browserNavigator{browser: browser}
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
	if v, ok := readInt(in, "plugin_topics_limit"); ok {
		out.limit = v
	}
	if v, ok := in["plugin_topics_sort"]; ok {
		if s, ok := v.(string); ok && s != "" {
			out.sort = s
		}
	}
	return out
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

// extrasEnabled mirrors the languages plugin helper: absent → enabled,
// explicit false → disabled. Avoids importing the languages package
// just for this util.
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

// truthy normalizes the various input shapes into a bool. Mirrors the
// helper in the languages package so we don't introduce a cross-package
// dependency just for this util.
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
