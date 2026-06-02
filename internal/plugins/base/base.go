// Package base owns the special "base" plugin: it runs before every
// other plugin and populates the shared Data.User / Data.Organization /
// Data.Computed fields that downstream plugins depend on.
//
// The M4 implementation covers:
//   - user-account branch (runUser) and organization-account branch
//     (runOrganization)
//   - repository paging with batch-halving on transient 5xx / timeout
//     (repositories.go), writing both totals into Computed.Repositories
//     and the per-node accumulator into Computed.RepositoryList
//   - indepth GraphQL query (indepth.go) that augments Computed with
//     contribution-calendar / commits / issues / PR totals when at
//     least one indepth-dependent plugin is enabled
//
// The repository-account branch (templates that target a single
// repository) is M7 territory and currently returns (nil, nil).
package base

import (
	"context"
	"fmt"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "base"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &basePlugin{}

func init() {
	plugins.Register(Plugin)
}

type basePlugin struct{}

func (p *basePlugin) Name() string                     { return Name }
func (p *basePlugin) Metadata() *config.PluginMetadata { return nil }

// Run dispatches by account kind. Each branch populates data.User /
// data.Organization and data.Computed.{Repositories, RepositoryList,
// ContributionCalendar, TotalCommits / Issues / PullRequests} from the
// GraphQL client. The full upstream-compatible behavior (batch-halving
// paging, organization branch, indepth augmentation) lands with M4.
func (p *basePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, fmt.Errorf("base: nil PluginContext or Data")
	}
	if pc.GraphQL == nil {
		return nil, fmt.Errorf("base: nil GraphQL client")
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return nil, fmt.Errorf("base: input %q is required", "user")
	}

	switch pc.Data.Account {
	case "", plugins.AccountUser:
		return p.runUser(ctx, pc, login)
	case plugins.AccountOrganization:
		return p.runOrganization(ctx, pc, login)
	case plugins.AccountRepository:
		return p.runRepository(ctx, pc, login)
	default:
		return nil, fmt.Errorf("base: unknown account kind %q", pc.Data.Account)
	}
}

// runRepository is the M7 base-plugin entry point for the
// repository template. It runs the same user fetch as runUser
// (downstream plugins still need data.User populated for things like
// avatar / sponsorshipsAsMaintainer) and then layers the single-repo
// fetch on top.
func (p *basePlugin) runRepository(ctx context.Context, pc *plugins.PluginContext, login string) (any, error) {
	// Fetch user first — downstream plugins + the repository template
	// header still need data.User.AvatarURL, etc.
	if _, err := p.runUser(ctx, pc, login); err != nil {
		return nil, err
	}
	pc.Data.Account = plugins.AccountRepository

	repo := repoFromInputs(pc.Inputs)
	if repo == "" {
		return nil, fmt.Errorf("base: repository template requires `repo` input")
	}

	r, err := FetchRepo(ctx, login, repo, pc.REST, pc.GraphQL)
	if err != nil {
		return nil, err
	}
	pc.Data.SetRepo(r)

	// Upstream `template.mjs:14-17` replaces `data.user.repositories.nodes`
	// with `[repository]` so existing user-centric plugins (languages /
	// activity / stargazers / projects / people / contributors / sponsors)
	// naturally produce repo-scoped output. We mirror that here by
	// synthesizing a single-element `Computed.RepositoryList` + matching
	// `Computed.Repositories` totals from data.Repo. Downstream plugins
	// stay unchanged.
	syntheticRepo := plugins.Repository{
		NameWithOwner: r.Owner + "/" + r.Name,
		Description:   r.Description,
		Stars:         r.Stargazers,
		Forks:         r.Forks,
	}
	if r.PrimaryLanguage != "" {
		lang := plugins.LanguageStat{
			Name:  r.PrimaryLanguage,
			Color: r.PrimaryLanguageColor,
		}
		syntheticRepo.Language = &lang
		// languages.Run iterates `repo.Languages` exclusively — leaving
		// the slice nil makes the plugin treat the synthetic repo as
		// having zero-byte language data and return Skipped. Seed a
		// single-language byte stat from PrimaryLanguage so the
		// languages plugin renders the section (FR-005). Size = 1
		// avoids zero-division; the displayed favorite is the primary
		// language regardless of the absolute number.
		syntheticRepo.Languages = []plugins.LanguageStat{{
			Name:  r.PrimaryLanguage,
			Color: r.PrimaryLanguageColor,
			Size:  1,
		}}
	}
	pc.Data.Computed.RepositoryList = []plugins.Repository{syntheticRepo}
	pc.Data.Computed.Repositories.Count = 1
	pc.Data.Computed.Repositories.Stargazers = r.Stargazers
	pc.Data.Computed.Repositories.Forks = r.Forks
	return nil, nil
}

// repoFromInputs returns the `repo` input value, or "" when unset.
func repoFromInputs(inputs map[string]any) string {
	if inputs == nil {
		return ""
	}
	v, ok := inputs["repo"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (p *basePlugin) runUser(ctx context.Context, pc *plugins.PluginContext, login string) (any, error) {
	resp, err := pc.GraphQL.User(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("base: user(%q): %w", login, err)
	}
	if resp == nil || resp.User == nil {
		return nil, fmt.Errorf("base: user(%q): not found", login)
	}
	u := resp.User
	pc.Data.Account = plugins.AccountUser
	pc.Data.User = &plugins.User{
		Login:                    u.Login,
		Name:                     derefString(u.Name),
		AvatarURL:                u.AvatarUrl,
		CreatedAt:                u.CreatedAt,
		Followers:                followersTotal(u),
		Following:                followingTotal(u),
		Watching:                 watchingTotal(u),
		SponsorshipsAsMaintainer: sponsorshipsAsMaintainerTotal(u),
		ContributedTo:            repositoriesContributedToTotal(u),
		RecentContributions:      recentContributionDays(u, baseHeaderCalendarDays),
		// 442: Activity-section aggregate counters.
		Commits:              contributionCommits(u),
		PullRequestsReviewed: contributionPullRequestReviews(u),
		PullRequestsOpened:   contributionPullRequests(u),
		IssuesOpened:         contributionIssues(u),
		IssueComments:        issueCommentsTotal(u),
		// 442: Community-stats counters.
		Organizations: organizationsTotal(u),
		Sponsoring:    sponsorshipsAsSponsorTotal(u),
		Starred:       starredRepositoriesTotal(u),
	}

	// M4: walk the entire repository connection with batch-halving on
	// transient 5xx / timeout. Downstream plugins consume both the
	// totals (Count, Stargazers, ...) and the per-node accumulator
	// (RepositoryList).
	if reposLimit := repositoriesLimit(pc.Settings); reposLimit > 0 {
		if err := populateRepositories(ctx, pc, login, reposLimit, true); err != nil {
			return nil, err
		}
	}

	// Trigger the indepth GraphQL query when at least one indepth-
	// dependent plugin is enabled. Per the contract this stays best-
	// effort: indepth failures land on Data.Errors and the standard
	// fields still surface.
	if indepthTriggered(pc.Inputs) {
		if err := runIndepth(ctx, pc, login); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// loginFromInputs returns the configured GitHub login.
func loginFromInputs(inputs map[string]any) string {
	if inputs == nil {
		return ""
	}
	if v, ok := inputs["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := inputs["login"].(string); ok {
		return v
	}
	return ""
}

// repositoriesLimit reads Settings.Repositories with a default of 100
// to match upstream behavior.
func repositoriesLimit(s *config.Settings) int {
	if s == nil {
		return 100
	}
	if s.Repositories > 0 {
		return s.Repositories
	}
	return 100
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// followersTotal / followingTotal / watchingTotal /
// sponsorshipsAsMaintainerTotal each return the totalCount sub-field
// from the matching User connection, or 0 when the connection is nil
// (GraphQL returned no data for that connection). They exist so
// runUser's User struct literal stays a clean field-by-field assignment.
func followersTotal(u *githubapi.UserUser) int {
	if u == nil || u.Followers == nil {
		return 0
	}
	return u.Followers.TotalCount
}

func followingTotal(u *githubapi.UserUser) int {
	if u == nil || u.Following == nil {
		return 0
	}
	return u.Following.TotalCount
}

func watchingTotal(u *githubapi.UserUser) int {
	if u == nil || u.Watching == nil {
		return 0
	}
	return u.Watching.TotalCount
}

func sponsorshipsAsMaintainerTotal(u *githubapi.UserUser) int {
	if u == nil || u.SponsorshipsAsMaintainer == nil {
		return 0
	}
	return u.SponsorshipsAsMaintainer.TotalCount
}

// 442: contribution* helpers read the lifetime aggregate counters from
// `user.contributionsCollection`. They back the base Activity section
// ("N Commits / N Pull requests reviewed / N Pull requests opened / N
// Issues opened"). Each returns 0 when the collection is nil so a
// degraded GraphQL response hides the row instead of panicking.
func contributionCommits(u *githubapi.UserUser) int {
	if u == nil || u.ContributionsCollection == nil {
		return 0
	}
	return u.ContributionsCollection.TotalCommitContributions
}

func contributionPullRequestReviews(u *githubapi.UserUser) int {
	if u == nil || u.ContributionsCollection == nil {
		return 0
	}
	return u.ContributionsCollection.TotalPullRequestReviewContributions
}

func contributionPullRequests(u *githubapi.UserUser) int {
	if u == nil || u.ContributionsCollection == nil {
		return 0
	}
	return u.ContributionsCollection.TotalPullRequestContributions
}

func contributionIssues(u *githubapi.UserUser) int {
	if u == nil || u.ContributionsCollection == nil {
		return 0
	}
	return u.ContributionsCollection.TotalIssueContributions
}

// 442: issueCommentsTotal / organizationsTotal / sponsorshipsAsSponsorTotal
// / starredRepositoriesTotal read the totalCount sub-field of the
// connections feeding the Activity "issue comments" row and the
// Community-stats rows ("Member of N organizations", "Sponsoring N
// repositories", "Starred N repositories"). Each returns 0 when its
// connection is nil.
func issueCommentsTotal(u *githubapi.UserUser) int {
	if u == nil || u.IssueComments == nil {
		return 0
	}
	return u.IssueComments.TotalCount
}

func organizationsTotal(u *githubapi.UserUser) int {
	if u == nil || u.Organizations == nil {
		return 0
	}
	return u.Organizations.TotalCount
}

func sponsorshipsAsSponsorTotal(u *githubapi.UserUser) int {
	if u == nil || u.SponsorshipsAsSponsor == nil {
		return 0
	}
	return u.SponsorshipsAsSponsor.TotalCount
}

func starredRepositoriesTotal(u *githubapi.UserUser) int {
	if u == nil || u.StarredRepositories == nil {
		return 0
	}
	return u.StarredRepositories.TotalCount
}

// repositoriesContributedToTotal returns the totalCount sub-field from
// `user.repositoriesContributedTo`, or 0 when the connection is nil
// (GraphQL returned no data, or the user has no contributions outside
// their own repos).
func repositoriesContributedToTotal(u *githubapi.UserUser) int {
	if u == nil || u.RepositoriesContributedTo == nil {
		return 0
	}
	return u.RepositoriesContributedTo.TotalCount
}

// baseHeaderCalendarDays is the number of trailing contribution days
// the BaseHeader mini calendar renders as a single horizontal row. 14
// mirrors upstream `core/index.mjs`'s `slice(-14)` over the flattened
// day list (see `base.header.ejs`).
const baseHeaderCalendarDays = 14

// recentContributionDays flattens
// `user.contributionsCollection.contributionCalendar.weeks` into a
// chronological day list and returns the trailing `n` days, mirroring
// upstream `core/index.mjs`'s `slice(-14)`.
//
// The GraphQL connection orders weeks (and days within a week) oldest
// -> newest, so the tail slice gives the most recent N days. When the
// calendar holds fewer than `n` days (fresh account whose history is
// shorter than the requested window) all available days are returned
// untruncated; the BaseHeader partial then renders only the cells that
// exist instead of padding with phantom days. Returns nil when the
// GraphQL payload is missing entirely so the partial hides the block.
func recentContributionDays(u *githubapi.UserUser, n int) []plugins.ContributionDay {
	if u == nil || u.ContributionsCollection == nil || u.ContributionsCollection.ContributionCalendar == nil {
		return nil
	}
	weeks := u.ContributionsCollection.ContributionCalendar.Weeks
	if len(weeks) == 0 {
		return nil
	}
	days := make([]plugins.ContributionDay, 0, len(weeks)*7)
	for _, w := range weeks {
		if w == nil {
			continue
		}
		for _, d := range w.ContributionDays {
			if d == nil {
				continue
			}
			days = append(days, plugins.ContributionDay{
				Date:              d.Date,
				ContributionCount: d.ContributionCount,
				Weekday:           d.Weekday,
				Color:             d.Color,
			})
		}
	}
	if len(days) == 0 {
		return nil
	}
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	return days
}
