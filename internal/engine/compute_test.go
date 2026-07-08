// Package engine_test — compute_test.go covers Compute branches that
// the bench file does not exercise: Die=true, JSON output format,
// empty Login error, template not found error.
package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"

	// Side-effect imports register the classic template + core plugin so
	// engine.Compute can resolve them (same as bench_full_test.go).
	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// TestCompute_EmptyLogin_ReturnsInputError asserts that Compute
// returns an *InputError (not nil) when the Login field is empty.
func TestCompute_EmptyLogin_ReturnsInputError(t *testing.T) {
	t.Parallel()

	deps := newBenchDeps(t)
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "",
		Template: "classic",
		Format:   "json",
		Inputs:   map[string]any{"use_mocked_data": true},
	}, deps)
	if err == nil {
		t.Fatal("expected error for empty Login, got nil")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Errorf("expected *InputError, got %T: %v", err, err)
	}
}

// TestCompute_TemplateNotFound_ReturnsError asserts that Compute
// returns an error when a non-existent template name is requested.
func TestCompute_TemplateNotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	deps := newBenchDeps(t)
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "no-such-template-xyz",
		Format:   "json",
		Inputs:   map[string]any{"use_mocked_data": true},
	}, deps)
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
}

// TestCompute_JSON_Format tests that Compute with Format="json" returns
// a valid JSON body and "application/json" MIME. Uses Format="json" to
// avoid the SVG/PNG render path.
func TestCompute_JSON_Format(t *testing.T) {
	t.Parallel()

	deps := newBenchDeps(t)
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
		Format:   "json",
		Inputs:   map[string]any{"use_mocked_data": true},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(json): %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.MIME != "application/json" {
		t.Errorf("MIME = %q, want application/json", res.MIME)
	}
	if !json.Valid(res.Output) {
		t.Errorf("Output is not valid JSON: %s", res.Output)
	}
}

// TestCompute_Noop_NoTemplate_JSONDefault asserts that when Template is
// "noop" and Format is "", the dispatcher defaults to json output.
func TestCompute_Noop_NoTemplate_JSONDefault(t *testing.T) {
	t.Parallel()

	deps := newBenchDeps(t)
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
		Format:   "",
		Inputs:   map[string]any{"use_mocked_data": true},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(noop,empty format): %v", err)
	}
	if res.MIME != "application/json" {
		t.Errorf("MIME = %q, want application/json (json default)", res.MIME)
	}
}
