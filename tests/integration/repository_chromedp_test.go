//go:build chromedp

package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

// TestRepositoryTemplate_PNG_MagicNumber (T030): chromedp-gated test
// that exercises the M3 render pipeline through the repository
// template and asserts the produced bytes start with the PNG
// signature `\x89PNG\r\n\x1a\n`.
//
// Runs only with `go test -tags=chromedp` and skips when
// METRICS_CHROME_PATH is not set.
func TestRepositoryTemplate_PNG_MagicNumber(t *testing.T) {
	t.Parallel()
	if os.Getenv("METRICS_CHROME_PATH") == "" {
		t.Skip("METRICS_CHROME_PATH not set; chromedp test requires a chromium binary")
	}
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	browser, err := render.New(render.BrowserOpts{
		ExecPath: os.Getenv("METRICS_CHROME_PATH"),
	})
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}
	defer func() { _ = browser.Close() }()
	deps.Render = browser

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "png",
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/png" {
		t.Errorf("MIME = %q, want image/png", res.MIME)
	}
	if len(res.Output) < 8 {
		t.Fatalf("PNG output too short: %d bytes", len(res.Output))
	}
	sig := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	for i, want := range sig {
		if res.Output[i] != want {
			t.Errorf("PNG signature byte %d = %#x, want %#x", i, res.Output[i], want)
		}
	}
}

// TestRepositoryTemplate_JPEG_MagicNumber (T030 sibling): JPEG SOI
// marker is `\xff\xd8`.
func TestRepositoryTemplate_JPEG_MagicNumber(t *testing.T) {
	t.Parallel()
	if os.Getenv("METRICS_CHROME_PATH") == "" {
		t.Skip("METRICS_CHROME_PATH not set; chromedp test requires a chromium binary")
	}
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	browser, err := render.New(render.BrowserOpts{
		ExecPath: os.Getenv("METRICS_CHROME_PATH"),
	})
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}
	defer func() { _ = browser.Close() }()
	deps.Render = browser

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "jpeg",
		Inputs:   map[string]any{"user": "octocat", "repo": "hello-world"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", res.MIME)
	}
	if len(res.Output) < 2 {
		t.Fatalf("JPEG output too short: %d bytes", len(res.Output))
	}
	if res.Output[0] != 0xff || res.Output[1] != 0xd8 {
		t.Errorf("JPEG SOI marker missing: got %#x %#x, want 0xff 0xd8",
			res.Output[0], res.Output[1])
	}
}
