package action

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// legacyChromeReplacement maps each v3.0-removed chrome input to the
// canonical replacement the user should write instead. Two of the
// keys have direct 1:1 chrome_* replacements; `base` (CSV) expanded
// to N sections so its hint stays generic and points at the docs.
//
// The map's keys form the closed set ParseInputs scans for — adding
// a new entry here automatically extends the deprecation warning.
var legacyChromeReplacement = map[string]string{
	"base":                     "chrome_<section>=yes (per-section booleans; see docs/plugins/base.md)",
	"plugin_base_activity":     "chrome_activity=yes",
	"plugin_base_repositories": "chrome_repositories=yes",
}

// legacyChromeInputKeys is the iteration order for legacyChromeReplacement.
// Map iteration is unordered in Go, so we keep a fixed slice so log
// output is deterministic across runs.
var legacyChromeInputKeys = []string{
	"base",
	"plugin_base_activity",
	"plugin_base_repositories",
}

// legacyChromeWarnOnce gates the deprecation warning for each removed
// key to a single emission per process — long-running invocations must
// not spam the log on every ParseInputs call.
var legacyChromeWarnOnce sync.Map // map[string]*sync.Once

// WarnLegacyChromeInputs scans inputs for keys removed in v3.0 and
// emits one slog.Warn per unique key per process. The function is
// purely diagnostic: it never translates, falls back, or mutates the
// inputs map. Callers downstream see the raw legacy keys as unknown
// inputs (silently ignored by the chrome reader).
//
// Each warning names the canonical chrome_* replacement so a stale-
// config user can fix CI from the log alone (PR #650 cap-1 review
// MUST-FIX #1).
//
// Called by runWith / runCLIWith after the full inputs map is
// assembled (env + preset + CLI flags) so a key surfaced by any layer
// is reported.
func WarnLegacyChromeInputs(inputs map[string]any) {
	if inputs == nil {
		return
	}
	for _, key := range legacyChromeInputKeys {
		if _, ok := inputs[key]; !ok {
			continue
		}
		gate, _ := legacyChromeWarnOnce.LoadOrStore(key, &sync.Once{})
		gate.(*sync.Once).Do(func() {
			slog.Warn(
				fmt.Sprintf("`%s` input was removed in v3.0; use `%s`", key, legacyChromeReplacement[key]),
			)
		})
	}
}

// resetLegacyChromeWarnOnceForTest clears the sync.Once gates so a
// test can call WarnLegacyChromeInputs multiple times and observe
// fresh emissions per case. Test-only seam.
func resetLegacyChromeWarnOnceForTest() {
	legacyChromeWarnOnce = sync.Map{}
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
