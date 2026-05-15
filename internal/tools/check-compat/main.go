// Command check-compat diffs the input-key sets between the upstream
// lowlighter/metrics checkout (./org_repo) and the vendored copy
// under assets/. The intent is to enforce constitution principle I
// (upstream input compatibility): every input declared by an adopted
// upstream plugin or template MUST be reachable from the project's
// own metadata.yml.
//
// Exit codes:
//
//	0  no diff (compatibility holds)
//	1  diff detected (compatibility broken; the report goes to stderr)
//	2  invocation / I/O error
//
// The tool intentionally does NOT diff non-input fields like
// `description` or `examples` — those drift naturally as upstream
// updates docs and we want to keep our reports focused on the
// behavioural surface.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// adoptedPlugins lists the upstream plugin slugs we have promised to
// remain key-compatible with (docs/design/15-selection-answer.md §6.4
// + base + core).
var adoptedPlugins = []string{
	"base", "core",
	"languages", "activity", "achievements", "repositories",
	"isocalendar", "calendar", "habits", "stars", "topics", "starlists",
	"people", "notable", "contributors", "reactions", "projects",
	"sponsors", "sponsorships", "stargazers", "traffic",
}

// adoptedTemplates lists the upstream template slugs we mirror.
var adoptedTemplates = []string{"classic", "repository"}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-compat: %v\n", err)
		os.Exit(2)
	}
	upstreamPlugins := filepath.Join(repoRoot, "org_repo", "source", "plugins")
	upstreamTemplates := filepath.Join(repoRoot, "org_repo", "source", "templates")
	localPlugins := filepath.Join(repoRoot, "assets", "plugins")
	localTemplates := filepath.Join(repoRoot, "assets", "templates")

	if _, err := os.Stat(upstreamPlugins); err != nil {
		fmt.Fprintln(os.Stderr, "check-compat: ./org_repo not present; skip")
		fmt.Fprintln(os.Stderr, "             (clone lowlighter/metrics there to enable the diff)")
		// Treat absence as soft-pass: contributors without the upstream
		// checkout should not be blocked from local builds.
		os.Exit(0)
	}

	diff := compareSet("plugin", upstreamPlugins, localPlugins, adoptedPlugins)
	diff = append(diff, compareSet("template", upstreamTemplates, localTemplates, adoptedTemplates)...)

	if len(diff) == 0 {
		fmt.Println("check-compat: 0 diff across", len(adoptedPlugins), "plugins and", len(adoptedTemplates), "templates")
		return
	}
	fmt.Fprintln(os.Stderr, "check-compat: compatibility diffs detected:")
	for _, d := range diff {
		fmt.Fprintln(os.Stderr, "  ", d)
	}
	os.Exit(1)
}

// compareSet walks every adopted slug, loads both metadata.yml copies,
// and returns one human-readable diff line per missing key.
func compareSet(kind, upstreamRoot, localRoot string, adopted []string) []string {
	var diff []string
	for _, name := range adopted {
		up := filepath.Join(upstreamRoot, name, "metadata.yml")
		local := filepath.Join(localRoot, name, "metadata.yml")

		upKeys, err := loadInputKeys(up)
		if err != nil {
			diff = append(diff, fmt.Sprintf("upstream %s %q: %v", kind, name, err))
			continue
		}
		localKeys, err := loadInputKeys(local)
		if err != nil {
			diff = append(diff, fmt.Sprintf("local %s %q: %v", kind, name, err))
			continue
		}
		for _, k := range diffKeys(upKeys, localKeys) {
			diff = append(diff, fmt.Sprintf("%s %q: upstream input %q missing from assets/", kind, name, k))
		}
	}
	return diff
}

// loadInputKeys reads the `inputs:` map keys from a metadata.yml file.
func loadInputKeys(path string) ([]string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path comes from a fixed allow-list.
	if err != nil {
		return nil, err
	}
	var meta struct {
		Inputs map[string]any `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(meta.Inputs))
	for k := range meta.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// diffKeys returns the keys present in want but missing from got. We
// intentionally do NOT report extra keys in got — adding a project-
// specific key is allowed (it is not a compatibility break).
func diffKeys(want, got []string) []string {
	have := make(map[string]struct{}, len(got))
	for _, k := range got {
		have[k] = struct{}{}
	}
	var missing []string
	for _, k := range want {
		if _, ok := have[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

// findRepoRoot walks upward from CWD looking for go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
