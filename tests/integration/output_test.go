package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"

	// Side-effect import: registers classic with templates registry.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// TestComputeJSON_DefaultFromTemplate covers the truth-table row:
//
//	Format="", Template="" → JSON because no template means no defaulted
//	format and engine falls back to application/json.
func TestComputeJSON_DefaultFromTemplate(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login: "octocat",
		// Format intentionally empty + no Template.
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "application/json" {
		t.Fatalf("MIME = %q, want application/json", res.MIME)
	}
	if !json.Valid(res.Output) {
		t.Fatalf("Output not valid JSON: %s", res.Output)
	}
}

// TestComputeJSON_DefaultWhenNoopTemplate exercises the noop short
// circuit: Template == "noop" leaves tmpl==nil so the dispatcher falls
// back to "json".
func TestComputeJSON_DefaultWhenNoopTemplate(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
	}, deps)
	if err != nil {
		t.Fatalf("Compute(noop): %v", err)
	}
	if res.MIME != "application/json" {
		t.Fatalf("MIME = %q, want application/json", res.MIME)
	}
}

// TestComputeJSON_DefaultFromClassicMetadata covers the truth-table row
// where Format is empty and a classic template is wired. Classic
// metadata advertises `svg` as the first format so the dispatcher
// should pick svg, not json.
func TestComputeJSON_DefaultFromClassicMetadata(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		// Format intentionally empty so engine resolves to svg.
	}, deps)
	if err != nil {
		t.Fatalf("Compute(classic,default): %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}
}

// TestComputeSVG_Classic exercises the explicit svg path through the
// classic template.
func TestComputeSVG_Classic(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
	}, deps)
	if err != nil {
		t.Fatalf("Compute(svg): %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}
	if !strings.HasPrefix(string(res.Output), "<svg") {
		t.Fatalf("Output should start with <svg; got %.80s", string(res.Output))
	}
}

// TestComputePNG_M3ReturnsImageBytes validates the M3 PNG branch: the
// FakeRenderer (injected by newEngineDeps) emits valid PNG bytes plus
// the matching image/png MIME, and the M2-era "chromedp conversion
// lands in M3" warn log is no longer produced (FR-009).
func TestComputePNG_M3ReturnsImageBytes(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	var sink bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	deps.Logger = logger

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "png",
	}, deps)
	if err != nil {
		t.Fatalf("Compute(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Fatalf("MIME = %q, want image/png", res.MIME)
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(res.Output) < len(pngMagic) || !bytes.HasPrefix(res.Output, pngMagic) {
		t.Fatalf("Output should start with PNG magic; got %x", res.Output[:min(8, len(res.Output))])
	}
	if strings.Contains(sink.String(), "chromedp conversion lands in M3") {
		t.Errorf("M2 warn log leaked into M3 output: %s", sink.String())
	}
}

// TestComputeUnknownFormat_Error checks the default arm of the
// dispatcher: an unknown format must return an UnsupportedFormatError
// that callers can detect via errors.As.
func TestComputeUnknownFormat_Error(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "bogus",
	}, deps)
	if err == nil {
		t.Fatal("Compute(bogus) should return an error")
	}
	var ufe *xerrors.UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnsupportedFormatError, got %T: %v", err, err)
	}
}

// TestComputeSVG_NoTemplate_Errors covers the row where the caller asks
// for an svg/png/jpeg output without registering a template. The
// dispatcher must reject early with InputError{Field:"template"} —
// classic's Check is bypassed because there is no template to call.
func TestComputeSVG_NoTemplate_Errors(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:  "octocat",
		Format: "svg",
		// Template intentionally empty.
	}, deps)
	if err == nil {
		t.Fatal("Compute(svg,no-template) should return an error")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Fatalf("expected *InputError, got %T: %v", err, err)
	}
	if ie.Field != "template" {
		t.Errorf("InputError.Field = %q, want template", ie.Field)
	}
}
