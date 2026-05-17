package main

import (
	"strings"
	"testing"
)

// TestGenerate_HasRequiredSections confirms the generator emits the
// four mandatory action.yml top-level keys.
func TestGenerate_HasRequiredSections(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		"name: 'github-metrics'",
		"inputs:",
		"outputs:",
		"runs:",
		"using: 'docker'",
		// Build from Dockerfile at the consumer-pinned ref — see the
		// gen-action-yml runs footer comment for the supply-chain
		// rationale. Reverting to ":latest" reintroduces the
		// REQUEST_CHANGES finding from the PR #266 self-review.
		"image: 'Dockerfile'",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("generated action.yml missing %q", want)
		}
	}
}

// TestGenerate_CoreInputsPresent confirms the core inputs (token,
// user, committer_*, filename) land in action.yml.
func TestGenerate_CoreInputsPresent(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, key := range []string{
		"\n  token:\n",
		"\n  user:\n",
		"\n  repo:\n", // M7 — top-level repo input (already shipped by core metadata; locked here)
		"\n  committer_branch:\n",
		"\n  filename:\n",
	} {
		if !strings.Contains(body, key) {
			t.Errorf("generated action.yml missing core input %q", key)
		}
	}
}

// TestGenerate_AdoptedPluginGatesPresent confirms each採用 plugin's
// `plugin_<slug>` enable gate appears.
func TestGenerate_AdoptedPluginGatesPresent(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	adoptedGates := []string{
		"plugin_languages", "plugin_activity", "plugin_achievements",
		"plugin_repositories", "plugin_isocalendar",
		"plugin_calendar", "plugin_habits", "plugin_stars", "plugin_people",
		"plugin_notable", "plugin_contributors", "plugin_reactions",
		"plugin_projects", "plugin_sponsors", "plugin_sponsorships",
		"plugin_stargazers", "plugin_traffic",
		"plugin_topics", "plugin_starlists",
	}
	for _, gate := range adoptedGates {
		if !strings.Contains(body, "\n  "+gate+":\n") {
			t.Errorf("generated action.yml missing adopted gate %q", gate)
		}
	}
}

// TestGenerate_NoUnadoptedPluginSlug enforces constitution 原則 III.
func TestGenerate_NoUnadoptedPluginSlug(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	unadopted := []string{
		"plugin_code", "plugin_discussions", "plugin_followup", "plugin_gists",
		"plugin_introduction", "plugin_licenses", "plugin_lines", "plugin_skyline",
		"plugin_support", "plugin_anilist", "plugin_leetcode", "plugin_music",
		"plugin_pagespeed", "plugin_posts", "plugin_rss", "plugin_stackoverflow",
		"plugin_steam", "plugin_tweets", "plugin_wakatime",
	}
	for _, slug := range unadopted {
		if strings.Contains(body, slug) {
			t.Errorf("generated action.yml contains unadopted slug %q (constitution 原則 III)", slug)
		}
	}
}

// TestGenerate_Deterministic confirms two consecutive runs produce
// byte-identical output (drift gate prerequisite).
func TestGenerate_Deterministic(t *testing.T) {
	t.Parallel()
	a, err := generate("../../../assets")
	if err != nil {
		t.Fatal(err)
	}
	b, err := generate("../../../assets")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("two runs produced different output (non-deterministic)")
	}
}
