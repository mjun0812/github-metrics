// Package compliance_test enforces the constitution-level rules that
// don't fit naturally into a unit test:
//
//   - Principle III (scope discipline): no unadopted plugin/template
//     name leaks into internal/ or cmd/ Go sources.
//   - Development workflow rule: no `// removed:` style sentinel
//     comments (constitution `CLAUDE.md` policy on backwards-compat
//     hacks).
//
// The tests scan the working tree, not git history; the dual
// constraint that ./org_repo never enters git history is enforced by
// .gitignore + the project constitution and verified manually by the
// release procedure.
package compliance_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// unadoptedPluginNames is the source of truth for "plugin slugs we
// deliberately do NOT implement in this MVP". The list mirrors
// docs/design/15-selection-answer.md §7 (Backlog).
//
// Each entry MUST be matched as a whole word; substring matching
// would false-flag legitimate strings like "code" inside "codebase".
var unadoptedPluginNames = []string{
	"wakatime", "pagespeed", "posts", "rss", "stackoverflow",
	"leetcode", "anilist", "music", "steam", "tweets",
	"crypto", "nightscout", "stock", "chess", "splatoon",
	"fortune", "poopmap", "screenshot", "16personalities",
	"lines", "gists", "followup", "discussions", "skyline", "support",
}

// scanRoots is the set of directory prefixes to walk. We focus on
// production Go code; tests and docs are allowed to mention
// unadopted plugin names freely (e.g. docs may document the
// full backlog).
var scanRoots = []string{
	"cmd",
	"internal/action",
	"internal/engine",
	"internal/plugins",
	"internal/render",
	"internal/templates",
}

// allowedFiles are paths that are permitted to mention unadopted
// plugin names (e.g. this very test file, or shared test helpers).
// Two false-positive shapes drive the M3 additions:
//
//   - `crypto/md5` import line in svg_hash.go matches the "crypto"
//     unadopted plugin slug.
//   - `CaptureScreenshot` references in svg_resize.go
//     match the "screenshot" unadopted plugin slug.
//
// The matches are word-boundary regex hits in import paths /
// browser API names that have nothing to do with the unadopted
// upstream plugins.
var allowedFiles = map[string]struct{}{
	"tests/compliance/compliance_test.go": {},
	"internal/render/svg_hash.go":         {},
	"internal/render/svg_resize.go":       {},
	// reactions plugin exposes `Discussions int json:"discussions"` as
	// part of the upstream data.plugins.reactions JSON shape
	// (constitution 原則 II output contract). The
	// word "discussions" overlaps the unadopted upstream "discussions"
	// plugin name; this allow-list entry preserves the JSON-shape
	// requirement.
	"internal/plugins/reactions/reactions.go": {},
	"internal/plugins/reactions/partial.go":   {},
	// M6 output_action registry includes the literal "support" inside
	// the migration message ("if Gist support is critical..."), which
	// false-matches the unadopted "support" plugin slug. The reference
	// is documentation/error-message English text, not plugin code.
	"internal/action/output_action.go": {},
	// M6 outputs.go imports crypto/rand for heredoc delimiter
	// uniqueness; the "crypto" substring is a standard-library import,
	// not the unadopted "crypto" plugin slug.
	"internal/action/outputs.go": {},
	// 011 v2 languages partial emits "N lines" as part of the upstream
	// `details: lines` column (EJS line 63). The word "lines" overlaps
	// the unadopted upstream "lines" plugin name; this allow-list entry
	// preserves the upstream-equivalent column rendering.
	"internal/plugins/languages/partial.go": {},
	// languages.go filters the "lines" detail column out when indepth
	// is not enabled (upstream index.mjs:33-34). Same false-positive
	// shape as partial.go above — "lines" is the upstream details
	// column name, not the unadopted upstream plugin slug.
	"internal/plugins/languages/languages.go": {},
	// languages.indepth now computes the same details column from cloned
	// repository contents, so it shares the false-positive shape above.
	"internal/plugins/languages/indepth.go": {},
	// activity plugin exposes PR diff stats as
	// `Lines *EventLines json:"lines"` mirroring upstream
	// data.plugins.activity.events[].lines (#465). The word "lines"
	// overlaps the unadopted upstream "lines" plugin name; this
	// allow-list entry preserves the upstream JSON shape and the
	// "N files changed ++A --D" render (activity.ejs line 79).
	"internal/plugins/activity/activity.go": {},
	"internal/plugins/activity/partial.go":  {},
	// achievements: 18-upstream-achievements rewrite reads
	// `User.Gists` / `User.DiscussionsStarted` / `User.DiscussionsComments`
	// / `User.DiscussionAnswers` off the always-fetched user query to
	// back the Gister / Chatter / Helper badges. The word "gists" /
	// "discussions" overlap the unadopted standalone plugin slugs;
	// this allow-list entry preserves the upstream-equivalent badge
	// shape without standing up those plugins.
	"internal/plugins/achievements/achievements.go": {},
	"internal/plugins/data.go":                      {},
}

// TestNoUnadoptedPluginReference walks scanRoots and asserts that no
// .go file mentions an unadopted plugin slug. This is the principle
// III gate.
func TestNoUnadoptedPluginReference(t *testing.T) {
	root := mustRepoRoot(t)

	patterns := make([]*regexp.Regexp, 0, len(unadoptedPluginNames))
	for _, name := range unadoptedPluginNames {
		// Whole-word, case-insensitive. Mostly to catch identifiers
		// like wakatimePlugin, AnilistRun, etc.
		patterns = append(patterns, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(name)+`\b`))
	}

	var violations []string
	for _, dir := range scanRoots {
		full := filepath.Join(root, dir)
		err := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			// _test.go files are documentation + scaffolding. The
			// constitution III gate targets production code; tests
			// reference variable names like `lines` / `events` that
			// coincide with unadopted plugin slugs without intent.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if _, ok := allowedFiles[rel]; ok {
				return nil
			}
			body, err := os.ReadFile(path) //nolint:gosec // path is filepath.WalkDir-controlled.
			if err != nil {
				return err
			}
			lines := bytes.Split(body, []byte("\n"))
			for i, line := range lines {
				// Skip comment-only lines so the doc strings in the
				// base / engine packages that legitimately reference
				// "backlog plugin X" stay legal.
				trimmed := bytes.TrimSpace(line)
				if bytes.HasPrefix(trimmed, []byte("//")) {
					continue
				}
				for j, p := range patterns {
					if p.Match(line) {
						violations = append(violations,
							fmt.Sprintf("%s:%d references unadopted plugin %q",
								rel, i+1, unadoptedPluginNames[j]))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", full, err)
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("constitution principle III: %s", v)
		}
	}
}

// TestNoRemovedSentinelComments asserts that nobody left a
// `// removed: ...` style placeholder in the tree. The constitution
// CLAUDE.md guidance forbids "removed:" / "TODO(removed)" markers
// that keep dead context alive past their usefulness.
func TestNoRemovedSentinelComments(t *testing.T) {
	root := mustRepoRoot(t)
	pattern := regexp.MustCompile(`(?i)//\s*(removed|TODO\(removed)`)

	var hits []string
	for _, dir := range []string{"cmd", "internal"} {
		full := filepath.Join(root, dir)
		err := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			body, err := os.ReadFile(path) //nolint:gosec // path is filepath.WalkDir-controlled.
			if err != nil {
				return err
			}
			for i, line := range bytes.Split(body, []byte("\n")) {
				if pattern.Match(line) {
					rel, _ := filepath.Rel(root, path)
					hits = append(hits,
						fmt.Sprintf("%s:%d contains a 'removed' sentinel comment", rel, i+1))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", full, err)
		}
	}
	if len(hits) > 0 {
		for _, h := range hits {
			t.Errorf("constitution Development Workflow: %s", h)
		}
	}
}

// adoptedM4Plugins is the canonical 21-plugin list M4 ships. Each
// entry corresponds to a subdirectory under internal/plugins/ that
// MUST exist after M4. Two entries — `languages.recent` and
// `languages.indepth` — are sub-modes of the `languages` plugin and
// share its directory; they don't map to their own subdirectories.
var adoptedM4Plugins = []string{
	// P1 MVP (5)
	"languages", "activity", "achievements", "repositories", "isocalendar",
	// P2 GraphQL/REST (11)
	"calendar", "habits", "stars", "people", "notable",
	"contributors", "reactions", "sponsors", "sponsorships",
	"stargazers", "traffic",
	// P3 heavy (2 with own directories; the recent/indepth
	// sub-modes live in internal/plugins/languages/)
	"topics", "starlists",
	// #602 header extraction (identity card lifted out of base into
	// its own opt-in plugin so per-plugin SVG embed flows can include
	// or exclude it independently).
	"header",
	// #625 base re-introduction: the activity+community / repositories
	// summary panels that #623 deleted were restored as an opt-in
	// plugin reading exclusively from the dataprovider.Provider layer.
	// It ships its own internal/plugins/base/ directory and counts
	// toward the adopted dir set, but it is structurally foundational
	// (no standalone card) so it stays out of the docs/plugins gallery
	// gate above (see TestCompliance_DocsPluginPagesMatchAdoptedSet).
	"base",
}

// nonPluginInternalDirs are internal/plugins/ children that are
// infrastructure, not plugin slugs. They MUST be excluded from the
// adopted-set comparison.
var nonPluginInternalDirs = map[string]struct{}{
	"core":            {}, // core plugin (settings + parallel runner)
	"pluginutil":      {}, // shared input-parsing / formatting helpers; not a user-facing plugin slug
	"requirestesting": {}, // #604 Requires() drift-detection test helpers; not a plugin slug
}

// TestCompliance_M4_AdoptedPlugins (T096 / SC-007) scans
// internal/plugins/ for subdirectories and asserts the directory set
// exactly matches the 21 採用 plugin list (less the base/core
// infrastructure plugins). Any extra directory is treated as an
// unadopted plugin landing in production — a constitution 原則 III
// violation. Any missing directory means an adopted plugin was lost.
func TestCompliance_M4_AdoptedPlugins(t *testing.T) {
	root := mustRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal", "plugins"))
	if err != nil {
		t.Fatalf("read internal/plugins/: %v", err)
	}

	have := map[string]struct{}{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, skip := nonPluginInternalDirs[name]; skip {
			continue
		}
		have[name] = struct{}{}
	}

	want := map[string]struct{}{}
	for _, slug := range adoptedM4Plugins {
		// languages.recent / languages.indepth share the languages dir.
		if strings.Contains(slug, ".") {
			continue
		}
		want[slug] = struct{}{}
	}

	missing := []string{}
	for slug := range want {
		if _, ok := have[slug]; !ok {
			missing = append(missing, slug)
		}
	}
	extra := []string{}
	for dir := range have {
		if _, ok := want[dir]; !ok {
			extra = append(extra, dir)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("constitution 原則 III (採用 21): missing adopted plugin dirs: %v", missing)
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Errorf("constitution 原則 III (採用 21): unadopted plugin dirs landed in internal/plugins/: %v", extra)
	}
	if len(missing) == 0 && len(extra) == 0 {
		t.Logf("M4 採用 21 plugin compliance OK (dirs: %d adopted + core)", len(have))
	}
}

// TestCompliance_DocsPluginPagesMatchAdoptedSet asserts that the set
// of `docs/plugins/*.md` pages matches the 19 user-facing adopted
// plugins. The infrastructure plugins `base` / `core` are foundational
// components: they ship their own doc page describing their inputs
// (see `internal/tools/gen-plugin-docs/main.go::foundationalSlugs`),
// but they are NOT part of the adopted-19 gating set — the README
// gallery is reserved for plugins that emit a user-visible card.
//
// The doc pages are generated by `make docs`; this test guards
// against drift between the generator and the compliance plugin list.
func TestCompliance_DocsPluginPagesMatchAdoptedSet(t *testing.T) {
	root := mustRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plugins"))
	if err != nil {
		t.Fatalf("read docs/plugins/: %v", err)
	}

	have := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		have[slug] = struct{}{}
	}

	want := map[string]struct{}{}
	for _, slug := range adoptedM4Plugins {
		if strings.Contains(slug, ".") {
			// languages.recent / languages.indepth share the languages page.
			continue
		}
		if _, skip := nonPluginInternalDirs[slug]; skip {
			continue
		}
		want[slug] = struct{}{}
	}

	// Foundational plugin pages are tolerated (they are documented in
	// docs/plugins/ for completeness even though they are not part of
	// the adopted-19 gallery). We drop them from `have` so the
	// strict-equality check below targets only the user-facing 19.
	for slug := range nonPluginInternalDirs {
		delete(have, slug)
	}

	var missing, extra []string
	for s := range want {
		if _, ok := have[s]; !ok {
			missing = append(missing, s)
		}
	}
	for s := range have {
		if _, ok := want[s]; !ok {
			extra = append(extra, s)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("missing docs/plugins/<slug>.md pages: %v (run `make docs`)", missing)
	}
	if len(extra) > 0 {
		t.Errorf("unexpected docs/plugins/<slug>.md pages (not in adopted set): %v", extra)
	}
}

// TestCompliance_M9_TestInfraInvariant asserts that
// `internal/testutil/` contains exactly the documented sub-packages
// (mocks, golden). Guards constitution III erosion via testutil
// growth — any new subpackage requires an explicit M9 amendment.
func TestCompliance_M9_TestInfraInvariant(t *testing.T) {
	root := mustRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal", "testutil"))
	if err != nil {
		t.Fatalf("read internal/testutil/: %v", err)
	}
	want := map[string]struct{}{
		"mocks":  {},
		"golden": {},
		// svgcontent: content-level SVG extraction helper backing the
		// tests/content/ verification suite (semantic / query-field /
		// recorded-fixture layers behind issues #463–#472). Added as
		// an explicit M9 amendment — it carries no production code.
		"svgcontent": {},
	}
	have := map[string]struct{}{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		have[e.Name()] = struct{}{}
	}
	for name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("adopted testutil sub-package missing: internal/testutil/%s/", name)
		}
	}
	for name := range have {
		if _, ok := want[name]; !ok {
			t.Errorf("unadopted testutil sub-package landed: internal/testutil/%s/ — constitution III violation", name)
		}
	}
}

// TestCompliance_M7_NonAffectedPluginsAreInvariantOnRepo verifies the
// repo-mode contract: only the 6 listed
// plugins (activity, contributors, languages, people,
// sponsors, stargazers) gain a Mode field in JSON output. The other
// adopted plugins MUST remain untouched. We inspect the plugin
// package source rather than running them to stay hermetic.
func TestCompliance_M7_NonAffectedPluginsAreInvariantOnRepo(t *testing.T) {
	root := mustRepoRoot(t)
	affected := map[string]struct{}{
		"activity":     {},
		"contributors": {},
		"languages":    {},
		"people":       {},
		"sponsors":     {},
		"stargazers":   {},
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "plugins"))
	if err != nil {
		t.Fatalf("read plugins dir: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		if _, skip := nonPluginInternalDirs[slug]; skip {
			continue
		}
		// languages.recent / languages.indepth live under languages/.
		if _, isAffected := affected[slug]; isAffected {
			continue
		}
		// Check the main plugin file for an AggregationMode reference,
		// which would indicate the plugin grew a repo-mode branch.
		// Topics has a pre-existing Mode field of its own that's
		// unrelated to M7 (icons vs spdx display).
		if slug == "topics" {
			continue
		}
		mainFile := filepath.Join(root, "internal", "plugins", slug, slug+".go")
		body, err := os.ReadFile(mainFile)
		if err != nil {
			continue
		}
		if strings.Contains(string(body), "plugins.AggregationMode") {
			t.Errorf("plugin %s: unexpected AggregationMode usage; M7 contract limits repo-mode to the 7 enumerated plugins", slug)
		}
	}
}

// nonTemplateInternalDirs lists subdirectories of
// `internal/templates/` that are NOT template implementations — they
// are shared helpers consumed by the real templates. They MUST be
// excluded from the adopted-template comparison.
var nonTemplateInternalDirs = map[string]struct{}{
	"chrome": {}, // shared footer / base-section / styles-loader helpers used by classic + repository
}

// TestCompliance_M7_TemplateInvariant asserts that `internal/templates/`
// hosts exactly the adopted templates (classic from M2, repository
// from M7) and nothing else. Adding `markdown`/`terminal`/etc. would
// silently violate the M5/M8 skipped-scope rule from
// docs/design/15-selection-answer.md.
func TestCompliance_M7_TemplateInvariant(t *testing.T) {
	root := mustRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal", "templates"))
	if err != nil {
		t.Fatalf("read internal/templates/: %v", err)
	}
	want := map[string]struct{}{
		"classic":    {},
		"repository": {},
	}
	have := map[string]struct{}{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, skip := nonTemplateInternalDirs[name]; skip {
			continue
		}
		have[name] = struct{}{}
	}
	for name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("adopted template missing: internal/templates/%s/", name)
		}
	}
	for name := range have {
		if _, ok := want[name]; !ok {
			t.Errorf("unadopted template landed: internal/templates/%s/ — constitution III violation", name)
		}
	}
}

// TestCompliance_M6_NoNewPlugins asserts the M6 invariant that the
// Action / CLI surface code (internal/action/, cmd/metrics-cli/)
// does NOT introduce new plugin or template subdirectories. M6 is a
// delivery layer — it wires existing M1-M4 components together. New
// adopted slugs must come through a separate spec to avoid silently
// landing unadopted plugins under the polish phase.
func TestCompliance_M6_NoNewPlugins(t *testing.T) {
	root := mustRepoRoot(t)
	for _, rel := range []string{
		filepath.Join("internal", "action"),
		filepath.Join("cmd", "metrics-cli"),
	} {
		path := filepath.Join(root, rel)
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			// `testdata/` is an std-go convention for test fixtures.
			if name == "testdata" {
				continue
			}
			t.Errorf("M6 constraint violated: %s/%s/ — Action surface must not host plugin/template subpackages", rel, name)
		}
	}
}

// TestCompliance_M10_PluginTemplateInvariant re-asserts the M1-M9
// adopted-feature set is unchanged after M10. M10 is a delivery
// pipeline finalization phase — constitution principle III says no
// plugin / template additions land here. This test consolidates the
// per-phase M4 / M7 invariants into one explicit M10 epoch gate so a
// silent regression (e.g. someone copies an M8 plugin into the tree
// alongside a Dockerfile change) is surfaced as an M10 failure.
func TestCompliance_M10_PluginTemplateInvariant(t *testing.T) {
	root := mustRepoRoot(t)

	// 21 adopted plugins (mirrors adoptedM4Plugins, with the two
	// language sub-modes collapsed into the languages directory).
	wantPlugins := map[string]struct{}{}
	for _, slug := range adoptedM4Plugins {
		if strings.Contains(slug, ".") {
			continue
		}
		wantPlugins[slug] = struct{}{}
	}

	havePlugins := map[string]struct{}{}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "plugins"))
	if err != nil {
		t.Fatalf("read internal/plugins/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, skip := nonPluginInternalDirs[name]; skip {
			continue
		}
		havePlugins[name] = struct{}{}
	}

	wantTemplates := map[string]struct{}{
		"classic":    {},
		"repository": {},
	}
	haveTemplates := map[string]struct{}{}
	entries, err = os.ReadDir(filepath.Join(root, "internal", "templates"))
	if err != nil {
		t.Fatalf("read internal/templates/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, skip := nonTemplateInternalDirs[name]; skip {
			continue
		}
		haveTemplates[name] = struct{}{}
	}

	diff := func(label string, want, have map[string]struct{}) {
		t.Helper()
		var missing, extra []string
		for k := range want {
			if _, ok := have[k]; !ok {
				missing = append(missing, k)
			}
		}
		for k := range have {
			if _, ok := want[k]; !ok {
				extra = append(extra, k)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("M10 invariant: %s missing after M10: %v", label, missing)
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			t.Errorf("M10 invariant: %s extras landed after M10 (constitution III): %v", label, extra)
		}
	}
	diff("plugin dirs", wantPlugins, havePlugins)
	diff("template dirs", wantTemplates, haveTemplates)
}

// TestOrgRepoIgnored asserts the constitution rule that ./org_repo
// MUST stay out of git history. We check .gitignore declaratively.
func TestOrgRepoIgnored(t *testing.T) {
	root := mustRepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore must exist: %v", err)
	}
	if !strings.Contains(string(body), "org_repo/") {
		t.Fatalf(".gitignore is missing the 'org_repo/' entry; ./org_repo would leak into git history")
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
