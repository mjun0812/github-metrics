// Package action implements the GitHub Action / CLI entry-point logic
// for the metrics-cli binary.
//
// The package exposes two top-level dispatch functions:
//
//   - Run(ctx) — Action mode. Reads INPUTS / INPUT_<UPPER> env vars,
//     builds an *Invocation, runs engine.Compute, writes the output to
//     /renders/<filename>, and optionally runs the configured
//     output_action (commit / pull-request[-merge|-squash|-rebase]).
//   - RunCLI(ctx, args) — CLI mode. Parses CLI flags, optionally loads
//     a YAML config file, builds the same *Invocation shape, and
//     follows the same downstream pipeline. Action and CLI modes share
//     the same input map / engine.Request / Committer paths so they
//     produce byte-identical output for equivalent inputs (spec SC-009).
//
// The package is internal because there is no expectation of external
// programmatic use — the metrics-cli binary is the only consumer.
package action
