// Command render-fixture is a one-off renderer that injects synthetic
// sponsors + topics plugin Results into a plugins.Data and dumps the
// classic-template SVG to disk so we can screenshot it for visual
// verification of the partials added in 011.
//
// Real upstream data isn't reachable for these two plugins yet (sponsors
// needs `read:user` scope + GraphQL wiring; topics needs the chromedp
// scraping path + `extras.metrics.run.puppeteer.scrapping` extras), so
// no real user — not even lowlighter — produces non-empty output via
// the live engine path today. Until that wiring lands, this fixture
// dump is the closest visual proof we can ship.
//
// Usage:
//
//	go run ./cmd/render-fixture --slug sponsors --out /tmp/sponsors.svg
//	go run ./cmd/render-fixture --slug topics   --out /tmp/topics.svg
//	go run ./cmd/render-fixture --slug topics_icons --out /tmp/topics_icons.svg
//
// Delete this directory once the plugins' Run wiring lands and real
// data renders end-to-end.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/notable"
	"github.com/mjun0812/github-metrics/internal/plugins/projects"
	"github.com/mjun0812/github-metrics/internal/plugins/repositories"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
	"github.com/mjun0812/github-metrics/internal/templates"

	// 013 side-effect imports — every plugin's init() registers a partial
	_ "github.com/mjun0812/github-metrics/internal/plugins/notable"
	_ "github.com/mjun0812/github-metrics/internal/plugins/projects"
	_ "github.com/mjun0812/github-metrics/internal/plugins/repositories"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	// Side-effect imports — every plugin's init() registers a partial,
	// and the classic package's init() registers the template itself.
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/topics"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

func main() {
	var (
		slug = flag.String("slug", "sponsors", "fixture to render: sponsors / topics / topics_icons / sponsorships / projects / notable / stargazers / repositories_pinned")
		out  = flag.String("out", "", "output SVG path (required)")
	)
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "render-fixture: --out is required")
		os.Exit(1)
	}

	data := plugins.NewData()
	inputs := map[string]any{
		// The mjun0812 capture renders each plugin standalone via
		// `--plugin base=""` — preserve that here so the output is a
		// dedicated plugin block with no base sections bleeding in.
		"base": "",
	}

	switch *slug {
	case "sponsors":
		seedSponsors(data, inputs)
	case "topics":
		seedTopics(data, inputs, "labels")
	case "topics_icons":
		seedTopics(data, inputs, "icons")
	case "sponsorships":
		seedSponsorships(data, inputs)
	case "projects":
		seedProjects(data, inputs)
	case "notable":
		seedNotable(data, inputs)
	case "stargazers":
		seedStargazers(data, inputs)
	case "repositories_pinned":
		seedRepositoriesPinned(data, inputs)
	default:
		fmt.Fprintf(os.Stderr, "render-fixture: unknown slug %q\n", *slug)
		os.Exit(1)
	}

	tmpl, err := templates.MustGet("classic")
	if err != nil {
		fmt.Fprintf(os.Stderr, "render-fixture: load classic template: %v\n", err)
		os.Exit(1)
	}
	pc := &templates.PartialContext{
		Inputs: inputs,
		Logger: slog.Default(),
		Data:   data,
	}
	svg, err := tmpl.Run(context.Background(), pc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render-fixture: render: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(svg), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "render-fixture: write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d bytes to %s\n", len(svg), *out)
}

// seedSponsors populates data.Plugins["sponsors"] with a representative
// goal + about + list payload mirroring what an active GitHub Sponsors
// profile (e.g. lowlighter) would produce.
func seedSponsors(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "lowlighter"}

	sponsorList := []sponsors.Sponsor{
		{Login: "github", Type: "organization", Avatar: "https://github.com/github.png?size=64", Tier: "Bronze", Since: mustTime("2023-01-15T00:00:00Z")},
		{Login: "kentcdodds", Type: "user", Avatar: "https://github.com/kentcdodds.png?size=64", Tier: "Silver", Since: mustTime("2023-04-02T00:00:00Z")},
		{Login: "sindresorhus", Type: "user", Avatar: "https://github.com/sindresorhus.png?size=64", Tier: "Gold", Since: mustTime("2024-06-19T00:00:00Z")},
		{Login: "yyx990803", Type: "user", Avatar: "https://github.com/yyx990803.png?size=64", Tier: "Bronze", Since: mustTime("2024-08-04T00:00:00Z")},
	}
	pastList := []sponsors.Sponsor{
		{Login: "gaearon", Type: "user", Avatar: "https://github.com/gaearon.png?size=64", Tier: "Silver", Since: mustTime("2022-03-01T00:00:00Z")},
		{Login: "tj", Type: "user", Avatar: "https://github.com/tj.png?size=64", Tier: "Bronze", Since: mustTime("2022-09-14T00:00:00Z")},
	}

	data.SetPlugin("sponsors", &sponsors.Result{
		Mode:         "user",
		Title:        "lowlighter's sponsors",
		User:         "lowlighter",
		Sections:     []string{"goal", "about", "list"},
		Sponsors:     sponsorList,
		Past:         pastList,
		PastIncluded: true,
		Size:         64,
		Count: sponsors.Count{
			Active: sponsors.CountBucket{Total: len(sponsorList)},
			Past:   sponsors.CountBucket{Total: len(pastList)},
		},
		Goal: &sponsors.Goal{
			Title:       "Help maintain `metrics` as a sustainable open-source project",
			Description: "Reaching the goal would let me keep dedicating ~1 day/week to metrics, including timely bug fixes, plugin reviews, and the next-generation rendering pipeline.",
			Progress:    62,
		},
		About: "I am the maintainer of `metrics`, an open-source GitHub Action that generates dynamic profile READMEs powered by ~30 community plugins. Your sponsorship pays for the time spent triaging issues, reviewing PRs, and keeping the project deployable on free-tier infrastructure.",
	})

	// Enable the sponsors plugin block in classic.
	inputs["plugin_sponsors"] = "yes"
	inputs["plugin_sponsors_sections"] = "goal, about, list"
	inputs["plugin_sponsors_past"] = "yes"
}

// seedTopics populates data.Plugins["topics"] with a representative
// set of starred topics so the topics partial renders both label and
// icon modes.
func seedTopics(data *plugins.Data, inputs map[string]any, mode string) {
	list := []topics.Topic{
		{Name: "actions", Description: "GitHub Actions makes it easy to automate all your software workflows.", Icon: dataURIIcon("⚙"), URL: "https://github.com/topics/actions"},
		{Name: "devops", Description: "DevOps is a set of practices for collaboration between teams.", Icon: dataURIIcon("∞"), URL: "https://github.com/topics/devops"},
		{Name: "github", Description: "GitHub is a development platform inspired by the way you work.", Icon: dataURIIcon("⌥"), URL: "https://github.com/topics/github"},
		{Name: "graphql", Description: "GraphQL is a query language for APIs.", Icon: dataURIIcon("◆"), URL: "https://github.com/topics/graphql"},
		{Name: "javascript", Description: "JavaScript is the language of the web.", Icon: dataURIIcon("JS"), URL: "https://github.com/topics/javascript"},
		{Name: "nodejs", Description: "Node.js is a JavaScript runtime built on V8.", Icon: dataURIIcon("N"), URL: "https://github.com/topics/nodejs"},
		{Name: "open-source", Description: "Open source is source code made freely available.", Icon: dataURIIcon("◇"), URL: "https://github.com/topics/open-source"},
		{Name: "puppeteer", Description: "Headless Chrome Node.js API.", Icon: dataURIIcon("◉"), URL: "https://github.com/topics/puppeteer"},
		{Name: "self-hosted", Description: "Self-hosted services and runners.", Icon: dataURIIcon("⌂"), URL: "https://github.com/topics/self-hosted"},
		{Name: "svg", Description: "SVG (Scalable Vector Graphics).", Icon: dataURIIcon("S"), URL: "https://github.com/topics/svg"},
		{Name: "typescript", Description: "TypeScript is a typed superset of JavaScript.", Icon: dataURIIcon("TS"), URL: "https://github.com/topics/typescript"},
		{Name: "rust", Description: "A language empowering everyone to build reliable and efficient software.", Icon: dataURIIcon("R"), URL: "https://github.com/topics/rust"},
	}

	data.SetPlugin("topics", &topics.Result{
		Mode:  mode,
		List:  list,
		Limit: 15,
		Sort:  "name",
	})
	inputs["plugin_topics"] = "yes"
	inputs["plugin_topics_limit"] = "15"
	inputs["plugin_topics_mode"] = mode
}

// dataURIIcon returns a 24x24 SVG data URI with the supplied label
// centred inside a soft blue rounded square. Used as a stand-in for the
// real GitHub topic icons (which are served from per-topic URLs the
// upstream scraper resolves at runtime — we have no equivalent
// data-fetch path yet, so the fixture supplies inline SVG icons so the
// `<img src=...>` path the partial uses actually renders something).
//
// The SVG body is URL-encoded so the templates' EscapeXML pass does not
// mangle the `<` / `>` characters inside the SVG source.
func dataURIIcon(label string) string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><rect width="24" height="24" rx="4" fill="#0366d6"/><text x="12" y="16" text-anchor="middle" font-family="Helvetica" font-size="11" font-weight="bold" fill="white">` + label + `</text></svg>`
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(svg)
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// seedSponsorships populates data.Plugins["sponsorships"] with a
// representative list of maintainers the viewer sponsors (spec 013).
func seedSponsorships(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "mjun0812"}
	list := []sponsorships.Sponsored{
		{Login: "lowlighter", Type: "user", Avatar: "https://github.com/lowlighter.png?size=64", Since: mustTime("2024-08-04T00:00:00Z")},
		{Login: "kubernetes", Type: "organization", Avatar: "https://github.com/kubernetes.png?size=64", Since: mustTime("2024-09-15T00:00:00Z")},
		{Login: "ggicci", Type: "user", Avatar: "https://github.com/ggicci.png?size=64", Since: mustTime("2025-01-10T00:00:00Z")},
	}
	data.SetPlugin("sponsorships", &sponsorships.Result{Active: list})
	inputs["plugin_sponsorships"] = "yes"
}

// seedProjects populates data.Plugins["projects"] with a representative
// set of GitHub Projects v2 entries (spec 013).
func seedProjects(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "mjun0812"}
	list := []projects.Project{
		{Name: "metrics-go migration", Description: "Track the upstream-compat Go rewrite roadmap.", URL: "https://github.com/users/mjun0812/projects/1", UpdatedAt: mustTime("2026-05-22T10:00:00Z"), Closed: false},
		{Name: "Personal OKRs Q2 2026", Description: "Quarterly OKR tracking.", URL: "https://github.com/users/mjun0812/projects/2", UpdatedAt: mustTime("2026-05-20T12:30:00Z"), Closed: false},
		{Name: "Plugin parity sprint", Description: "Catch-all for the per-plugin partial refactor.", URL: "https://github.com/users/mjun0812/projects/3", UpdatedAt: mustTime("2026-05-15T09:00:00Z"), Closed: true},
	}
	data.SetPlugin("projects", &projects.Result{Mode: "user", List: list, Limit: 4})
	inputs["plugin_projects"] = "yes"
}

// seedNotable populates data.Plugins["notable"] with notable owner
// contributions (spec 013, basic mode).
func seedNotable(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "mjun0812"}
	list := []notable.NotableContrib{
		{Repo: "mjun0812/github-metrics", Title: "mjun0812/github-metrics", Description: "Go re-implementation of upstream `lowlighter/metrics`.", Type: "owner", StargazerCount: 142, ForkCount: 18},
		{Repo: "mjun0812/dotfiles", Title: "mjun0812/dotfiles", Description: "Personal dotfiles with Nix flakes.", Type: "owner", StargazerCount: 67, ForkCount: 9},
		{Repo: "mjun0812/research-notes", Title: "mjun0812/research-notes", Description: "Public research notebook.", Type: "owner", StargazerCount: 23, ForkCount: 4},
		{Repo: "mjun0812/aws-toolkit", Title: "mjun0812/aws-toolkit", Description: "Lightweight helpers for AWS operations.", Type: "owner", StargazerCount: 11, ForkCount: 2},
		{Repo: "mjun0812/k8s-playground", Title: "mjun0812/k8s-playground", Description: "Kubernetes experiments and manifests.", Type: "owner", StargazerCount: 5, ForkCount: 1},
	}
	data.SetPlugin("notable", &notable.Result{List: list})
	inputs["plugin_notable"] = "yes"
}

// seedStargazers populates data.Plugins["stargazers"] with a synthetic
// month-bucketed cumulative chart (spec 013).
func seedStargazers(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "mjun0812"}
	series := []stargazers.ChartPoint{
		{Date: mustTime("2024-09-01T00:00:00Z"), Count: 5},
		{Date: mustTime("2024-10-01T00:00:00Z"), Count: 14},
		{Date: mustTime("2024-11-01T00:00:00Z"), Count: 27},
		{Date: mustTime("2024-12-01T00:00:00Z"), Count: 41},
		{Date: mustTime("2025-01-01T00:00:00Z"), Count: 58},
		{Date: mustTime("2025-02-01T00:00:00Z"), Count: 73},
		{Date: mustTime("2025-03-01T00:00:00Z"), Count: 89},
		{Date: mustTime("2025-04-01T00:00:00Z"), Count: 104},
		{Date: mustTime("2025-05-01T00:00:00Z"), Count: 121},
		{Date: mustTime("2025-06-01T00:00:00Z"), Count: 138},
		{Date: mustTime("2025-07-01T00:00:00Z"), Count: 151},
		{Date: mustTime("2025-08-01T00:00:00Z"), Count: 167},
		{Date: mustTime("2025-09-01T00:00:00Z"), Count: 184},
		{Date: mustTime("2025-10-01T00:00:00Z"), Count: 198},
		{Date: mustTime("2025-11-01T00:00:00Z"), Count: 213},
		{Date: mustTime("2025-12-01T00:00:00Z"), Count: 228},
		{Date: mustTime("2026-01-01T00:00:00Z"), Count: 245},
		{Date: mustTime("2026-02-01T00:00:00Z"), Count: 261},
		{Date: mustTime("2026-03-01T00:00:00Z"), Count: 278},
		{Date: mustTime("2026-04-01T00:00:00Z"), Count: 296},
		{Date: mustTime("2026-05-01T00:00:00Z"), Count: 314},
	}
	data.SetPlugin("stargazers", &stargazers.Result{
		Mode:   "user",
		List:   []stargazers.Stargazer{},
		Charts: stargazers.StargazersCharts{Type: "classic-latest100", Series: series},
	})
	inputs["plugin_stargazers"] = "yes"
}

// seedRepositoriesPinned populates data.Plugins["repositories"].Pinned
// with a representative pin set (spec 013).
func seedRepositoriesPinned(data *plugins.Data, inputs map[string]any) {
	data.User = &plugins.User{Login: "mjun0812"}
	pinned := []plugins.Repository{
		{NameWithOwner: "mjun0812/github-metrics", Description: "Go re-implementation of upstream `lowlighter/metrics`.", URL: "https://github.com/mjun0812/github-metrics", Stars: 142, Forks: 18, Language: &plugins.LanguageStat{Name: "Go", Color: "#00ADD8"}},
		{NameWithOwner: "mjun0812/dotfiles", Description: "Personal dotfiles with Nix flakes.", URL: "https://github.com/mjun0812/dotfiles", Stars: 67, Forks: 9, Language: &plugins.LanguageStat{Name: "Nix", Color: "#7e7eff"}},
		{NameWithOwner: "mjun0812/k8s-playground", Description: "Kubernetes experiments and manifests.", URL: "https://github.com/mjun0812/k8s-playground", Stars: 5, Forks: 1, Language: &plugins.LanguageStat{Name: "YAML", Color: "#cb171e"}},
	}
	data.SetPlugin("repositories", &repositories.Result{
		Featured: pinned,
		Pinned:   pinned,
	})
	data.Computed.RepositoryList = pinned // base shape so partial passes
	inputs["plugin_repositories"] = "yes"
	inputs["plugin_repositories_pinned"] = "yes"
}
