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

// CLIFlags is the parsed result of `metrics-cli <flags>`. After the
// v3.0 mode-unification (#646) every invocation goes through this
// parser; flags are layered on top of the INPUT_<UPPER> / INPUTS JSON
// env values so a workflow can mix `with: token:` from a secret with
// `run: metrics-cli --debug` overrides.
//
// The fields map 1:1 onto action.yml inputs so the merged inputs map
// produced by applyFlagsOver / ToInvocation feeds the same engine
// pipeline regardless of whether the data came from env, YAML, or
// flags.
type CLIFlags struct {
	Config    string            // --config <path>.yaml
	User      string            // --user <login>
	Template  string            // --template <name>
	Repo      string            // --repo <name> (M7 — repository template input)
	Plugins   map[string]string // --plugin key=value (repeatable)
	Output    string            // --output svg|png|jpeg|json
	Filename  string            // --filename <path-or-->; setting this implies --combined.
	Dryrun    bool              // --dryrun
	OutputDir string            // --output-dir <dir> (per-plugin mode)
	Combined  bool              // --combined (opt into single-SVG mode)
	NoEnv     bool              // --no-env (skip INPUT_*/INPUTS env layer; CLI flags only)

	// SkipPrivateRepo is the dedicated CLI surface for the core input
	// `repositories_skip_private`. Equivalent to
	// `--plugin repositories_skip_private=yes` but avoids threading a
	// core-level filter through the per-plugin flag namespace.
	SkipPrivateRepo bool // --skip-private-repo

	// setFlags records which flags were explicitly provided on the
	// command line (populated via fs.Visit in ParseFlags). The unified
	// pipeline uses it to decide whether to override env-provided
	// inputs: a flag without a user-supplied value MUST NOT clobber
	// INPUT_<UPPER>. When nil (CLIFlags built directly in tests),
	// applyFlagsOver falls back to "non-zero value == set" semantics
	// to preserve historical test behaviour.
	setFlags map[string]bool
}

// ParseFlags parses the supplied args (typically os.Args[1:] passed in
// by cmd/metrics-cli after bootstrap flags are stripped) into a
// CLIFlags struct. Errors come from flag.FlagSet (continue on error).
//
// Default values for `template` and `output` are applied so callers
// that read CLIFlags directly (not through applyFlagsOver) still see
// the historical "classic" / "svg" defaults. The setFlags map only
// records flags the user explicitly passed — defaults do NOT participate
// — so the unified pipeline correctly leaves INPUT_TEMPLATE alone when
// the user did not pass --template.
func ParseFlags(args []string) (*CLIFlags, error) {
	cf := &CLIFlags{Plugins: map[string]string{}}
	fs := flag.NewFlagSet("metrics-cli", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cf.Config, "config", "", "YAML config path (action.yml-equivalent inputs)")
	fs.StringVar(&cf.User, "user", "", "GitHub user / org login")
	fs.StringVar(&cf.Repo, "repo", "", "repository name (required when --template=repository)")
	fs.StringVar(&cf.Template, "template", "", "template name (default: classic)")
	fs.StringVar(&cf.Output, "output", "", "output format: svg|png|jpeg|json")
	fs.StringVar(&cf.Filename, "filename", "", "output path; '-' for stdout")
	fs.BoolVar(&cf.Dryrun, "dryrun", false, "skip commit/PR output_action side effects")
	fs.StringVar(&cf.OutputDir, "output-dir", "", "directory for per-plugin SVG output (default mode)")
	fs.BoolVar(&cf.Combined, "combined", false, "render a single combined SVG instead of per-plugin files")
	fs.BoolVar(&cf.NoEnv, "no-env", false, "ignore INPUT_*/INPUTS env vars; resolve inputs from CLI flags only")
	fs.BoolVar(&cf.SkipPrivateRepo, "skip-private-repo", false, "exclude private repositories across all plugins (sets repositories_skip_private=yes)")

	fs.Var(&pluginFlag{m: cf.Plugins}, "plugin", "key=value plugin input (repeatable)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Record only flags the user explicitly passed (fs.Visit skips
	// defaults). This is what the unified pipeline consults so a
	// hybrid `INPUT_TEMPLATE=foo metrics-cli --debug` invocation
	// keeps INPUT_TEMPLATE intact.
	cf.setFlags = map[string]bool{}
	fs.Visit(func(f *flag.Flag) { cf.setFlags[f.Name] = true })

	// Defaults are applied AFTER setFlags is populated so callers that
	// read cf.Template / cf.Output directly still see the historical
	// fallback values. Tests + the unified pipeline check setFlags, not
	// the value, to decide whether a flag was actually supplied.
	if cf.Template == "" {
		cf.Template = "classic"
	}
	if cf.Output == "" {
		cf.Output = "svg"
	}
	return cf, nil
}

// applyFlagsOver overlays CLI-supplied flags onto inputs and returns
// the list of overwritten input keys.
//
// When setFlags is populated (= ParseFlags path), only flags the user
// explicitly passed override env-provided values — this is what makes
// hybrid `INPUT_TOKEN=secret metrics-cli --debug` invocations work
// correctly. When setFlags is nil (= test-constructed CLIFlags), every
// non-zero scalar field counts as "set", preserving the legacy
// ToInvocation semantics those tests depend on.
func (c *CLIFlags) applyFlagsOver(inputs map[string]any) []string {
	if c == nil {
		return nil
	}
	wasSet := func(name string, present bool) bool {
		if c.setFlags == nil {
			return present
		}
		return c.setFlags[name]
	}
	var applied []string
	set := func(key string, value any) {
		inputs[key] = value
		applied = append(applied, key)
	}

	// `--config` is layered as a distinct overlay in assembleInputs, so
	// we do not write to inputs here. The keys it brings carry their
	// own "config" source label in that step.
	_ = wasSet

	if wasSet("user", c.User != "") {
		set("user", c.User)
	}
	if wasSet("repo", c.Repo != "") {
		repo := c.Repo
		if idx := strings.LastIndex(repo, "/"); idx >= 0 {
			slog.Warn("--repo: drop the 'owner/' prefix; canonical form is --user <owner> --repo <name>",
				"got", repo, "using", repo[idx+1:])
			repo = repo[idx+1:]
		}
		set("repo", repo)
	}
	if wasSet("template", c.Template != "") {
		set("template", c.Template)
	}
	if wasSet("output", c.Output != "") {
		set("config_output", c.Output)
	}
	if wasSet("filename", c.Filename != "") {
		set("filename", c.Filename)
	}
	if wasSet("dryrun", c.Dryrun) {
		set("dryrun", true)
	}
	if wasSet("output-dir", c.OutputDir != "") {
		set("output_dir", c.OutputDir)
	}
	if wasSet("combined", c.Combined) {
		set("combined", true)
	}
	if wasSet("skip-private-repo", c.SkipPrivateRepo) {
		set("repositories_skip_private", true)
	}
	// --plugin key=value entries always overlay; each Set call already
	// reflects an explicit user intent at the flag layer. This runs
	// AFTER the dedicated flag above so an explicit
	// `--plugin repositories_skip_private=no` still wins as an escape
	// hatch.
	for k, v := range c.Plugins {
		set(k, v)
	}
	return applied
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

// ToInvocation merges --config YAML, env, and CLI flag overrides into
// a flat `map[string]any` suitable for newInvocation.
//
// Priority (highest first):
//  1. CLI flag (--user, --plugin key=val, ...)
//  2. --config <path>.yaml
//  3. INPUT_<UPPER> / INPUTS env variables (loaded via ParseInputs).
//  4. metadata.yml defaults (applied by ParseInputs).
//
// The unified runtime pipeline (runWith → assembleInputs) does NOT go
// through ToInvocation — it does the layering inline so it can also
// emit per-key source attribution for debug logs. ToInvocation is
// retained because internal tests construct CLIFlags directly and
// assert on the merged map shape.
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

	c.applyFlagsOver(inputs)
	return inputs, nil
}

// ResolveOutputWriter returns the io.Writer + close func for the
// resolved output target. `-` returns os.Stdout (no close), with a
// warning when the format is non-text (png/jpeg). Other filenames are
// os.Create'd after mkdir -p of their parent.
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
