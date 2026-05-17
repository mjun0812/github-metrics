package action

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CLIFlags is the parsed result of `metrics-action <flags>` when the
// binary runs outside GitHub Actions. Spec data-model E-004 +
// contracts/cli-flags.md.
//
// The fields map 1:1 onto action.yml inputs so the merged
// *Invocation produced by ToInvocation is interchangeable with the
// Action-mode pipeline (FR-019).
type CLIFlags struct {
	Config   string            // --config <path>.yaml
	User     string            // --user <login>
	Template string            // --template <name>
	Token    string            // --token <PAT>
	TokenEnv string            // --token-env <ENV_NAME>
	Plugins  map[string]string // --plugin key=value (repeatable)
	Output   string            // --output svg|png|jpeg|json
	Filename string            // --filename <path-or-->
	Dryrun   bool              // --dryrun
	Preset   string            // --preset <path>.yaml
}

// ParseFlags parses the supplied args (typically os.Args[1:] after
// bootstrap flags are stripped in cmd/metrics-action) into a CLIFlags
// struct. Errors come from flag.FlagSet (continue on error).
func ParseFlags(args []string) (*CLIFlags, error) {
	cf := &CLIFlags{Plugins: map[string]string{}}
	fs := flag.NewFlagSet("metrics-action", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cf.Config, "config", "", "YAML config path (action.yml-equivalent inputs)")
	fs.StringVar(&cf.User, "user", "", "GitHub user / org login")
	fs.StringVar(&cf.Template, "template", "", "template name (default: classic)")
	fs.StringVar(&cf.Token, "token", "", "GitHub PAT (history-visible; prefer --token-env)")
	fs.StringVar(&cf.TokenEnv, "token-env", "", "read token from os.Getenv(<NAME>)")
	fs.StringVar(&cf.Output, "output", "", "output format: svg|png|jpeg|json")
	fs.StringVar(&cf.Filename, "filename", "", "output path; '-' for stdout")
	fs.BoolVar(&cf.Dryrun, "dryrun", false, "skip commit/PR output_action side effects")
	fs.StringVar(&cf.Preset, "preset", "", "preset YAML path (config_presets)")

	fs.Var(&pluginFlag{m: cf.Plugins}, "plugin", "key=value plugin input (repeatable)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cf.Template == "" {
		cf.Template = "classic"
	}
	if cf.Output == "" {
		cf.Output = "svg"
	}
	return cf, nil
}

// pluginFlag is the flag.Value used to accumulate repeated
// `--plugin key=value` flags into the CLIFlags.Plugins map.
type pluginFlag struct {
	m map[string]string
}

func (p *pluginFlag) String() string {
	if p == nil || len(p.m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(p.m))
	for k := range p.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+p.m[k])
	}
	return strings.Join(parts, ",")
}

func (p *pluginFlag) Set(raw string) error {
	idx := strings.IndexByte(raw, '=')
	if idx <= 0 {
		return fmt.Errorf("--plugin must be key=value (got %q)", raw)
	}
	key := strings.TrimSpace(raw[:idx])
	val := raw[idx+1:]
	if key == "" {
		return fmt.Errorf("--plugin key must be non-empty (got %q)", raw)
	}
	p.m[key] = val
	return nil
}

// LoadYAMLConfig reads a YAML file at path and flattens its
// hierarchical `plugins:` / `config:` / `committer:` maps into the
// action.yml-flat key namespace expected by ParseInputs (prefixes
// `plugin_`, `config_`, `committer_`).
//
// Top-level keys are passed through as-is. Returns nil + error if
// the file is missing or malformed.
func LoadYAMLConfig(path string) (map[string]any, error) {
	if path == "" {
		return nil, errors.New("LoadYAMLConfig: empty path")
	}
	raw, err := os.ReadFile(path) //nolint:gosec // user-supplied config file (intended)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %s: %w", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("config parse: %s: %w", path, err)
	}
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		switch k {
		case "plugins", "config", "committer":
			nested, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("config %q must be a map", k)
			}
			prefix := strings.TrimSuffix(k, "s") + "_" // plugins → plugin_, config → config_, committer → committer_
			if k == "committer" {
				prefix = "committer_"
			}
			for nk, nv := range nested {
				out[prefix+nk] = nv
			}
		case "output":
			// `output` is the format short-hand for `config_output`.
			out["config_output"] = v
		default:
			out[k] = v
		}
	}
	return out, nil
}

// ToInvocation merges CLI flags (highest priority), YAML config, env,
// and metadata defaults into a flat `map[string]any` suitable for
// newInvocation. Spec contracts/cli-flags.md §3.
//
// Priority (highest first):
//  1. CLI flag (--user, --plugin key=val, ...)
//  2. --config <path>.yaml
//  3. --preset <path>.yaml (loaded by Run-side preset overlay; we
//     just emit `config_presets` here so the unified pipeline runs
//     it through LoadPreset like the Action path).
//  4. INPUT_<UPPER> env variables (loaded via ParseInputs).
//  5. metadata.yml defaults (applied by ParseInputs).
//
// Returns the flat inputs map; the caller passes it through
// ParseInputs's metadata-default layer (newInvocation already does).
func (c *CLIFlags) ToInvocation(env map[string]string) (map[string]any, error) {
	inputs, err := ParseInputs(env)
	if err != nil {
		return nil, fmt.Errorf("CLI ToInvocation: env: %w", err)
	}

	if c.Config != "" {
		ycfg, yerr := LoadYAMLConfig(c.Config)
		if yerr != nil {
			return nil, yerr
		}
		for k, v := range ycfg {
			inputs[k] = v
		}
	}

	if c.Preset != "" {
		inputs["config_presets"] = c.Preset
	}

	if c.User != "" {
		inputs["user"] = c.User
	}
	if c.Template != "" {
		inputs["template"] = c.Template
	}
	if c.Output != "" {
		inputs["config_output"] = c.Output
	}
	if c.Filename != "" {
		inputs["filename"] = c.Filename
	}
	if c.Dryrun {
		inputs["dryrun"] = true
	}
	if c.Token != "" {
		inputs["token"] = c.Token
	}
	for k, v := range c.Plugins {
		inputs[k] = v
	}

	return inputs, nil
}

// ResolveToken applies the CLI token priority rules (contracts §4):
//
//	--token-env > --token (with warning) > INPUT_TOKEN > error
//
// `envLookup` is dependency-injected (defaults to os.Getenv) so
// tests can run hermetically.
func ResolveToken(cf *CLIFlags, envLookup func(string) string, inputToken string) (string, error) {
	if envLookup == nil {
		envLookup = os.Getenv
	}
	if cf.TokenEnv != "" && cf.Token != "" {
		slog.Warn("--token ignored when --token-env is set")
	}
	if cf.TokenEnv != "" {
		val := envLookup(cf.TokenEnv)
		if val == "" {
			return "", fmt.Errorf("--token-env: env %q is empty or unset", cf.TokenEnv)
		}
		return val, nil
	}
	if cf.Token != "" {
		slog.Warn("--token: token visible in shell history; prefer --token-env")
		return cf.Token, nil
	}
	if inputToken != "" {
		return inputToken, nil
	}
	return "", errors.New("token required: pass via --token, --token-env, or use --dryrun with use_mocked_data: yes")
}

// ResolveOutputWriter returns the io.Writer + close func for the CLI
// output target. `-` returns os.Stdout (no close), with a warning when
// the format is non-text (png/jpeg). Other filenames are os.Create'd
// after mkdir -p of their parent.
func ResolveOutputWriter(filename, format string) (io.Writer, func() error, error) {
	if filename == "-" {
		if format == "png" || format == "jpeg" {
			slog.Warn("writing binary format to stdout; pipe to a file or process for correctness",
				"format", format)
		}
		return os.Stdout, func() error { return nil }, nil
	}
	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // user-chosen path
			return nil, nil, fmt.Errorf("mkdir -p %s: %w", dir, err)
		}
	}
	f, err := os.Create(filename) //nolint:gosec // user-chosen path
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}
