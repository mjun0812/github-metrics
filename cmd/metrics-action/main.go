// Package main provides the metrics-action binary entrypoint.
//
// This is the GitHub Action / CLI surface for the Go port of
// lowlighter/metrics. The binary dispatches based on the
// `GITHUB_ACTIONS=true` env var: when set, the Action path
// (action.Run) reads INPUTS / INPUT_<UPPER> env vars; otherwise the
// CLI path (action.RunCLI) parses os.Args flags. The legacy
// --help / --version / --debug / --log-format flags from M1 stay
// supported so existing wrappers do not break.
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

const binaryName = "metrics-action"

const usageText = `metrics-action: GitHub Action / CLI entry point for github-metrics.

Usage:
  metrics-action [flags]

Action mode (set automatically by the GitHub Actions runner):
  GITHUB_ACTIONS=true INPUT_USER=octocat INPUT_TOKEN=<PAT> metrics-action

CLI mode (direct invocation):
  metrics-action --user <login> --template classic [--config inputs.yaml]
                 [--token <PAT> | --token-env <ENV_NAME>]
                 [--plugin key=value ...] [--output svg|png|jpeg|json]
                 [--filename <path-or-->] [--dryrun]

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

func run(args []string, stdout, stderr io.Writer, env []string) error {
	// Handle the no-arg help-friendly bootstrap flags (--help / --version /
	// --debug / --log-format) first. These are kept outside action.Run /
	// action.RunCLI so the binary stays usable as a diagnostic tool even
	// when full Action / CLI inputs are unavailable.
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

	// Dispatch: Action mode when GitHub Actions runner sets
	// GITHUB_ACTIONS=true; CLI mode otherwise. The CLI path receives
	// the user-supplied args after the bootstrap flags are removed so
	// `--user`, `--template`, `--plugin key=value` etc. land in the
	// flag.FlagSet that action.RunCLI defines.
	if envValue(env, "GITHUB_ACTIONS") == "true" {
		return action.Run(ctx)
	}

	// No-arg CLI invocation = print banner + usage (legacy M1 behavior).
	if len(cliArgs) == 0 {
		banner(stdout)
		_, _ = fmt.Fprintln(stdout, usageText)
		return nil
	}
	return action.RunCLI(ctx, cliArgs)
}

// splitBootstrapArgs separates the M1 bootstrap flags (--help, -h,
// --version, --debug, --log-format) from the rest of the args so the
// bootstrap fs only sees what it understands and the remaining args
// flow into action.RunCLI's own flag set.
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

// envValue looks up `name=value` in the supplied env slice (matching
// os.Environ()'s format). Returns "" when missing — same semantics as
// os.Getenv but operating on a passed-in slice for testability.
func envValue(env []string, name string) string {
	prefix := name + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func banner(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s %s (go %s, %s/%s)\n",
		binaryName, version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
