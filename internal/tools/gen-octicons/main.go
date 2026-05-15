// Command gen-octicons reads the @primer/octicons build/data.json
// shipped by the npm package and writes a trimmed, embed-friendly
// assets/octicons/data.json that internal/render.ReplaceOcticons
// consumes at runtime.
//
// Approach:
//
//   - The upstream build/data.json is a map keyed by icon name. Each
//     entry has a "heights" sub-object whose keys are the available
//     pixel sizes ("16", "24"). Inside each size we have a "width" and
//     a "path" (an HTML fragment such as `<path d="..."/>`).
//
//   - We materialize the equivalent `<svg xmlns=... width=H height=H
//     viewBox="0 0 H H">{path}</svg>` string for every (name, size)
//     pair and store it in our output JSON under
//     `icons[name][size]`. The `class="octicon"` attribute is NOT
//     baked in; the runtime injects it during the replacement to keep
//     this asset reusable.
//
// Output schema (see specs/003-chromedp-rendering-pipeline/data-model.md E-006):
//
//	{
//	  "_meta": {"source": "primer/octicons@<version>", "generated_at": "<rfc3339>"},
//	  "icons": { "<name>": { "<size>": "<svg ...>...</svg>" } }
//	}
//
// Exit codes:
//
//	0  data.json written
//	1  invocation / I/O / decode error
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// upstreamIcon mirrors the relevant subset of @primer/octicons
// build/data.json entries. Unknown fields are ignored.
type upstreamIcon struct {
	Name    string                  `json:"name"`
	Heights map[string]upstreamSize `json:"heights"`
}

type upstreamSize struct {
	Width int    `json:"width"`
	Path  string `json:"path"`
}

// outDoc is the trimmed shape this tool writes.
type outDoc struct {
	Meta  outMeta                      `json:"_meta"`
	Icons map[string]map[string]string `json:"icons"`
}

type outMeta struct {
	Source      string `json:"source"`
	GeneratedAt string `json:"generated_at"`
}

func main() {
	in := flag.String("in", "node_modules/@primer/octicons/build/data.json", "input primer/octicons build/data.json")
	out := flag.String("out", "assets/octicons/data.json", "output asset path")
	source := flag.String("source", "", "human-readable source label (defaults to @primer/octicons@<package.json version> when discoverable)")
	flag.Parse()

	if err := run(*in, *out, *source, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, "gen-octicons: %v\n", err)
		os.Exit(1)
	}
}

// run is the testable core of the tool: it reads the upstream JSON
// from `inPath`, materializes the trimmed output, and writes it to
// `outPath`. The `nowFn`-equivalent `now` parameter makes the
// generated_at metadata deterministic in tests.
func run(inPath, outPath, source string, now time.Time) error {
	raw, err := os.ReadFile(inPath) //nolint:gosec // inPath is supplied via CLI flag from a controlled developer/CI environment
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}

	var upstream map[string]upstreamIcon
	if decErr := json.Unmarshal(raw, &upstream); decErr != nil {
		return fmt.Errorf("decode upstream data: %w", decErr)
	}

	doc := outDoc{
		Meta: outMeta{
			Source:      resolveSource(source, inPath),
			GeneratedAt: now.Format(time.RFC3339),
		},
		Icons: make(map[string]map[string]string, len(upstream)),
	}

	for name, ic := range upstream {
		if name == "" {
			continue
		}
		entry := make(map[string]string, len(ic.Heights))
		// Stable iteration order for diff hygiene.
		sizes := make([]string, 0, len(ic.Heights))
		for s := range ic.Heights {
			sizes = append(sizes, s)
		}
		sort.Strings(sizes)
		for _, size := range sizes {
			variant := ic.Heights[size]
			if variant.Path == "" {
				continue
			}
			entry[size] = buildSVG(variant.Width, atoiSafe(size), variant.Path)
		}
		if len(entry) > 0 {
			doc.Icons[name] = entry
		}
	}

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	// Always end with a single newline for git-friendly diffs.
	encoded = append(encoded, '\n')

	if mkErr := os.MkdirAll(filepath.Dir(outPath), 0o750); mkErr != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), mkErr)
	}
	if writeErr := os.WriteFile(outPath, encoded, 0o600); writeErr != nil { //nolint:gosec // outPath is supplied via CLI flag from a controlled developer/CI environment
		return fmt.Errorf("write %s: %w", outPath, writeErr)
	}
	return nil
}

// buildSVG assembles the `<svg ...>{path}</svg>` fragment matching the
// upstream visual. width/height fall back to size when the upstream
// width is missing — squarish icons make up the entire primer/octicons
// catalog so this is safe.
func buildSVG(width, size int, path string) string {
	if width <= 0 {
		width = size
	}
	height := size
	viewBox := fmt.Sprintf("0 0 %d %d", width, height)
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="%s">%s</svg>`,
		width, height, viewBox, path,
	)
}

// resolveSource picks the most informative source label we can: an
// explicit --source flag wins, otherwise we attempt to read the sibling
// package.json's version.
func resolveSource(explicit, inPath string) string {
	if explicit != "" {
		return explicit
	}
	pkgPath := filepath.Join(filepath.Dir(filepath.Dir(inPath)), "package.json")
	body, err := os.ReadFile(pkgPath) //nolint:gosec // pkgPath is derived from the CLI-supplied input path in a controlled dev/CI environment
	if err != nil {
		return "primer/octicons@unknown"
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil || pkg.Version == "" {
		return "primer/octicons@unknown"
	}
	return "primer/octicons@" + strings.TrimSpace(pkg.Version)
}

func atoiSafe(s string) int {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
