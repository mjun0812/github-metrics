package engine

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// stubTemplate is a minimal templates.Template used to keep the
// dispatcher self-contained: it accepts every (account, format) pair
// and emits a fixed SVG when Run is invoked.
type stubTemplate struct{}

func (stubTemplate) Name() string { return "stub" }
func (stubTemplate) Metadata() *templates.TemplateMetadata {
	return &templates.TemplateMetadata{Formats: []string{"svg"}}
}
func (stubTemplate) FS() fs.FS { return fstest.MapFS{} }
func (stubTemplate) Check(_ map[string]any, _, _ string) error {
	return nil
}

func (stubTemplate) Run(_ context.Context, _ *templates.PartialContext) (string, error) {
	return `<svg id="metrics-end"></svg>`, nil
}

// TestDispatch_FakeRenderer_PNG injects a FakeRenderer, calls
// dispatchOutput directly with Format=png, and asserts the renderer's
// PNG bytes flow through unchanged. Critically: the M2-era warn log
// "png conversion lands in M3" must NOT appear (FR-009).
func TestDispatch_FakeRenderer_PNG(t *testing.T) {
	t.Parallel()

	var sink bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	deps := Deps{Logger: logger, Render: &render.FakeRenderer{}}

	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "png", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: logger},
		&Result{},
	)
	if err != nil {
		t.Fatalf("dispatchOutput(png): %v", err)
	}
	if mime != "image/png" {
		t.Errorf("MIME = %q, want image/png", mime)
	}
	if len(out) < 8 {
		t.Fatalf("Output too short: %d bytes", len(out))
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, want := range pngMagic {
		if out[i] != want {
			t.Errorf("Output[%d] = %#x, want %#x (PNG magic)", i, out[i], want)
		}
	}
	if strings.Contains(sink.String(), "png conversion lands in M3") {
		t.Errorf("M2 warn log leaked into M3 output: %s", sink.String())
	}
}

// TestDispatch_FakeRenderer_JPEG mirrors the PNG test for jpeg.
func TestDispatch_FakeRenderer_JPEG(t *testing.T) {
	t.Parallel()

	deps := Deps{Logger: slog.Default(), Render: &render.FakeRenderer{}}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "jpeg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		&Result{},
	)
	if err != nil {
		t.Fatalf("dispatchOutput(jpeg): %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", mime)
	}
	if len(out) < 3 || out[0] != 0xFF || out[1] != 0xD8 || out[2] != 0xFF {
		t.Errorf("Output prefix %#x %#x %#x, want JPEG SOI", out[0], out[1], out[2])
	}
}

// TestDispatch_SVG_SkipsRenderer confirms the #409 Phase C contract: the
// SVG branch returns the post-decoration SVG verbatim WITHOUT invoking
// the Renderer (the template already wrote a Go-computed height, so no
// browser measurement pass is needed). A FakeRenderer wired to error on
// "svg" would surface that error if it were called; the run staying clean
// proves the renderer is bypassed.
func TestDispatch_SVG_SkipsRenderer(t *testing.T) {
	t.Parallel()

	deps := Deps{
		Logger: slog.Default(),
		Render: &render.FakeRenderer{ErrOnConvert: map[string]error{"svg": errors.New("renderer must not be called for svg")}},
	}
	res := &Result{}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		res,
	)
	if err != nil {
		t.Fatalf("dispatchOutput(svg): %v", err)
	}
	if mime != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml", mime)
	}
	if !bytes.Contains(out, []byte(`<svg id="metrics-end">`)) {
		t.Errorf("Output should contain the stub template's SVG verbatim; got %q", string(out))
	}
	if len(res.Errors) != 0 {
		t.Errorf("svg path must not touch the Renderer, but got %d error(s): %v", len(res.Errors), res.Errors)
	}
}

// TestDispatch_RendererError_PNG_NilOutput verifies the FR-018
// fallback: when the Renderer returns an error on the PNG branch,
// dispatchOutput records the error in res.Errors and returns
// (nil, "").
func TestDispatch_RendererError_PNG_NilOutput(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated renderer failure")
	deps := Deps{
		Logger: slog.Default(),
		Render: &render.FakeRenderer{ErrOnConvert: map[string]error{"png": sentinel}},
	}

	res := &Result{}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "png", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		res,
	)
	if err != nil {
		t.Fatalf("dispatchOutput should not return a top-level error on Renderer failure: %v", err)
	}
	if out != nil {
		t.Errorf("Output should be nil on PNG Renderer failure, got %d bytes", len(out))
	}
	if mime != "" {
		t.Errorf("MIME should be empty on PNG Renderer failure, got %q", mime)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error in res.Errors, got %d", len(res.Errors))
	}
	if !errors.Is(res.Errors[0], sentinel) {
		t.Errorf("res.Errors[0] = %v, want wrap of sentinel", res.Errors[0])
	}
}

// TestDispatch_SVG_RendererInitIgnored pins the #409 Phase C corollary
// of the #666 contract: because the svg path no longer constructs a
// Renderer, a broken METRICS_RESVG_PATH can no longer degrade an SVG
// render. The run must succeed with the decorated SVG and record no
// error (there is nothing to fail).
func TestDispatch_SVG_RendererInitIgnored(t *testing.T) {
	t.Setenv("METRICS_RESVG_PATH", "/nonexistent/resvg-binary")

	deps := Deps{Logger: slog.Default()} // Render nil → would lazy-init for png/jpeg

	res := &Result{}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		res,
	)
	if err != nil {
		t.Fatalf("dispatchOutput: %v", err)
	}
	if mime != "image/svg+xml" || !bytes.Contains(out, []byte(`<svg id="metrics-end">`)) {
		t.Fatalf("svg output expected; mime=%q out=%q", mime, string(out))
	}
	if len(res.Errors) != 0 {
		t.Errorf("svg path must not init a Renderer, but got %d error(s): %v", len(res.Errors), res.Errors)
	}
}

// TestDispatch_UnknownFormat keeps the M2 contract: bogus Format
// surfaces as *UnsupportedFormatError.
func TestDispatch_UnknownFormat(t *testing.T) {
	t.Parallel()
	deps := Deps{Logger: slog.Default()}
	_, _, err := dispatchOutput(
		context.Background(),
		Request{Format: "bogus", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		&Result{},
	)
	var ufe *xerrors.UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Errorf("expected *UnsupportedFormatError, got %T: %v", err, err)
	}
}
