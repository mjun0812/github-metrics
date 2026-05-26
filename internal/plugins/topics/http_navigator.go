package topics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// httpDoer is the minimal interface that our Navigator needs from an
// HTTP client. *http.Client and *httpx.Client.HTTPClient() both
// satisfy it; tests can supply a stub.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// httpNavigator is the production Navigator. It fetches the public
// server-rendered "/stars/<user>/topics" page over plain HTTPS and
// extracts the topic list with goquery — no headless browser needed.
//
// The page is fully SSR (no `include-fragment` / `turbo-frame`
// hydration), so we can read every topic directly from the static HTML.
// Confirmed on github.com/stars/lowlighter/topics: 30 topics returned
// with a single `curl` request (May 2026 PoC).
type httpNavigator struct {
	client    httpDoer
	userAgent string
	timeout   time.Duration
}

// NewHTTPNavigator returns the production Navigator. When client is
// nil, http.DefaultClient is used. userAgent is sent on every request
// — GitHub serves a degraded HTML page to requests with an empty UA,
// so we always send a real-looking one.
func NewHTTPNavigator(client httpDoer, userAgent string) Navigator {
	if client == nil {
		client = http.DefaultClient
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &httpNavigator{
		client:    client,
		userAgent: userAgent,
		timeout:   30 * time.Second,
	}
}

// defaultUserAgent is sent when the caller did not supply one. We
// emit a recognisable product token so GitHub operators can identify
// our traffic in their logs.
const defaultUserAgent = "github-metrics (+https://github.com/mjun0812/github-metrics)"

// Fetch implements Navigator.
func (n *httpNavigator) Fetch(ctx context.Context, target string) ([]Topic, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("topics: build request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	// GitHub gates HTML vs JSON via Accept — be explicit so the SSR
	// HTML body comes back even if the client overrides defaults.
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("topics: GET %s: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain a small chunk to keep the connection reusable.
		_, _ = io.CopyN(io.Discard, resp.Body, 1<<10)
		return nil, fmt.Errorf("topics: GET %s: status %d", target, resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("topics: parse HTML: %w", err)
	}
	return extractTopics(doc), nil
}

// extractTopics walks the HTML document and returns one Topic per
// distinct "/topics/<slug>" anchor. Used by Fetch and by tests that
// want to drive extraction off a static body without round-tripping
// HTTP.
func extractTopics(doc *goquery.Document) []Topic {
	seen := map[string]struct{}{}
	out := make([]Topic, 0, 16)
	doc.Find(`a[href^="/topics/"]`).Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		slug := topicSlugFromHref(href)
		if slug == "" {
			return
		}
		if _, dup := seen[slug]; dup {
			return
		}
		seen[slug] = struct{}{}

		// Display name: prefer a heading inside the anchor (h3 / p / span
		// with one of GitHub's `f3` / `topic-name` shapes), fall back to
		// the slug capitalised lightly. We trim aggressively because
		// GitHub indents the HTML.
		name := firstNonEmptyText(
			a,
			"h3",
			".topic-name",
			"p.f3", "p.f4", "p.lh-condensed",
			"p",
		)
		if name == "" {
			name = slug
		}

		// Description: a paragraph sibling of the heading inside the
		// anchor. Fall back to empty when GitHub omits it.
		desc := ""
		a.Find("p").Each(func(_ int, p *goquery.Selection) {
			t := strings.TrimSpace(p.Text())
			if t == "" || t == name {
				return
			}
			if desc == "" {
				desc = t
			}
		})

		// Icon: the first <img> inside the anchor.
		icon := ""
		if img := a.Find("img").First(); img.Length() > 0 {
			if src, ok := img.Attr("src"); ok {
				icon = src
			}
		}

		// starred-at metadata may be exposed on an ancestor element
		// (article / li) via a data-starred-at attribute. The current
		// production page does not expose it; tests can still use the
		// attribute to seed a sort order.
		starredAt := ""
		for sel := a; sel.Length() > 0; sel = sel.Parent() {
			if v, ok := sel.Attr("data-starred-at"); ok && v != "" {
				starredAt = v
				break
			}
		}

		out = append(out, Topic{
			Name:        name,
			Description: desc,
			Icon:        icon,
			URL:         href,
			StarredAt:   starredAt,
		})
	})
	return out
}

// topicSlugFromHref returns the "<slug>" portion of a "/topics/<slug>"
// URL, ignoring query strings, fragments and the index page
// ("/topics/"). Returns "" when the href is not a topic detail link.
func topicSlugFromHref(href string) string {
	if href == "" {
		return ""
	}
	// Strip query / fragment to keep the slug comparison stable.
	if u, err := url.Parse(href); err == nil {
		href = u.Path
	}
	if !strings.HasPrefix(href, "/topics/") {
		return ""
	}
	rest := strings.TrimPrefix(href, "/topics/")
	// Drop anything after the next `/` so "/topics/go/whatever" still
	// classifies as the "go" topic.
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	return rest
}

// firstNonEmptyText returns the first non-empty trimmed text from the
// selectors, evaluated against `a` in order.
func firstNonEmptyText(a *goquery.Selection, selectors ...string) string {
	for _, sel := range selectors {
		text := strings.TrimSpace(a.Find(sel).First().Text())
		if text != "" {
			return text
		}
	}
	return ""
}

// Ensure httpNavigator implements Navigator at compile time.
var _ Navigator = (*httpNavigator)(nil)
