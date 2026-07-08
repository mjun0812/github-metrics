package render

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ImageFetcher fetches a remote image and returns a
// `data:<mime>;base64,<payload>` URI suitable for embedding directly in
// SVG/HTML. *httpx.Client satisfies this via its ImgB64 method.
type ImageFetcher interface {
	ImgB64(ctx context.Context, target string) (string, error)
}

// inlineWorkers bounds the number of concurrent image fetches the inline
// stage issues. Avatar-heavy renders (people / contributors / sponsors)
// reference dozens of CDN URLs; fetching a few at a time keeps the
// render fast without hammering the CDN.
const inlineWorkers = 8

// inlineFetchTimeout bounds each individual image fetch so a single hung
// CDN connection cannot stall the whole render. The httpx client has no
// deadline of its own (transport Timeout is 0 and retries are enabled),
// so this per-fetch context caps the underlying retry/backoff loop —
// mirroring the per-call deadlines other outbound fetches already use
// (internal/plugins/repositories, internal/plugins/languages/indepth).
const inlineFetchTimeout = 10 * time.Second

// imgSrcRe captures the URL inside either an HTML `<img ... src="URL">`
// or a native-SVG `<image ... href="URL">` / `<image ... xlink:href="URL">`
// element. Avatars migrated from `<img>` to `<image href>` with the
// native-SVG conversions (#409 Phase B1-B3: header, then topics /
// sponsors / sponsorships, then people / contributors) while the still
// unconverted partials (notable) emit `<img src>`, so both spellings
// must inline. The URL value is XML-escaped in the rendered
// SVG (EscapeXML turns `&` into `&amp;`), so the captured text may carry
// entities that must be unescaped before the URL is fetched. Group 1 is
// the tag up to and including the opening quote, group 2 is the URL,
// group 3 is the closing quote.
var imgSrcRe = regexp.MustCompile(`(<(?:img\b[^>]*?\bsrc|image\b[^>]*?\b(?:xlink:)?href)=")([^"]+)(")`)

// InlineImagesStage returns a decoration stage that rewrites every
// remote `<img src="http(s)://...">` (and native-SVG
// `<image href="http(s)://...">`) in the SVG into a self-contained
// `data:` URI by fetching the resource through fetcher.
//
// Why this exists: GitHub renders embedded SVGs through its camo image
// proxy, which does not resolve external resources referenced from
// inside the SVG — the avatar / icon tags the partials emit (the
// native-SVG base.header / people / contributors / sponsors /
// sponsorships / topics `<image>` avatars plus the notable `<img>`
// tags). An SVG that keeps
// `src="https://avatars.githubusercontent.com/..."` therefore shows
// broken icons on GitHub and anywhere the file is opened offline.
// Inlining each image as base64 makes the SVG self-contained.
//
// Best-effort (FR-018): URLs that are already inline (`data:`) or use a
// non-http scheme pass through untouched, and a fetch failure leaves the
// original URL in place while the error is aggregated into Apply's
// []error. Each distinct URL is fetched at most once per invocation.
func InlineImagesStage(ctx context.Context, fetcher ImageFetcher) PipelineStage {
	return PipelineStage{
		Name: "image-inline",
		Run: func(in string) (string, error) {
			if in == "" || fetcher == nil {
				return in, nil
			}
			return inlineImages(ctx, fetcher, in)
		},
	}
}

// inlineImages collects the distinct remote URLs referenced by `<img>`
// tags, resolves them to data URIs, then swaps each occurrence in place.
func inlineImages(ctx context.Context, fetcher ImageFetcher, in string) (string, error) {
	targets := make(map[string]struct{})
	for _, m := range imgSrcRe.FindAllStringSubmatch(in, -1) {
		if raw := m[2]; isRemoteURL(raw) {
			targets[html.UnescapeString(raw)] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return in, nil
	}

	resolved, errs := fetchAll(ctx, fetcher, targets)

	out := imgSrcRe.ReplaceAllStringFunc(in, func(match string) string {
		g := imgSrcRe.FindStringSubmatch(match)
		prefix, raw, suffix := g[1], g[2], g[3]
		if !isRemoteURL(raw) {
			return match
		}
		dataURI := resolved[html.UnescapeString(raw)]
		if dataURI == "" {
			return match // fetch failed; keep the original URL.
		}
		return prefix + dataURI + suffix
	})
	return out, errors.Join(errs...)
}

// isRemoteURL reports whether s is an http(s) URL the stage should fetch.
// data: URIs and other schemes are left untouched.
func isRemoteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// fetchAll resolves each target URL to a data URI, fetching up to
// inlineWorkers at a time. Failures are collected into the returned
// slice and their map entry is left empty so the caller keeps the
// original URL in place.
func fetchAll(ctx context.Context, fetcher ImageFetcher, targets map[string]struct{}) (map[string]string, []error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		errs     []error
		resolved = make(map[string]string, len(targets))
		sem      = make(chan struct{}, inlineWorkers)
	)
	for target := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fetchCtx, cancel := context.WithTimeout(ctx, inlineFetchTimeout)
			defer cancel()
			dataURI, err := fetcher.ImgB64(fetchCtx, target)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("inline %s: %w", target, err))
				return
			}
			resolved[target] = dataURI
		}()
	}
	wg.Wait()
	return resolved, errs
}
