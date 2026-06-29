// Package main provides the metrics-cli binary entrypoint.
//
// This is the unified GitHub Action / CLI surface for the Go port of
// lowlighter/metrics. After the v3.0 mode-unification (#646) there is
// a single dispatch path: the binary always reads INPUT_<UPPER> /
// INPUTS env vars first and then layers any CLI flags on top (flags
// win on conflict). The pre-v3.0 GITHUB_ACTIONS=true switch is no
// longer consulted — the GitHub Actions runner still sets it, but the
// binary ignores it because the runner-supplied INPUT_<UPPER> env vars
// already feed the same unified pipeline. Pass `--no-env` to suppress
// the env layer for local debug runs.
//
// The legacy --help / --version / --debug / --log-format bootstrap
// flags from M1 stay supported so existing wrappers do not break.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/mjun0812/github-metrics/internal/action"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/logger"
)

// version is populated at build time via -ldflags "-X main.version=...".
// It is mirrored into engine.SetVersion so every place that emits a
// version string (metadata footer, User-Agent, banner) reads from the
// same source of truth.
var version = "dev"

func init() {
	engine.SetVersion(version)
}

const binaryName = "metrics-cli"

const usageText = `metrics-cli: unified GitHub Action / CLI entry point for github-metrics.

Usage:
  metrics-cli [flags]

The binary reads inputs from two layers on every invocation; later
layers override earlier ones:

  1. Env vars   INPUT_<UPPER> (GitHub Actions runner) and INPUTS (JSON).
  2. CLI flags  --user / --template / --plugin key=value / ...

Pass --no-env to skip layer 1 entirely (useful for local debug runs).
The GitHub token is read from inputs["token"] (= INPUT_TOKEN) or the
GITHUB_TOKEN env var — there is no --token / --token-env flag.

Common invocations:

  # GitHub Actions runner — the workflow's with: keys arrive as
  # INPUT_<UPPER> env vars and feed the unified pipeline:
  INPUT_USER=octocat INPUT_TOKEN=<PAT> metrics-cli

  # Local CLI — same pipeline, driven by flags (and GITHUB_TOKEN):
  GITHUB_TOKEN=$(gh auth token) \
    metrics-cli --user <login> --template classic [--config inputs.yaml]
                [--plugin key=value ...] [--output svg|png|jpeg|json]
                [--filename <path-or-->] [--dryrun]
                [--output-dir <dir>] [--combined] [--plugins a,b,c]

  # Hybrid — token from a workflow secret, --debug from the run: step:
  GITHUB_TOKEN=<PAT> metrics-cli --debug --user octocat

Common flags:
  -h, --help        Show this help message and exit.
      --version     Print the version string and exit.
      --debug       Enable debug-level logging.
      --log-format  Logging format: "json" (default) or "text".

See https://github.com/mjun0812/github-metrics for full documentation.`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer, _ []string) error {
	// Handle the bootstrap flags (--help / --version / --debug /
	// --log-format) first. These are kept outside action.Run so the
	// binary stays usable as a diagnostic tool even when full
	// pipeline inputs are unavailable.
	cliArgs, bootArgs := splitBootstrapArgs(args)

	fs := flag.NewFlagSet(binaryName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		showVersion bool
		debug       bool
		logFormat   string
	)
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&debug, "debug", false, "enable debug-level logging")
	fs.StringVar(&logFormat, "log-format", "json", `log format: "json" or "text"`)
	fs.Usage = func() { _, _ = fmt.Fprintln(stdout, usageText) }

	if err := fs.Parse(bootArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if showVersion {
		_, _ = fmt.Fprintln(stdout, version)
		return nil
	}

	// Configure foundational logger before handing off to the action
	// pipeline so all downstream logs flow through one handler.
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	logger.SetDefault(logger.Options{
		Format: logger.Format(logFormat),
		Level:  level,
		Writer: stderr,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// No-arg invocation = print banner + usage (legacy M1 behavior).
	// Anything else flows into the unified action.Run pipeline. We do
	// NOT branch on GITHUB_ACTIONS — runner-supplied INPUT_<UPPER> env
	// vars already feed the same pipeline whether or not the workflow
	// step passes additional CLI args.
	if len(cliArgs) == 0 && !hasActionInputs(os.Environ()) {
		banner(stdout)
		_, _ = fmt.Fprintln(stdout, usageText)
		return nil
	}
	return action.Run(ctx, cliArgs)
}

// splitBootstrapArgs separates the M1 bootstrap flags (--help, -h,
// --version, --debug, --log-format) from the rest of the args so the
// bootstrap fs only sees what it understands and the remaining args
// flow into action.Run's own flag set.
func splitBootstrapArgs(args []string) (cliArgs, bootArgs []string) {
	bootstrap := map[string]bool{
		"--help": true, "-h": true,
		"--version":    true,
		"--debug":      true,
		"--log-format": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		// --log-format=text  ←single arg
		if strings.HasPrefix(a, "--log-format=") {
			bootArgs = append(bootArgs, a)
			continue
		}
		// --log-format text  ←two-arg form
		if a == "--log-format" && i+1 < len(args) {
			bootArgs = append(bootArgs, a, args[i+1])
			i++
			continue
		}
		if bootstrap[a] {
			bootArgs = append(bootArgs, a)
			continue
		}
		cliArgs = append(cliArgs, a)
	}
	return cliArgs, bootArgs
}

// hasActionInputs reports whether any non-empty INPUT_<UPPER> / INPUTS
// env var is present. The unified pipeline treats those vars as a
// valid source of inputs (the GitHub Actions runner populates them
// from `with:` keys), so we MUST hand control to action.Run even when
// the invocation has no CLI args — otherwise a workflow that only
// sets `with:` (no extra `run:` step) would never reach the pipeline.
//
// We deliberately do NOT consult GITHUB_ACTIONS=true (#646): the
// presence of actual inputs is what matters, not the runner marker.
// Empty INPUT_* values are ignored because the runner emits
// `INPUT_FOO=` for every workflow input regardless of whether the
// user supplied a value, so the presence of an empty entry is not
// evidence that the workflow intends to drive the pipeline.
func hasActionInputs(env []string) bool {
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "INPUT_"):
			if idx := strings.IndexByte(kv, '='); idx >= 0 && idx < len(kv)-1 {
				return true
			}
		case strings.HasPrefix(kv, "INPUTS="):
			if len(kv) > len("INPUTS=") {
				return true
			}
		}
	}
	return false
}

func banner(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s %s (go %s, %s/%s)\n",
		binaryName, version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
