package render

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeFetcher records the URLs it is asked to fetch and returns a
// deterministic data URI per URL, or an error for URLs listed in fail.
type fakeFetcher struct {
	mu     sync.Mutex
	calls  map[string]int
	fail   map[string]bool
	prefix string
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{calls: map[string]int{}, fail: map[string]bool{}, prefix: "data:image/png;base64,AAAA"}
}

func (f *fakeFetcher) ImgB64(_ context.Context, target string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[target]++
	if f.fail[target] {
		return "", errors.New("boom")
	}
	return f.prefix, nil
}

// TestInlineImages_ReplacesRemoteAvatar asserts a remote `<img>` src is
// swapped for the fetched data URI.
func TestInlineImages_ReplacesRemoteAvatar(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	in := `<img class="avatar" src="https://avatars.githubusercontent.com/u/1?v=4" width="20" height="20"/>`
	got, err := InlineImagesStage(context.Background(), f).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(got, "https://avatars.githubusercontent.com") {
		t.Errorf("remote URL should be gone; got %q", got)
	}
	if !strings.Contains(got, `src="data:image/png;base64,AAAA"`) {
		t.Errorf("data URI not inlined; got %q", got)
	}
}

// TestInlineImages_UnescapesBeforeFetch asserts the XML-escaped `&amp;`
// in the src is reversed before the URL is fetched (otherwise the CDN
// would 404 on the literal entity).
func TestInlineImages_UnescapesBeforeFetch(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	in := `<img class="avatar" src="https://example/a.png?v=4&amp;s=40"/>`
	if _, err := InlineImagesStage(context.Background(), f).Run(in); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := f.calls["https://example/a.png?v=4&s=40"]; !ok {
		t.Errorf("expected fetch of unescaped URL; calls=%v", f.calls)
	}
}

// TestInlineImages_DedupesFetches asserts a URL referenced multiple
// times is fetched only once but replaced everywhere.
func TestInlineImages_DedupesFetches(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	url := "https://example/x.png"
	in := `<img src="` + url + `"/><img src="` + url + `"/>`
	got, err := InlineImagesStage(context.Background(), f).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := f.calls[url]; n != 1 {
		t.Errorf("want 1 fetch, got %d", n)
	}
	if c := strings.Count(got, "data:image/png;base64,"); c != 2 {
		t.Errorf("want 2 inlined occurrences, got %d (%q)", c, got)
	}
}

// TestInlineImages_PassThrough asserts data URIs and non-http schemes
// are left untouched and never fetched.
func TestInlineImages_PassThrough(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	in := `<img src="data:image/png;base64,ZZZZ"/><image href="data:image/png;base64,YYYY"/>`
	got, err := InlineImagesStage(context.Background(), f).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != in {
		t.Errorf("input should be unchanged; got %q", got)
	}
	if len(f.calls) != 0 {
		t.Errorf("expected no fetches; calls=%v", f.calls)
	}
}

// TestInlineImages_ReplacesSVGImageHref asserts a native-SVG
// `<image href="http…">` (the header avatar after the #409 Phase B1
// conversion) and its `xlink:href` spelling are inlined like `<img src>`.
func TestInlineImages_ReplacesSVGImageHref(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	in := `<image class="avatar" href="https://avatars.githubusercontent.com/u/1?v=4" width="20" height="20"/>` +
		`<image xlink:href="https://avatars.githubusercontent.com/u/2?v=4"/>`
	got, err := InlineImagesStage(context.Background(), f).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(got, "https://avatars.githubusercontent.com") {
		t.Errorf("remote URL should be gone; got %q", got)
	}
	if c := strings.Count(got, `="data:image/png;base64,AAAA"`); c != 2 {
		t.Errorf("want 2 inlined hrefs, got %d (%q)", c, got)
	}
}

// TestInlineImages_FetchFailureKeepsURL asserts a fetch error is
// surfaced while the original URL is preserved (best-effort, FR-018).
func TestInlineImages_FetchFailureKeepsURL(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	bad := "https://example/bad.png"
	f.fail[bad] = true
	in := `<img src="` + bad + `"/>`
	got, err := InlineImagesStage(context.Background(), f).Run(in)
	if err == nil {
		t.Fatal("expected aggregated fetch error")
	}
	if !strings.Contains(got, bad) {
		t.Errorf("failed URL should be preserved; got %q", got)
	}
}

// TestInlineImages_NilFetcher asserts the stage is a no-op when no
// fetcher is wired (mocked-data path).
func TestInlineImages_NilFetcher(t *testing.T) {
	t.Parallel()
	in := `<img src="https://example/x.png"/>`
	got, err := InlineImagesStage(context.Background(), nil).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != in {
		t.Errorf("nil fetcher should pass through; got %q", got)
	}
}

// TestInlineImages_Idempotent asserts a second pass over already-inlined
// output performs no further fetches.
func TestInlineImages_Idempotent(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	in := `<img src="https://example/x.png"/>`
	once, err := InlineImagesStage(context.Background(), f).Run(in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	twice, err := InlineImagesStage(context.Background(), f).Run(once)
	if err != nil {
		t.Fatalf("Run (2nd): %v", err)
	}
	if once != twice {
		t.Errorf("second pass changed output: %q != %q", once, twice)
	}
	if n := f.calls["https://example/x.png"]; n != 1 {
		t.Errorf("want 1 fetch across both passes, got %d", n)
	}
}
