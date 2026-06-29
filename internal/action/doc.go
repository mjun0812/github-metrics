// Package action implements the entry-point logic for the metrics-cli
// binary.
//
// Run(ctx, args) is the single dispatch function (the pre-v3.0
// Action-mode / CLI-mode split was collapsed in #646). Every invocation
// reads CLI flag args AND env vars (INPUT_<UPPER> + INPUTS JSON); CLI
// flags take precedence on conflict. Workflows that set `with:` keys
// still work unchanged because the runner exposes them as INPUT_<UPPER>
// env vars, and the pipeline still reads those — there is just no
// longer a separate Action codepath.
//
// The runtime pipeline assembles inputs via assembleInputs(), builds an
// *Invocation, runs engine.Compute, writes the rendered output to
// OutputDir/OutputFilename (or stdout when --filename -), and optionally
// runs the configured output_action (commit / pull-request[-merge|
// -squash|-rebase]).
//
// The package is internal because there is no expectation of external
// programmatic use — the metrics-cli binary is the only consumer.
package action
