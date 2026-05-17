package topics

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"

	"github.com/mjun0812/github-metrics/internal/render"
)

// browserNavigator is the production Navigator backed by a chromedp
// tab spawned from *render.Browser. It navigates to the starred-topics
// page, waits for the topic cards, then extracts the relevant fields
// via an in-page JS expression.
type browserNavigator struct {
	browser *render.Browser
}

// NewBrowserNavigator returns a production Navigator. Exposed for
// chromedp-tagged tests that want to drive the real scrape path
// against a local fixture server.
func NewBrowserNavigator(b *render.Browser) Navigator {
	return &browserNavigator{browser: b}
}

// Fetch implements Navigator.
func (n *browserNavigator) Fetch(ctx context.Context, url string) ([]Topic, error) {
	tabCtx, cancel, err := n.browser.NewTab(ctx)
	if err != nil {
		return nil, fmt.Errorf("topics: new tab: %w", err)
	}
	defer cancel()

	const extractJS = `
		(() => {
			const cards = Array.from(document.querySelectorAll('.starred-list-topics .topic-card, .starred-list-topics article'));
			return cards.map(card => {
				const a = card.querySelector('a');
				const name = (card.querySelector('.topic-name') || a || {}).textContent || '';
				const desc = (card.querySelector('.topic-description') || {}).textContent || '';
				const img = card.querySelector('img');
				const icon = img ? img.getAttribute('src') : '';
				const href = a ? a.getAttribute('href') : '';
				return {
					name: name.trim(),
					description: desc.trim(),
					icon: icon || '',
					url: href || '',
					starredAt: card.getAttribute('data-starred-at') || '',
				};
			});
		})()`

	var raw []rawTopic
	err = chromedp.Run(
		tabCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.starred-list-topics`, chromedp.ByQuery),
		chromedp.Evaluate(extractJS, &raw),
	)
	if err != nil {
		return nil, fmt.Errorf("topics: chromedp run: %w", err)
	}
	out := make([]Topic, 0, len(raw))
	for _, r := range raw {
		out = append(out, Topic(r))
	}
	return out, nil
}

type rawTopic struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	URL         string `json:"url"`
	StarredAt   string `json:"starredAt"`
}

// Ensure browserNavigator implements Navigator at compile time.
var _ Navigator = (*browserNavigator)(nil)
