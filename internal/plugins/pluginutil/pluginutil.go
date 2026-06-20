// Package pluginutil collects the small input-parsing and formatting
// helpers that nearly every plugin reimplements verbatim. Keep this
// file the single source of truth for those primitives so the per-
// plugin _.go files stay focused on their domain logic.
package pluginutil

import (
	"strconv"
	"strings"
)

// ZeroSHA is the all-zero git object id GitHub returns on the create-
// branch "before" / delete-branch "after" sides of a PushEvent.
const ZeroSHA = "0000000000000000000000000000000000000000"

// IsZeroSHA reports whether sha is empty or the all-zero git OID
// (transient create/delete markers in events).
func IsZeroSHA(sha string) bool { return sha == "" || sha == ZeroSHA }

// Truthy collapses the input-map's any to a bool. Accepts bool, the
// strings "true"/"1"/"yes" (case-insensitive, whitespace-trimmed), and
// non-zero numerics. Mirrors upstream metadata.mjs's `descriptor.type
// === "boolean"` parsing — the action YAML loader normalizes bools,
// but tests and direct callers may still pass raw strings.
func Truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

// TruthyInput is Truthy applied to in[key]. A nil map or missing key
// returns false.
func TruthyInput(in map[string]any, key string) bool {
	if in == nil {
		return false
	}
	return Truthy(in[key])
}

// ReadInt parses in[key] into an int, accepting Go ints, int64,
// float64 (the YAML loader's default for numerics), and base-10
// strings.
func ReadInt(in map[string]any, key string) (int, bool) {
	v, ok := in[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// ReadIntDefault returns ReadInt's value or def when the key is
// missing or unparsable.
func ReadIntDefault(in map[string]any, key string, def int) int {
	if n, ok := ReadInt(in, key); ok {
		return n
	}
	return def
}

// ReadBool parses in[key] into a bool via the same rules as Truthy.
// Missing/unparsable returns false.
func ReadBool(in map[string]any, key string) bool {
	return TruthyInput(in, key)
}

// ReadBoolDefault parses in[key] into a bool. Missing key returns def;
// a present but unparsable value also returns def (mirrors the legacy
// per-plugin behavior — a literal "off"/"no" string evaluates as
// !Truthy and therefore yields def, which matches upstream's
// "any unrecognized value means default" semantics).
func ReadBoolDefault(in map[string]any, key string, def bool) bool {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		switch s {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}

// ReadCSV parses in[key] as a comma-separated list. Accepts []string,
// []any, and string. Empty/whitespace-only entries are dropped.
func ReadCSV(in map[string]any, key string) []string {
	v, ok := in[key]
	if !ok {
		return nil
	}
	return ReadCSVValue(v)
}

// ReadCSVValue is ReadCSV's single-value form, for callers that have
// already pulled the value out of the map (or get it from elsewhere).
func ReadCSVValue(v any) []string {
	switch x := v.(type) {
	case []string:
		return TrimEmpty(x)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
				continue
			}
			out = append(out, toString(item))
		}
		return TrimEmpty(out)
	case string:
		return TrimEmpty(strings.Split(x, ","))
	}
	return nil
}

// TrimEmpty strips leading/trailing whitespace from each entry and
// drops the ones that collapse to empty.
func TrimEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// LoginFromInputs returns the GitHub login the plugin should act on:
// `user` first, then the upstream alias `login`. nil map returns "".
func LoginFromInputs(in map[string]any) string {
	if in == nil {
		return ""
	}
	if v, ok := in["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := in["login"].(string); ok {
		return v
	}
	return ""
}

// ExtrasEnabled gates a "show by default unless the user explicitly
// turned it off" toggle. Missing key (or nil map) means enabled.
func ExtrasEnabled(in map[string]any, key string) bool {
	if in == nil {
		return true
	}
	v, ok := in[key]
	if !ok {
		return true
	}
	return Truthy(v)
}

// Plural mirrors upstream's `s()` template helper: returns "" when n
// is 1, "s" otherwise. Use for English-style "1 commit" / "2 commits"
// labels.
func Plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return ""
}
