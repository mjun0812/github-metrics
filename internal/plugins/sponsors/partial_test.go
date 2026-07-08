package sponsors_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// renderPartial drives the classic sponsors partial against a Data that
// carries the supplied Result under the "sponsors" slug.
func renderPartial(t *testing.T, r *sponsors.Result) string {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin(sponsors.Name, r)
	pc := &templates.PartialContext{
		Data:   data,
		Inputs: map[string]any{"plugin_sponsors": true},
	}
	frag, _, err := sponsors.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return frag
}

// TestPartial_ZeroSponsors_RendersHeadingAndAbout is the regression guard
// for #451: even with zero active sponsors and no goal, the partial must
// render the section wrapper, the "Sponsor Me!" heading and the `about`
// bio markdown — matching docs/reference_examples/metrics.plugin.sponsors.svg.
func TestPartial_ZeroSponsors_RendersHeadingAndAbout(t *testing.T) {
	t.Parallel()
	r := &sponsors.Result{
		Sections: []string{"goal", "list", "about"},
		Sponsors: []sponsors.Sponsor{}, // zero active sponsors
		Past:     []sponsors.Sponsor{},
		Title:    "Sponsor Me!",
		User:     "mjun0812",
		Size:     24,
		About: "Hi! I'm Junya Morioka, a Japanese developer.\n" +
			"If I have been of any help, I would be happy if you could sponsor me!\n\n" +
			"[📝 About Me](https://mjunya.com/about/)",
	}
	frag := renderPartial(t, r)

	if frag == "" {
		t.Fatal("partial rendered empty fragment with zero sponsors; expected heading + about")
	}
	checks := []string{
		`<section data-section="sponsors">`,
		`<h2 class="field">`,
		`Sponsor Me!`,
		`<section class="sponsors goal">`,      // goal/list section wrapper
		`<div class="markdown">`,               // about markdown body
		`Hi! I&#39;m Junya Morioka`,            // escaped bio text node
		`<a href="https://mjunya.com/about/">`, // rendered markdown link (unescaped markup)
	}
	for _, want := range checks {
		if !strings.Contains(frag, want) {
			t.Errorf("rendered fragment missing %q\n--- fragment ---\n%s", want, frag)
		}
	}
}

// TestPartial_Skipped_RendersNothing confirms the partial still short-
// circuits to an empty string when the upstream-style mode gate marked the
// Result Skipped (e.g. repository mode). This is the one path that should
// blank the card.
func TestPartial_Skipped_RendersNothing(t *testing.T) {
	t.Parallel()
	r := &sponsors.Result{Skipped: true, SkippedReason: "repository mode"}
	if frag := renderPartial(t, r); frag != "" {
		t.Errorf("Skipped Result should render empty; got %q", frag)
	}
}

// TestPartial_EmptyAbout_StillRendersHeading verifies the section renders
// even when the GitHub Sponsors listing has no bio (About == ""): the
// heading and the goal section wrapper still appear, so the card is never
// the empty height=8 wrapper.
func TestPartial_EmptyAbout_StillRendersHeading(t *testing.T) {
	t.Parallel()
	r := &sponsors.Result{
		Sections: []string{"goal", "list", "about"},
		Sponsors: []sponsors.Sponsor{},
		Past:     []sponsors.Sponsor{},
		Title:    "Sponsor Me!",
		User:     "mjun0812",
		Size:     24,
	}
	frag := renderPartial(t, r)
	for _, want := range []string{
		`<section data-section="sponsors">`,
		`Sponsor Me!`,
		`<section class="sponsors goal">`,
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("rendered fragment missing %q\n--- fragment ---\n%s", want, frag)
		}
	}
}
