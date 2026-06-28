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
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps(), nil)
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

// TestComputePerPlugin_AllowlistFilters verifies that an explicit allowlist
// limits the output to only the listed plugins.
func TestComputePerPlugin_AllowlistFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	req := engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		// All three plugins enabled in inputs...
		Inputs: baseInputsForPerPlugin(),
	}
	// ...but allowlist restricts to only two.
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps(), []string{"header", "stars"})
	if err != nil {
		t.Fatalf("ComputePerPlugin: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2 (allowlist restricted)", len(results))
	}
	for _, pr := range results {
		if pr.Plugin != "header" && pr.Plugin != "stars" {
			t.Errorf("unexpected plugin %q in results", pr.Plugin)
		}
	}
}

// TestComputePerPlugin_StarsAllowlistOnly verifies the single-plugin case
// works (renderer produces non-empty SVG output for the listed plugin).
func TestComputePerPlugin_StarsAllowlistOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	req := engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs: map[string]any{
			"user":            "octocat",
			"use_mocked_data": true,
			"optimize":        []string{"css", "xml"},
			"plugin_stars":    true,
		},
	}
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps(), []string{"stars"})
	if err != nil {
		t.Fatalf("ComputePerPlugin: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("stars plugin: unexpected error: %v", results[0].Error)
	}
	if len(results[0].Output) == 0 {
		t.Errorf("stars plugin: empty output")
	}
}

// TestComputePerPlugin_EmptyAllowlist checks that an empty allowlist
// renders all truthy plugin gates (and excludes false ones).
func TestComputePerPlugin_EmptyAllowlist(t *testing.T) {
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
	results, err := engine.ComputePerPlugin(ctx, req, newPerPluginTestDeps(), nil)
	if err != nil {
		t.Fatalf("ComputePerPlugin: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results (header+languages), got %d", len(results))
	}
}
