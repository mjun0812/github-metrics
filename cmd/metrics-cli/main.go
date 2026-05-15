// Package main provides the metrics-cli binary entrypoint.
//
// This is the standalone CLI surface for the Go port of
// lowlighter/metrics. In M1 (project foundation) it exposes --help and
// --version, initializes the foundational logger and a signal-aware
// context, and emits a startup banner. Full CLI flags ship in task
// T-117 (M6).
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
	"syscall"

	"github.com/mjun0812/github-metrics/internal/logger"
)

// version is populated at build time via -ldflags "-X main.version=...".
var version = "dev"

const binaryName = "metrics-cli"

const usageText = `metrics-cli: standalone CLI for github-metrics.

Usage:
  metrics-cli [flags]

Flags:
  -h, --help        Show this help message and exit.
      --version     Print the version string and exit.
      --debug       Enable debug-level logging.
      --log-format  Logging format: "json" (default) or "text".

In M1 this binary only supports the flags above. Full CLI flags
(--user, --template, --token, --plugin, --output, ...) are
implemented in T-117 (Phase M6).`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
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

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if showVersion {
		_, _ = fmt.Fprintln(stdout, version)
		return nil
	}

	// Wire the foundational logger and signal-aware context. Downstream
	// phases (M6 T-117) take this context into the full CLI pipeline.
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
	_ = ctx // not yet consumed in M1; reserved for T-117.

	banner(stdout)
	_, _ = fmt.Fprintln(stdout, usageText)
	return nil
}

func banner(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s %s (go %s, %s/%s)\n",
		binaryName, version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
