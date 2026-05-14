package config

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// InputDef captures the relevant fields of an input definition inside a
// plugin or template metadata.yml. Default is kept as a yaml.Node so the
// loader does not have to commit to a Go type at parse time — the input
// normalizer ([Inputs.Normalize]) interprets it against the declared
// type at use site.
type InputDef struct {
	Description string     `yaml:"description"`
	Type        string     `yaml:"type"`
	Format      StringList `yaml:"format"`
	Default     yaml.Node  `yaml:"default"`
	Values      []string   `yaml:"values"`
	Min         *float64   `yaml:"min"`
	Max         *float64   `yaml:"max"`
	Global      bool       `yaml:"global"`
	Preset      *bool      `yaml:"preset"`
	Extras      []string   `yaml:"extras"`
}

// StringList accepts either a single scalar or a YAML sequence and
// normalizes both into a slice. Some upstream metadata.yml fields
// (notably `format` and `values`) come in either shape.
type StringList []string

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*s = StringList{node.Value}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			out = append(out, item.Value)
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("expected scalar or sequence, got kind %d", node.Kind)
	}
}

// PluginMetadata is the parsed assets/plugins/<name>/metadata.yml shape.
type PluginMetadata struct {
	Name        string              `yaml:"name"`
	Category    string              `yaml:"category"`
	Description string              `yaml:"description"`
	Index       int                 `yaml:"index"`
	Supports    []string            `yaml:"supports"`
	Scopes      []string            `yaml:"scopes"`
	Inputs      map[string]InputDef `yaml:"inputs"`
	Examples    map[string]string   `yaml:"examples"`
}

// TemplateMetadata is the parsed assets/templates/<name>/metadata.yml shape.
type TemplateMetadata struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Index       int                 `yaml:"index"`
	Supports    []string            `yaml:"supports"`
	Formats     []string            `yaml:"formats"`
	Inputs      map[string]InputDef `yaml:"inputs"`
	Examples    map[string]string   `yaml:"examples"`
}

// ActionMetadata is the parsed action.yml at the repository root.
type ActionMetadata struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	Author      string                    `yaml:"author"`
	Branding    map[string]string         `yaml:"branding"`
	Runs        map[string]any            `yaml:"runs"`
	Inputs      map[string]ActionInputDef `yaml:"inputs"`
}

// ActionInputDef is the simpler shape used by action.yml.
type ActionInputDef struct {
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Required    bool   `yaml:"required"`
}

// PackageMetadata carries the upstream version captured by sync-assets.sh
// in assets/version.txt.
type PackageMetadata struct {
	UpstreamVersion string
}

// MetadataLoader aggregates every metadata.yml-shaped artifact the
// engine needs at runtime. It is read-only after [Load] completes.
type MetadataLoader struct {
	Plugins   map[string]*PluginMetadata
	Templates map[string]*TemplateMetadata
	Action    *ActionMetadata
	Package   *PackageMetadata
}

// LoadOptions controls non-default Load behavior.
type LoadOptions struct {
	// ActionPath is the path inside fsys for action.yml. Empty value
	// disables action.yml parsing (useful in tests that only exercise
	// plugin/template metadata).
	ActionPath string
	// VersionPath is the path inside fsys for version.txt. Empty value
	// skips version capture.
	VersionPath string
	// Logger is used for warn-level messages (e.g. unknown input type).
	// nil falls back to slog.Default().
	Logger *slog.Logger
}

// LoadMetadata walks the embedded filesystem rooted at fsys, reading
// every metadata.yml beneath plugins/ and templates/, plus an optional
// action.yml and version.txt. Unknown input types are logged at warn
// level and the offending input is dropped (constitution edge case for
// forward compatibility); other parse errors abort the load.
func LoadMetadata(fsys fs.FS, opts LoadOptions) (*MetadataLoader, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	out := &MetadataLoader{
		Plugins:   map[string]*PluginMetadata{},
		Templates: map[string]*TemplateMetadata{},
	}

	plugins, err := readPluginsDir(fsys, "plugins")
	if err != nil {
		return nil, fmt.Errorf("metadata: plugins: %w", err)
	}
	for name, raw := range plugins {
		var pm PluginMetadata
		if err := yaml.Unmarshal(raw, &pm); err != nil {
			return nil, fmt.Errorf("metadata: plugin %s: %w", name, err)
		}
		sanitizeInputs(pm.Inputs, "plugin "+name, logger)
		out.Plugins[name] = &pm
	}

	tmpls, err := readPluginsDir(fsys, "templates")
	if err != nil {
		return nil, fmt.Errorf("metadata: templates: %w", err)
	}
	for name, raw := range tmpls {
		var tm TemplateMetadata
		if err := yaml.Unmarshal(raw, &tm); err != nil {
			return nil, fmt.Errorf("metadata: template %s: %w", name, err)
		}
		sanitizeInputs(tm.Inputs, "template "+name, logger)
		out.Templates[name] = &tm
	}

	if opts.ActionPath != "" {
		raw, err := fs.ReadFile(fsys, opts.ActionPath)
		if err != nil {
			return nil, fmt.Errorf("metadata: action: %w", err)
		}
		var am ActionMetadata
		if err := yaml.Unmarshal(raw, &am); err != nil {
			return nil, fmt.Errorf("metadata: action parse: %w", err)
		}
		out.Action = &am
	}

	if opts.VersionPath != "" {
		raw, err := fs.ReadFile(fsys, opts.VersionPath)
		if err != nil {
			return nil, fmt.Errorf("metadata: version: %w", err)
		}
		out.Package = &PackageMetadata{
			UpstreamVersion: strings.TrimSpace(string(raw)),
		}
	}

	return out, nil
}

// readPluginsDir returns a map from directory-name to the bytes of that
// directory's metadata.yml. Directories without metadata.yml are
// skipped silently (they may host queries or examples but no schema).
func readPluginsDir(fsys fs.FS, dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		// An entirely missing directory is acceptable: the caller may
		// pass a filesystem trimmed to plugins or to templates only.
		if isNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := path.Join(dir, e.Name(), "metadata.yml")
		raw, err := fs.ReadFile(fsys, metaPath)
		if err != nil {
			if isNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", metaPath, err)
		}
		out[e.Name()] = raw
	}
	return out, nil
}

// isNotExist mirrors os.IsNotExist for fs.ErrNotExist-wrapped errors.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	// fs.ReadFile / fs.ReadDir returns *fs.PathError wrapping fs.ErrNotExist.
	for _, target := range []error{fs.ErrNotExist} {
		if errorsIs(err, target) {
			return true
		}
	}
	return false
}

// errorsIs is a tiny indirection so this file does not import the
// "errors" package and shadow the project's internal/errors usage.
func errorsIs(err, target error) bool {
	type wrapped interface{ Unwrap() error }
	for err != nil {
		if err == target { //nolint:errorlint // explicit identity match is intentional
			return true
		}
		w, ok := err.(wrapped)
		if !ok {
			return false
		}
		err = w.Unwrap()
	}
	return false
}

// allowedInputTypes mirrors the set the inputs parser knows how to
// normalize (see contracts/cli.md §1.3). Unknown types are warn-logged
// and dropped so forward-compatible upstream changes do not block load.
var allowedInputTypes = map[string]struct{}{
	"string":  {},
	"number":  {},
	"boolean": {},
	"array":   {},
	"token":   {},
	"json":    {},
}

// sanitizeInputs drops inputs whose type is unknown to the loader and
// emits a warn-level log for each removal.
func sanitizeInputs(inputs map[string]InputDef, scope string, logger *slog.Logger) {
	if len(inputs) == 0 {
		return
	}
	var bad []string
	for name, def := range inputs {
		if def.Type == "" {
			continue
		}
		if _, ok := allowedInputTypes[def.Type]; !ok {
			bad = append(bad, name)
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	for _, name := range bad {
		logger.Warn("metadata: unknown input type, dropping",
			"scope", scope, "input", name, "type", inputs[name].Type)
		delete(inputs, name)
	}
}

// ToAction returns the action.yml input key corresponding to a logical
// metadata key. Upstream collapses keys directly (no rewriting) — we
// preserve that behavior so existing user workflows remain compatible.
func (m *MetadataLoader) ToAction(key string) string { return key }

// ToWeb returns the web query parameter for a key (alias of ToAction in
// upstream).
func (m *MetadataLoader) ToWeb(key string) string { return key }

// ToQuery returns the internal query map key for an input.
func (m *MetadataLoader) ToQuery(key string) string { return key }

// PluginNames returns plugin keys sorted alphabetically (deterministic
// for golden file comparisons).
func (m *MetadataLoader) PluginNames() []string {
	out := make([]string, 0, len(m.Plugins))
	for k := range m.Plugins {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TemplateNames returns template keys sorted alphabetically.
func (m *MetadataLoader) TemplateNames() []string {
	out := make([]string, 0, len(m.Templates))
	for k := range m.Templates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Extras reports whether a given extras feature flag is enabled for the
// plugin/template named scope. Upstream encodes extras as a list of
// dotted feature names on PluginMetadata.Inputs[...].Extras; this helper
// keeps callers from reaching into the map.
func (m *MetadataLoader) Extras(scope, feature string) bool {
	if pm, ok := m.Plugins[scope]; ok {
		for _, def := range pm.Inputs {
			for _, f := range def.Extras {
				if f == feature {
					return true
				}
			}
		}
	}
	return false
}
