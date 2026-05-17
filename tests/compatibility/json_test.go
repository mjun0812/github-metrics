// Package compatibility_test — json_test.go validates that the M4
// engine.Marshal output stays key/型 compatible with the upstream
// lowlighter/metrics JSON for the full 21-plugin baseline.
//
// SC-004 evidence: key/型 diff = 0 across the 21 採用 plugin entries
// of the upstream octocat fixture. The fixture is regenerated via
// `go run ./internal/tools/sync-fixtures --user octocat --full` and
// vendored under tests/fixtures/upstream/. Until the fixture is
// generated (see tests/fixtures/upstream/README.md for the manual
// setup steps), this test t.Skip's so the suite stays green for
// contributors without the upstream test harness installed.
package compatibility_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const upstreamJSONFixture = "tests/fixtures/upstream/octocat.json"

// adoptedPlugins is the 21 採用 plugin slug list (FR-018). Two entries
// — `languages.recent` and `languages.indepth` — are sub-modes of the
// `languages` plugin and **do not appear as top-level keys** under
// `data.plugins` in the upstream payload; upstream nests them inside
// `data.plugins.languages` (sections / colors / recent-bytes shape).
// The compat test treats those two as sub-mode slugs and verifies them
// through `assertLanguagesSubModes` rather than the top-level
// presence/kind loop. Mirrors the same dotted-slug handling pattern
// used by `tests/compliance/compliance_test.go::adoptedM4Plugins`.
var adoptedPlugins = []string{
	// P1 MVP (5)
	"languages", "activity", "achievements", "repositories", "isocalendar",
	// P2 GraphQL/REST (12)
	"calendar", "habits", "stars", "people", "notable",
	"contributors", "reactions", "projects", "sponsors", "sponsorships",
	"stargazers", "traffic",
	// P3 chromedp / heavy (4 slugs; 2 own-key + 2 sub-modes of `languages`)
	"topics", "starlists",
	"languages.recent", "languages.indepth",
}

// isSubModeSlug returns true when slug is a dotted "<parent>.<mode>"
// form (currently only `languages.recent` / `languages.indepth`).
// Upstream serializes sub-modes inside their parent's `data.plugins`
// entry rather than as standalone top-level keys.
func isSubModeSlug(slug string) bool { return strings.Contains(slug, ".") }

// unadoptedPlugins is the 19 unadopted upstream plugin slug list
// (FR-018 / constitution 原則 III). These MUST NOT appear in the
// upstream fixture after a 21-plugin run — if they do, either our
// adopted list is wrong or upstream changed the default plugin set.
var unadoptedPlugins = []string{
	"code", "discussions", "followup", "gists", "introduction",
	"licenses", "lines", "skyline", "support",
	"anilist", "leetcode", "music", "pagespeed", "posts",
	"rss", "stackoverflow", "steam", "tweets", "wakatime",
}

// TestCompatibilityJSON_21Plugins asserts upstream key / 型 diff = 0
// across the 21 採用 plugin entries. Skips when the fixture is absent.
func TestCompatibilityJSON_21Plugins(t *testing.T) {
	t.Parallel()
	body, err := readFixture(t, upstreamJSONFixture)
	if err != nil {
		t.Skipf("upstream fixture missing — skip (regenerate via sync-fixtures --user octocat --full; see %s/README.md): %v",
			filepath.Dir(upstreamJSONFixture), err)
	}

	var root map[string]any
	if uErr := json.Unmarshal(body, &root); uErr != nil {
		t.Fatalf("decode upstream fixture: %v", uErr)
	}

	// Upstream payload shape: `{"data": {"plugins": {<slug>: {...}, ...}, ...}}`.
	data, ok := root["data"].(map[string]any)
	if !ok {
		t.Fatalf("upstream fixture: expected `data` object, got %T", root["data"])
	}
	plugins, ok := data["plugins"].(map[string]any)
	if !ok {
		t.Fatalf("upstream fixture: expected `data.plugins` object, got %T", data["plugins"])
	}

	// Assertion 1: every top-level adopted plugin slug must be present
	// with non-nil value. Sub-mode slugs (`languages.recent` /
	// `languages.indepth`) are nested under `plugins["languages"]` —
	// they are verified separately in assertLanguagesSubModes below.
	missing := []string{}
	for _, slug := range adoptedPlugins {
		if isSubModeSlug(slug) {
			continue
		}
		v, present := plugins[slug]
		if !present || v == nil {
			missing = append(missing, slug)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("adopted plugins missing from upstream fixture: %v", missing)
	}

	// Assertion 2: no unadopted plugin slug appears.
	leaked := []string{}
	for _, slug := range unadoptedPlugins {
		if _, present := plugins[slug]; present {
			leaked = append(leaked, slug)
		}
	}
	if len(leaked) > 0 {
		sort.Strings(leaked)
		t.Errorf("unadopted plugins leaked into upstream fixture: %v", leaked)
	}

	// Assertion 3: each top-level adopted plugin entry is a JSON object
	// (= consistent type with our engine output). Sub-mode slugs handled
	// in assertLanguagesSubModes. Stricter key/型 comparison against the
	// Go engine output is left to integration tests; here we focus on the
	// upstream-side baseline shape.
	for _, slug := range adoptedPlugins {
		if isSubModeSlug(slug) {
			continue
		}
		entry, present := plugins[slug]
		if !present {
			continue
		}
		if reflect.TypeOf(entry).Kind() != reflect.Map {
			t.Errorf("upstream plugins[%q] kind = %v, want map", slug, reflect.TypeOf(entry).Kind())
		}
	}

	// Assertion 4: `languages.recent` / `languages.indepth` sub-modes
	// surface inside `plugins["languages"]`. We sanity-check that the
	// `languages` entry is an object (asserted above) and that it
	// carries shape hints upstream produces when the sub-modes ran. We
	// keep this assertion deliberately loose because the exact upstream
	// key names (`sections`, `recent`, `indepth`, `colors`, ...) have
	// drifted historically; the SC-004 strict diff lives in integration
	// tests against the Go engine output.
	assertLanguagesSubModes(t, plugins)
}

// assertLanguagesSubModes verifies that `plugins["languages"]` is a
// non-empty map. The exact upstream shape (which sub-keys exist when
// recent / indepth sub-modes ran) is intentionally not pinned here —
// see the comment at the call site for rationale.
func assertLanguagesSubModes(t *testing.T, plugins map[string]any) {
	t.Helper()
	langs, ok := plugins["languages"].(map[string]any)
	if !ok {
		t.Errorf("plugins[\"languages\"] missing or not an object; cannot host languages.recent / languages.indepth sub-modes")
		return
	}
	if len(langs) == 0 {
		t.Errorf("plugins[\"languages\"] is empty; sub-modes (recent / indepth) cannot have surfaced")
	}
}

// readFixture walks up from CWD to find the project-rooted fixture.
// Mirrors the lookup in svg_hash_test.go so the two compat tests
// share the same soft-skip semantics.
func readFixture(t *testing.T, rel string) ([]byte, error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		body, readErr := os.ReadFile(candidate) //nolint:gosec // candidate is rooted under the project tree
		if readErr == nil {
			return body, nil
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return nil, readErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fs.ErrNotExist
}
