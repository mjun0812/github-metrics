package action

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
