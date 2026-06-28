package action

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// canonicalChromeSections is the canonical ordered list of `chrome_*`
// section names used by both the deprecation alias translator below
// and gen-action-yml's discovered metadata. Kept in lockstep with
// assets/plugins/chrome/metadata.yml.
var canonicalChromeSections = []string{
	"header", "activity", "community", "repositories", "metadata", "introduction",
}

// TranslateLegacyChromeInputs rewrites the deprecated v2 chrome inputs
// (`base=CSV`, `plugin_base_activity`, `plugin_base_repositories`) into
// the canonical `chrome_<section>` booleans (#640), emitting a single
// deprecation slog.Warn per legacy key that names both the input it
// found and the chrome_* key it set.
//
// The function NEVER overwrites a chrome_* key the caller already set;
// the new input always wins so callers in the middle of a migration
// can pin a specific section without the legacy alias overriding it.
//
// Returns the (possibly mutated) inputs map for chainability; callers
// typically discard the return value and rely on the in-place edit.
func TranslateLegacyChromeInputs(inputs map[string]any) map[string]any {
	if inputs == nil {
		return inputs
	}

	// 1. base= CSV → chrome_<section>=yes for each listed section.
	//
	// Skip the empty-string case (`--plugin base=`) entirely: it was the
	// v2 "no chrome" idiom, and chrome_* default-no maps to the same
	// outcome. Warning the user about a v3-removed input they are not
	// even *using* would be noise (cap-1 review SHOULD-FIX #2).
	if raw, ok := readBaseCSV(inputs); ok && raw != "" {
		sections := splitBaseCSV(raw)
		// Translate in canonical order so the warning's "translated"
		// list is deterministic.
		translated := make([]string, 0, len(sections))
		for _, s := range canonicalChromeSections {
			if _, want := sections[s]; !want {
				continue
			}
			key := "chrome_" + s
			if _, exists := inputs[key]; exists {
				continue // caller already pinned this section explicitly.
			}
			inputs[key] = true
			translated = append(translated, key+"=yes")
		}
		// Surface unknown sections so typos are visible.
		var unknown []string
		known := map[string]struct{}{}
		for _, s := range canonicalChromeSections {
			known[s] = struct{}{}
		}
		for s := range sections {
			if _, ok := known[s]; !ok {
				unknown = append(unknown, s)
			}
		}
		sort.Strings(unknown)
		slog.Warn(
			"`base` input is deprecated; use `chrome_<section>=yes` (removed in v3.0)",
			"base", raw,
			"translated", strings.Join(translated, ", "),
			"unknown_sections", strings.Join(unknown, ", "),
		)
	}

	// 2. plugin_base_activity → chrome_activity.
	if isTruthy(inputs["plugin_base_activity"]) {
		translateLegacyChromeBool(inputs, "plugin_base_activity", "chrome_activity")
	}

	// 3. plugin_base_repositories → chrome_repositories.
	if isTruthy(inputs["plugin_base_repositories"]) {
		translateLegacyChromeBool(inputs, "plugin_base_repositories", "chrome_repositories")
	}

	return inputs
}

// translateLegacyChromeBool sets the canonical `chrome_*` key from a
// truthy legacy `plugin_base_*` input and emits a deprecation warning
// whose phrasing reflects whether the legacy alias actually moved the
// chrome_* key. When the caller already pinned chrome_* explicitly,
// the warning notes that the legacy alias was ignored — avoiding the
// misleading "translated" phrasing called out in PR #641 cap-1
// review SHOULD-FIX #3.
func translateLegacyChromeBool(inputs map[string]any, legacyKey, chromeKey string) {
	msg := fmt.Sprintf(
		"`%s` is deprecated; use `%s=yes` (removed in v3.0)",
		legacyKey, chromeKey,
	)
	if _, exists := inputs[chromeKey]; exists {
		slog.Warn(
			msg,
			"note", chromeKey+" already set explicitly; legacy alias ignored",
		)
		return
	}
	inputs[chromeKey] = true
	slog.Warn(msg, "translated", chromeKey+"=yes")
}

// readBaseCSV pulls the legacy `base` input out of the map, accepting
// the string / []string / []any shapes ParseInputs may produce.
// Returns (value, true) when the key is present (even empty) so the
// translator can decide based on presence.
func readBaseCSV(in map[string]any) (string, bool) {
	v, ok := in["base"]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case []string:
		return strings.Join(x, ","), true
	case []any:
		parts := make([]string, 0, len(x))
		for _, p := range x {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ","), true
	}
	return "", false
}

// splitBaseCSV is a local copy of chrome.splitBaseCSV; the action
// package does not import internal/templates/chrome to avoid pulling
// the templates layer into the entrypoint.
func splitBaseCSV(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		s := strings.ToLower(strings.TrimSpace(part))
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}

// ParseInputs builds the unified inputs map used by both Action mode
// and CLI mode. Priority (highest first):
//
//  1. INPUTS — a single env var holding a JSON object. Set by callers
//     that want to pass a complete input set in one go (e.g., the
//     composite GitHub Actions wrapper or programmatic CLI users).
//  2. INPUT_<UPPER> — individual env vars per input. Set by GitHub
//     Actions runner for each `with:` key in the workflow.
//
// Keys are normalized to lowercase + underscores (matching the
// metadata.yml convention).
func ParseInputs(env map[string]string) (map[string]any, error) {
	out := map[string]any{}

	// Layer 1 (lowest): INPUT_<UPPER>.
	for key, value := range env {
		if !strings.HasPrefix(key, "INPUT_") {
			continue
		}
		name := strings.ToLower(strings.TrimPrefix(key, "INPUT_"))
		out[name] = value
	}

	// Layer 2 (highest): INPUTS JSON. Overwrites INPUT_<UPPER> on key
	// collision per the documented precedence.
	if raw, ok := env["INPUTS"]; ok && raw != "" {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, fmt.Errorf("ParseInputs: INPUTS JSON decode: %w", err)
		}
		for key, value := range decoded {
			out[strings.ToLower(key)] = value
		}
	}

	return out, nil
}

// WildcardFilename resolves the `*` placeholder in a filename like
// `github-metrics.*` using the configured output format.
//
// Examples:
//
//	WildcardFilename("github-metrics.*", "svg")  → "github-metrics.svg"
//	WildcardFilename("github-metrics.*", "json") → "github-metrics.json"
//	WildcardFilename("custom.png",        "svg") → "custom.png" (no wildcard)
//
// Returns an error when the input contains more than one `*` since
// the upstream contract only defines a single wildcard placeholder.
func WildcardFilename(template, format string) (string, error) {
	count := strings.Count(template, "*")
	switch count {
	case 0:
		return template, nil
	case 1:
		ext := normalizeExtension(format)
		return strings.Replace(template, "*", ext, 1), nil
	default:
		return "", fmt.Errorf("WildcardFilename: template %q contains multiple '*' (only 1 allowed)", template)
	}
}

func normalizeExtension(format string) string {
	switch strings.ToLower(format) {
	case "", "svg":
		return "svg"
	case "png":
		return "png"
	case "jpeg", "jpg":
		return "jpeg"
	case "json":
		return "json"
	default:
		// Unknown formats pass through verbatim so downstream
		// validation (output_action / Compute) surfaces the error.
		return format
	}
}

// PresetBundle holds the inputs overlay loaded from a preset YAML
// file referenced via the `config_presets` input.
type PresetBundle struct {
	Path string         // source file path
	Q    map[string]any // input overrides
}

// LoadPreset reads the YAML at path and returns a PresetBundle. The
// expected schema is a single top-level `q:` map whose entries
// override individual inputs.
//
//	# example preset
//	q:
//	  plugin_languages: true
//	  plugin_languages_limit: 5
func LoadPreset(path string) (*PresetBundle, error) {
	body, err := os.ReadFile(path) //nolint:gosec // path is user-supplied via the config_presets input by design
	if err != nil {
		return nil, fmt.Errorf("LoadPreset: read %s: %w", path, err)
	}
	var doc struct {
		Q map[string]any `yaml:"q"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("LoadPreset: parse %s: %w", path, err)
	}
	if doc.Q == nil {
		return nil, fmt.Errorf("LoadPreset: %s missing required `q:` map", path)
	}
	return &PresetBundle{Path: path, Q: doc.Q}, nil
}

// MergeInto overlays the preset's q: entries onto the inputs map.
// Keys already present in inputs are NOT overwritten — preset values
// rank below CLI flag / INPUT_<UPPER> / INPUTS JSON in the priority
// chain.
func (p *PresetBundle) MergeInto(inputs map[string]any) {
	if p == nil {
		return
	}
	for key, value := range p.Q {
		k := strings.ToLower(key)
		if _, exists := inputs[k]; !exists {
			inputs[k] = value
		}
	}
}
