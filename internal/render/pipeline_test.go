package render

import (
	"errors"
	"strings"
	"testing"
)

// TestApply_AllSucceed: 3-stage chain, every stage returns the next
// string. Final output is the last stage's result; errors slice is
// empty.
func TestApply_AllSucceed(t *testing.T) {
	t.Parallel()

	stages := []PipelineStage{
		{Name: "uc", Run: func(in string) (string, error) { return strings.ToUpper(in), nil }},
		{Name: "exc", Run: func(in string) (string, error) { return in + "!", nil }},
		{Name: "ws", Run: func(in string) (string, error) { return " " + in + " ", nil }},
	}

	got, errs := Apply(stages, "hello")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if got != " HELLO! " {
		t.Errorf("got %q, want %q", got, " HELLO! ")
	}
}

// TestApply_StageErrorKeepsPartialOutput: when a middle stage fails, its
// best-effort output is still forwarded to the downstream stage (e.g.
// image-inline keeps successfully inlined images even when other fetches
// failed), and the error is aggregated.
func TestApply_StageErrorKeepsPartialOutput(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated stage failure")
	stages := []PipelineStage{
		{Name: "first", Run: func(in string) (string, error) { return in + "+first", nil }},
		{Name: "broken", Run: func(in string) (string, error) {
			// A failing stage returns its best-effort partial
			// output; Apply must forward it downstream.
			return in + "+partial", sentinel
		}},
		{Name: "third", Run: func(in string) (string, error) {
			// Expect "x+first+partial" as input.
			return in + "+third", nil
		}},
	}

	got, errs := Apply(stages, "x")
	if got != "x+first+partial+third" {
		t.Errorf("got %q, want %q (third must see the broken stage's best-effort output)", got, "x+first+partial+third")
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	if !errors.Is(errs[0], sentinel) {
		t.Errorf("errs[0] not wrapping sentinel: %v", errs[0])
	}
	if !strings.Contains(errs[0].Error(), `stage "broken"`) {
		t.Errorf("errs[0] missing stage-name prefix: %v", errs[0])
	}
}

// TestApply_EmptyStages: passthrough behavior when no stages are
// supplied — Apply returns the input unchanged with no errors.
func TestApply_EmptyStages(t *testing.T) {
	t.Parallel()

	got, errs := Apply(nil, "untouched")
	if got != "untouched" {
		t.Errorf("got %q, want untouched", got)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	got, errs = Apply([]PipelineStage{}, "still-untouched")
	if got != "still-untouched" {
		t.Errorf("got %q, want still-untouched", got)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

// TestApply_NilRunSkipped: a stage with nil Run is treated as a no-op
// rather than a nil-deref crash. This matters for the M3 dispatch
// where we may build a stages slice incrementally.
func TestApply_NilRunSkipped(t *testing.T) {
	t.Parallel()

	stages := []PipelineStage{
		{Name: "real", Run: func(in string) (string, error) { return in + "+real", nil }},
		{Name: "nil-run"},
		{Name: "second-real", Run: func(in string) (string, error) { return in + "+second", nil }},
	}

	got, errs := Apply(stages, "x")
	if got != "x+real+second" {
		t.Errorf("got %q, want x+real+second (nil Run should be skipped)", got)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

// TestApply_IdentityStage: a stage that returns its input verbatim
// does not introduce errors and does not interfere with the chain.
func TestApply_IdentityStage(t *testing.T) {
	t.Parallel()

	stages := []PipelineStage{
		{Name: "identity", Run: func(in string) (string, error) { return in, nil }},
		{Name: "append", Run: func(in string) (string, error) { return in + "!", nil }},
	}

	got, errs := Apply(stages, "hello")
	if got != "hello!" {
		t.Errorf("got %q, want hello!", got)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}
