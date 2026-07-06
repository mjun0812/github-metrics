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

	"github.com/mjun0812/github-metrics/internal/config"
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
func (stubTemplate) Metadata() *config.TemplateMetadata {
	return &config.TemplateMetadata{Formats: []string{"svg"}}
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
// "chromedp conversion lands in M3" must NOT appear (FR-009).
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
	if strings.Contains(sink.String(), "chromedp conversion lands in M3") {
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

// TestDispatch_FakeRenderer_SVG_Passthrough confirms that the SVG
// branch routes the post-decoration SVG through the renderer (which
// the FakeRenderer passes through verbatim).
func TestDispatch_FakeRenderer_SVG_Passthrough(t *testing.T) {
	t.Parallel()

	deps := Deps{Logger: slog.Default(), Render: &render.FakeRenderer{}}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: deps.Logger},
		&Result{},
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
}

// TestDispatch_RendererError_PNG_NilOutput verifies the FR-018
// fallback: when the Renderer returns an error on the PNG branch,
// dispatchOutput records the error in res.Errors and returns
// (nil, "").
func TestDispatch_RendererError_PNG_NilOutput(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated chromedp failure")
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

// TestDispatch_RendererError_SVG_PassthroughDecoratedSVG verifies the
// SVG branch's softer fallback: a Renderer error still returns the
// (decoration-pass) SVG bytes so the README badge does not break.
func TestDispatch_RendererError_SVG_PassthroughDecoratedSVG(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated chromedp failure")
	deps := Deps{
		Logger: slog.Default(),
		Render: &render.FakeRenderer{ErrOnConvert: map[string]error{"svg": sentinel}},
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
		t.Fatalf("dispatchOutput should not return a top-level error: %v", err)
	}
	if mime != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml (SVG fallback path)", mime)
	}
	if !bytes.Contains(out, []byte(`<svg id="metrics-end">`)) {
		t.Errorf("Output should contain the un-resized SVG; got %q", string(out))
	}
	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error in res.Errors, got %d", len(res.Errors))
	}
}

// TestDispatch_RendererError_SVG_LogsWarn pins the #666 fix: a Resize
// failure on the svg path must emit a Warn log in addition to
// res.Errors — without it the run exits 0 with an untrimmed SVG and
// zero log evidence (this hid the bookworm chromium 150 breakage).
func TestDispatch_RendererError_SVG_LogsWarn(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	sentinel := errors.New("simulated chromedp failure")
	deps := Deps{
		Logger: logger,
		Render: &render.FakeRenderer{ErrOnConvert: map[string]error{"svg": sentinel}},
	}

	res := &Result{}
	if _, _, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: logger},
		res,
	); err != nil {
		t.Fatalf("dispatchOutput: %v", err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "resize failed") {
		t.Errorf("expected 'resize failed' Warn in logs; got:\n%s", logs)
	}
	if !strings.Contains(logs, "simulated chromedp failure") {
		t.Errorf("expected underlying error in logs; got:\n%s", logs)
	}
}

// TestDispatch_RendererInitError_SVG_LogsWarn pins the same #666
// visibility contract for the lazy-renderer init branch: deps.Render
// nil + a bogus METRICS_CHROME_PATH makes obtainRenderer fail, which
// must Warn instead of degrading silently.
func TestDispatch_RendererInitError_SVG_LogsWarn(t *testing.T) {
	t.Setenv("METRICS_CHROME_PATH", "/nonexistent/chromium-binary")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	deps := Deps{Logger: logger} // Render nil → lazy init path

	res := &Result{}
	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg", Template: "stub"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		&templates.PartialContext{Logger: logger},
		res,
	)
	if err != nil {
		t.Fatalf("dispatchOutput: %v", err)
	}
	if mime != "image/svg+xml" || !bytes.Contains(out, []byte(`<svg id="metrics-end">`)) {
		t.Fatalf("svg fallback expected; mime=%q out=%q", mime, string(out))
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "renderer init failed") {
		t.Errorf("expected 'renderer init failed' Warn in logs; got:\n%s", logs)
	}
	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error in res.Errors, got %d", len(res.Errors))
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
