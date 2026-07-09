package render

import (
	"fmt"
)

// PipelineStage is the function signature each decoration / optimization
// step exposes. Stages are composed by Apply.
type PipelineStage struct {
	// Name labels the stage in error wrapping and debug logs.
	Name string
	// Run takes the current SVG and returns the next SVG. Any error
	// is surfaced via Apply's []error return; the returned string is
	// still forwarded to the next stage, so Run must always return its
	// best-effort output — the input unchanged on total failure, or a
	// partially-processed result (best-effort chain).
	Run func(in string) (string, error)
}

// Apply runs the stages in order. The returned string is the final SVG
// after all (successful) stages. Errors are returned alongside in
// registration order, each wrapped with the offending stage's name.
// Apply never panics on stage errors and never short-circuits on a
// failure: the caller decides whether to treat collected errors as
// fatal.
func Apply(stages []PipelineStage, in string) (string, []error) {
	var errs []error
	current := in
	for _, s := range stages {
		if s.Run == nil {
			continue
		}
		next, err := s.Run(current)
		if err != nil {
			errs = append(errs, fmt.Errorf("stage %q: %w", s.Name, err))
			// Keep `next` anyway: stages return their best-effort
			// output alongside the error (e.g. image-inline keeps
			// the successfully inlined images even when other
			// fetches failed — FR-018).
		}
		current = next
	}
	return current, errs
}
