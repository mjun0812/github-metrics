package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// NormalizeInput converts raw (typically a string from env or YAML
// decoding) into the Go value implied by def.Type. The supported types
// are the set declared in [allowedInputTypes]. Unsupported types return
// an InputError-style error; type-coercion failures likewise return an
// error so callers can surface them on the upstream-compatible warning
// path.
//
// Empty / nil raw plus a non-empty Default node falls back to the
// default value rendered through the same normalization.
func NormalizeInput(def InputDef, raw any) (any, error) {
	if isEmpty(raw) {
		if def.Default.Kind != 0 {
			raw = renderYAMLNode(def.Default)
		}
	}

	switch def.Type {
	case "", "string":
		return toString(raw), nil
	case "number":
		return toNumber(raw)
	case "boolean":
		return toBoolean(raw), nil
	case "array":
		return toArray(raw, def.Format), nil
	case "token":
		return NewToken(toString(raw)), nil
	case "json":
		return toJSON(raw)
	default:
		return nil, fmt.Errorf("normalize: unsupported type %q", def.Type)
	}
}

// Inputs is the per-scope normalized view of inputs keyed by metadata
// input name.
type Inputs struct {
	defs    map[string]InputDef
	loaded  map[string]any
	resolve placeholderResolver
}

// NewInputs returns an Inputs ready to ingest values for the provided
// definition map. The defs typically come from
// [MetadataLoader.Plugins[...].Inputs] merged with the corresponding
// template inputs.
func NewInputs(defs map[string]InputDef) *Inputs {
	return &Inputs{
		defs:    defs,
		loaded:  map[string]any{},
		resolve: noopResolver{},
	}
}

// ForAction reads action-style inputs (INPUTS JSON + INPUT_<UPPER>
// environment variables), applies any preset overrides, normalizes each
// value against the input definition, and returns a flat map ready for
// PluginContext consumption.
//
// Resolution order (from highest to lowest precedence):
//  1. INPUTS JSON keys (when present)
//  2. INPUT_<UPPER> environment values
//  3. preset overrides
//  4. metadata default
//
// The function leaves placeholders (`.user.login`,
// `.repository.name`, ...) intact unless an explicit data context has
// been wired in via [Inputs.WithData]; the engine resolves them later.
func (i *Inputs) ForAction(env map[string]string, preset map[string]any) (map[string]any, error) {
	q := map[string]any{}

	inputsJSON, hasInputsJSON := env["INPUTS"]
	jsonOverrides := map[string]json.RawMessage{}
	if hasInputsJSON && strings.TrimSpace(inputsJSON) != "" {
		if err := json.Unmarshal([]byte(inputsJSON), &jsonOverrides); err != nil {
			return nil, fmt.Errorf("inputs: parse INPUTS: %w", err)
		}
	}

	for name, def := range i.defs {
		raw, present := lookupAction(name, env, jsonOverrides, preset)
		if !present && def.Default.Kind == 0 {
			continue
		}
		val, err := NormalizeInput(def, raw)
		if err != nil {
			return nil, fmt.Errorf("inputs: %s: %w", name, err)
		}
		if s, ok := val.(string); ok {
			val = i.resolve.resolve(s)
		}
		q[name] = val
	}
	return q, nil
}

// ForWeb reads a url.Values-like map (web instance flow); the upstream
// behavior is mostly identical to ForAction without the INPUTS JSON
// hop. Implemented here for parity even though Web is not part of M1
// scope (constitution principle III honors the input contract).
func (i *Inputs) ForWeb(query map[string][]string) (map[string]any, error) {
	q := map[string]any{}
	for name, def := range i.defs {
		var raw any
		if vs, ok := query[name]; ok && len(vs) > 0 {
			raw = vs[0]
		}
		val, err := NormalizeInput(def, raw)
		if err != nil {
			return nil, fmt.Errorf("inputs: %s: %w", name, err)
		}
		if s, ok := val.(string); ok {
			val = i.resolve.resolve(s)
		}
		q[name] = val
	}
	return q, nil
}

// ForData is invoked once `data.User` (and friends) are populated; the
// resolver knows how to substitute `.user.login` and similar
// placeholders. M1 callers may pass nil for data — placeholders then
// pass through unchanged.
func (i *Inputs) ForData(q map[string]any, data *DataView, account string) map[string]any {
	res := newDataResolver(data, account)
	out := make(map[string]any, len(q))
	for k, v := range q {
		if s, ok := v.(string); ok {
			out[k] = res.resolve(s)
			continue
		}
		out[k] = v
	}
	return out
}

// DataView is the minimal subset of engine data needed to resolve
// inputs placeholders. The engine package wraps a richer Data into a
// DataView before calling [Inputs.ForData], so internal/config does not
// import internal/engine.
type DataView struct {
	UserLogin      string
	UserName       string
	RepositoryName string
	RepositoryFull string
}

// lookupAction picks the highest-precedence raw value for the given
// input name out of the INPUTS JSON, INPUT_<UPPER> env, and preset map.
func lookupAction(
	name string,
	env map[string]string,
	jsonOverrides map[string]json.RawMessage,
	preset map[string]any,
) (any, bool) {
	if raw, ok := jsonOverrides[name]; ok {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		return string(raw), true
	}
	if v, ok := env["INPUT_"+strings.ToUpper(strings.ReplaceAll(name, ".", "_"))]; ok {
		return v, true
	}
	if preset != nil {
		if v, ok := preset[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// --- type coercion helpers --------------------------------------------------

func isEmpty(raw any) bool {
	switch v := raw.(type) {
	case nil:
		return true
	case string:
		return v == ""
	default:
		return false
	}
}

func renderYAMLNode(n yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	default:
		// Defaults are typically scalars; non-scalar defaults are
		// passed through as their YAML serialization (rare and only
		// affects niche inputs).
		b, err := yaml.Marshal(&n)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
}

func toString(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toNumber(raw any) (float64, error) {
	switch v := raw.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("number parse %q: %w", v, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("number unsupported type %T", raw)
	}
}

// truthyValues matches the upstream legacy-converter boolean casts
// (docs/design/13-appendix.md §F): "yes" / "true" / "1" / "on" are true;
// "" defers to default; everything else is false.
var truthyValues = map[string]bool{
	"yes": true, "true": true, "on": true, "1": true,
	"no": false, "false": false, "off": false, "0": false, "": false,
}

func toBoolean(raw any) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		if b, ok := truthyValues[strings.ToLower(strings.TrimSpace(v))]; ok {
			return b
		}
		return false
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	default:
		return false
	}
}

// toArray splits a scalar/multi-line string according to the requested
// format(s). The metadata loader normalizes format to a StringList, so
// callers can pass multiple separators (most commonly the upstream
// "newline-separated, comma-separated" pair).
func toArray(raw any, formats StringList) []string {
	if len(formats) == 0 {
		formats = StringList{"comma-separated"}
	}
	var s string
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, toString(item))
		}
		return dedup(trim(out))
	case []string:
		return dedup(trim(v))
	default:
		s = toString(raw)
	}
	if s == "" {
		return []string{}
	}
	separators := separatorsFor(formats)
	parts := []string{s}
	for _, sep := range separators {
		next := make([]string, 0, len(parts))
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	return dedup(trim(parts))
}

func separatorsFor(formats StringList) []string {
	out := make([]string, 0, len(formats))
	for _, f := range formats {
		switch f {
		case "comma-separated":
			out = append(out, ",")
		case "space-separated":
			out = append(out, " ")
		case "newline-separated":
			out = append(out, "\n")
		}
	}
	if len(out) == 0 {
		return []string{","}
	}
	return out
}

func trim(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func toJSON(raw any) (json.RawMessage, error) {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return json.RawMessage("null"), nil
		}
		var probe any
		if err := json.Unmarshal([]byte(v), &probe); err != nil {
			return nil, fmt.Errorf("json parse: %w", err)
		}
		return json.RawMessage(v), nil
	case json.RawMessage:
		return v, nil
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
}

// --- placeholder resolution ------------------------------------------------

type placeholderResolver interface {
	resolve(s string) string
}

type noopResolver struct{}

func (noopResolver) resolve(s string) string { return s }

type dataResolver struct {
	user     string
	username string
	repo     string
	repoFull string
	account  string
}

func newDataResolver(d *DataView, account string) placeholderResolver {
	if d == nil {
		return noopResolver{}
	}
	return &dataResolver{
		user:     d.UserLogin,
		username: d.UserName,
		repo:     d.RepositoryName,
		repoFull: d.RepositoryFull,
		account:  account,
	}
}

func (r *dataResolver) resolve(s string) string {
	replacements := []struct {
		token string
		value string
	}{
		{".user.login", r.user},
		{".user.name", r.username},
		{".repository.name", r.repo},
		{".repository.full_name", r.repoFull},
	}
	for _, rep := range replacements {
		if rep.value == "" {
			continue
		}
		s = strings.ReplaceAll(s, rep.token, rep.value)
	}
	return s
}
