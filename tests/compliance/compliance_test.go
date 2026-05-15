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
// production Go code; tests / specs / docs are allowed to mention
// unadopted plugin names freely (e.g. spec.md, tasks.md document the
// full backlog).
var scanRoots = []string{
	"cmd",
	"internal/plugins",
	"internal/templates",
}

// allowedFiles are paths that are permitted to mention unadopted
// plugin names (e.g. this very test file, or shared test helpers).
var allowedFiles = map[string]struct{}{
	"tests/compliance/compliance_test.go": {},
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
