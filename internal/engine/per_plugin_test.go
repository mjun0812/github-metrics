package engine_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/render"

	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/plugins/header"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// newPerPluginTestDeps returns test-friendly engine.Deps (fake renderer,
// no REST/GraphQL — per-plugin tests use use_mocked_data).
func newPerPluginTestDeps() engine.Deps {
	return engine.Deps{
		Settings: &config.Settings{Repositories: 100},
		Render:   render.NewFakeRenderer(),
	}
}

func baseInputsForPerPlugin() map[string]any {
	return map[string]any{
		"user":             "octocat",
		"use_mocked_data":  true,
		"optimize":         []string{"css", "xml"},
		"plugin_header":    true,
		"plugin_languages": true,
		"plugin_stars":     true,
	}
}

// TestComputePerPlugin_ThreePlugins verifies that per-plugin mode
// produces one SVG file per enabled plugin.
func TestComputePerPlugin_ThreePlugins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	req := engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   baseInputsForPerPlugin(),
	}
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps())
	if err != nil {
		t.Fatalf("ComputePerPlugin: %v", err)
	}
	// Should produce 3 results (one per enabled plugin).
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	slugs := make(map[string]bool)
	for _, pr := range results {
		if pr.Error != nil {
			t.Errorf("plugin %q: unexpected error: %v", pr.Plugin, pr.Error)
		}
		if len(pr.Output) == 0 {
			t.Errorf("plugin %q: empty output", pr.Plugin)
		}
		slugs[pr.Plugin] = true
	}
	for _, want := range []string{"header", "languages", "stars"} {
		if !slugs[want] {
			t.Errorf("missing result for plugin %q", want)
		}
	}
}

// TestComputePerPlugin_HonorsTruthyGates checks that only truthy
// `plugin_<slug>=true` gates produce SVG output; explicit `false`
// gates are excluded, matching the post-#654 single-resolution path.
func TestComputePerPlugin_HonorsTruthyGates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	req := engine.Request{
		Login:  "octocat",
		Format: "svg",
		Inputs: map[string]any{
			"user":             "octocat",
			"use_mocked_data":  true,
			"optimize":         []string{"css", "xml"},
			"plugin_header":    true,
			"plugin_languages": true,
			"plugin_stars":     false,
		},
	}
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps())
	if err != nil {
		t.Fatalf("ComputePerPlugin: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results (header+languages), got %d", len(results))
	}
	for _, pr := range results {
		if pr.Plugin == "stars" {
			t.Errorf("stars=false should have been excluded; got result %+v", pr)
		}
	}
}
