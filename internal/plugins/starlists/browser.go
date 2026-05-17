package starlists

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"

	"github.com/mjun0812/github-metrics/internal/render"
)

// browserNavigator is the production Navigator backed by *render.Browser.
type browserNavigator struct {
	browser *render.Browser
}

// NewBrowserNavigator returns a production Navigator. Exposed for
// chromedp-tagged tests that want to drive the real scrape path
// against a local fixture server.
func NewBrowserNavigator(b *render.Browser) Navigator {
	return &browserNavigator{browser: b}
}

const listsExtractJS = `
	(() => {
		const cards = Array.from(document.querySelectorAll('.starred-list, article.starred-list, [data-list-name]'));
		return cards.map(card => {
			const a = card.querySelector('a');
			const name = (card.querySelector('.starred-list-name') || a || {}).textContent || '';
			const desc = (card.querySelector('.starred-list-description') || {}).textContent || '';
			const count = parseInt((card.querySelector('.starred-list-count') || {}).textContent || '0', 10) || 0;
			const url = a ? a.getAttribute('href') : '';
			return {
				name: name.trim(),
				description: desc.trim(),
				count: count,
				url: url || '',
			};
		});
	})()`

const reposExtractJS = `
	(() => {
		const links = Array.from(document.querySelectorAll('.starred-list-repo a, .starred-list-detail-repos a'));
		const out = [];
		const seen = new Set();
		for (const a of links) {
			const href = a.getAttribute('href') || '';
			// /owner/repo style links → "owner/repo"
			const m = href.match(/^\/([^\/]+)\/([^\/]+)(?:[\/?#]|$)/);
			if (!m) continue;
			const nwo = m[1] + '/' + m[2];
			if (seen.has(nwo)) continue;
			seen.add(nwo);
			out.push(nwo);
		}
		return out;
	})()`

func (n *browserNavigator) FetchLists(ctx context.Context, url string) ([]Starlist, error) {
	tabCtx, cancel, err := n.browser.NewTab(ctx)
	if err != nil {
		return nil, fmt.Errorf("starlists: new tab: %w", err)
	}
	defer cancel()

	var raw []rawStarlist
	err = chromedp.Run(
		tabCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.starred-list, article.starred-list`, chromedp.ByQuery),
		chromedp.Evaluate(listsExtractJS, &raw),
	)
	if err != nil {
		return nil, fmt.Errorf("starlists: chromedp run: %w", err)
	}
	out := make([]Starlist, 0, len(raw))
	for _, r := range raw {
		out = append(out, Starlist{
			Name:        r.Name,
			Description: r.Description,
			Count:       r.Count,
			URL:         r.URL,
		})
	}
	return out, nil
}

func (n *browserNavigator) FetchRepos(ctx context.Context, listURL string) ([]string, error) {
	tabCtx, cancel, err := n.browser.NewTab(ctx)
	if err != nil {
		return nil, fmt.Errorf("starlists: new tab: %w", err)
	}
	defer cancel()

	var repos []string
	err = chromedp.Run(
		tabCtx,
		chromedp.Navigate(listURL),
		chromedp.WaitVisible(`.starred-list-repo, .starred-list-detail-repos`, chromedp.ByQuery),
		chromedp.Evaluate(reposExtractJS, &repos),
	)
	if err != nil {
		return nil, fmt.Errorf("starlists: chromedp run: %w", err)
	}
	return repos, nil
}

type rawStarlist struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Count       int    `json:"count"`
	URL         string `json:"url"`
}

var _ Navigator = (*browserNavigator)(nil)
