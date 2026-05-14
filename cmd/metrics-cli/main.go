// Package main provides the metrics-cli binary entrypoint.
//
// This is the standalone CLI surface for the Go port of
// lowlighter/metrics. In M1 (project foundation) it only exposes a
// --help flag; full CLI flags ship in task T-117 (M6).
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is populated at build time via -ldflags "-X main.version=...".
var version = "dev"

const usageText = `metrics-cli: standalone CLI for github-metrics.

Usage:
  metrics-cli [flags]

Flags:
  -h, --help        Show this help message and exit.
      --version     Print the version string and exit.

In M1 this binary only supports the flags above. Full CLI flags
(--user, --template, --token, --plugin, --output, ...) are
implemented in T-117 (Phase M6).`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("metrics-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() { fmt.Fprintln(stdout, usageText) }

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return nil
	}
	// Default behavior in M1: print usage and exit 0.
	fmt.Fprintln(stdout, usageText)
	return nil
}
