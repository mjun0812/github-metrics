package engine

import "testing"

// TestSinglePluginInputs_ForcesChromeOff anchors the invariant that
// per-plugin SVG output never leaks chrome (#640 / PR #641 cap-1
// review SHOULD-FIX #5). Each of the six chrome_<section> booleans
// MUST be present in the returned inputs map and set to false, so the
// chrome.AnyChromeInputPresent short-circuit fires and overrides any
// caller-provided chrome_*=yes (the per-plugin showcase is always
// chrome-free regardless of what the caller passes).
func TestSinglePluginInputs_ForcesChromeOff(t *testing.T) {
	t.Parallel()
	// Hostile fixture: caller asks for every chrome surface to be on
	// AND passes the legacy CSV. singlePluginInputs must override.
	hostile := map[string]any{
		"base":                     "header,activity,community,repositories,metadata,introduction",
		"chrome_header":            true,
		"chrome_activity":          true,
		"chrome_community":         true,
		"chrome_repositories":      true,
		"chrome_metadata":          true,
		"chrome_introduction":      true,
		"plugin_base_activity":     true,
		"plugin_base_repositories": true,
		// the actual plugin gate we're showcasing
		"plugin_stars": true,
	}
	out := singlePluginInputs(hostile, "stars")

	for _, key := range []string{
		"chrome_header", "chrome_activity", "chrome_community",
		"chrome_repositories", "chrome_metadata", "chrome_introduction",
	} {
		v, ok := out[key]
		if !ok {
			t.Errorf("chrome key %q missing from per-plugin inputs; AnyChromeInputPresent short-circuit will not fire", key)
			continue
		}
		if b, isBool := v.(bool); !isBool || b {
			t.Errorf("chrome key %q must be false in per-plugin inputs, got %v", key, v)
		}
	}

	// Legacy v2 keys must also be stripped from the per-plugin inputs
	// map (safety-net strip retained in v3.0 — #649) so stale CI
	// configs cannot smuggle unknown garbage into the per-plugin SVG.
	if _, ok := out["base"]; ok {
		t.Errorf("legacy `base` input must be stripped from per-plugin inputs; got %v", out["base"])
	}
	if _, ok := out["plugin_base_activity"]; ok {
		t.Errorf("legacy `plugin_base_activity` input must be stripped from per-plugin inputs")
	}
	if _, ok := out["plugin_base_repositories"]; ok {
		t.Errorf("legacy `plugin_base_repositories` input must be stripped from per-plugin inputs")
	}

	// The showcased plugin's own gate must survive unmolested.
	if v, ok := out["plugin_stars"]; !ok || v == false {
		t.Errorf("plugin_stars gate must be preserved; got %v", v)
	}
}
