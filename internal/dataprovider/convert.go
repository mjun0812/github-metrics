package dataprovider

import (
	"sort"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// topLicenseShares converts the raw license-name → count map produced by
// the repository paging loop into a top-N slice sorted by Count
// descending, breaking ties alphabetically so the ordering is
// deterministic. Percent is computed against licensedRepos (the number
// of repos that reported a non-nil licenseInfo); when licensedRepos is 0
// the function returns nil. Mirrors the legacy base.topLicenseShares.
func topLicenseShares(counts map[string]int, licensedRepos, limit int) []plugins.LicenseShare {
	if len(counts) == 0 || licensedRepos <= 0 {
		return nil
	}
	if limit <= 0 {
		limit = licensePreferenceTopN
	}
	shares := make([]plugins.LicenseShare, 0, len(counts))
	for name, count := range counts {
		shares = append(shares, plugins.LicenseShare{
			Name:    name,
			Count:   count,
			Percent: float64(count) * 100 / float64(licensedRepos),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Count != shares[j].Count {
			return shares[i].Count > shares[j].Count
		}
		return shares[i].Name < shares[j].Name
	})
	if len(shares) > limit {
		shares = shares[:limit]
	}
	return shares
}

// Generated genqlient types are deeply-nested per-query distinct
// structs; aliasing keeps the call sites readable.
type (
	userRepoNode = githubapi.UserRepositoriesUserRepositoriesRepositoryConnectionNodesRepository
	orgRepoNode  = githubapi.OrganizationRepositoriesOrganizationRepositoriesRepositoryConnectionNodesRepository
)

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func connTotalFollowers(u *githubapi.UserUser) int {
	if u == nil || u.Followers == nil {
		return 0
	}
	return u.Followers.TotalCount
}

func connTotalFollowing(u *githubapi.UserUser) int {
	if u == nil || u.Following == nil {
		return 0
	}
	return u.Following.TotalCount
}

func connTotalWatching(u *githubapi.UserUser) int {
	if u == nil || u.Watching == nil {
		return 0
	}
	return u.Watching.TotalCount
}

// The following helpers wrap each contributionsCollection / connection
// subfield in a nil-safe accessor mirroring the originals in
// internal/plugins/base/base.go. Centralising them here lets the base
// package shrink without losing the degraded-payload semantics
// downstream consumers depend on.

func connTotalSponsorMaintainer(u *githubapi.UserUser) int {
	if u == nil || u.SponsorshipsAsMaintainer == nil {
		return 0
	}
	return u.SponsorshipsAsMaintainer.TotalCount
}

func connTotalIssueComments(u *githubapi.UserUser) int {
	if u == nil || u.IssueComments == nil {
		return 0
	}
	return u.IssueComments.TotalCount
}

func connTotalOrganizations(u *githubapi.UserUser) int {
	if u == nil || u.Organizations == nil {
		return 0
	}
	return u.Organizations.TotalCount
}

func connTotalSponsorshipsAsSponsor(u *githubapi.UserUser) int {
	if u == nil || u.SponsorshipsAsSponsor == nil {
		return 0
	}
	return u.SponsorshipsAsSponsor.TotalCount
}

func connTotalStarred(u *githubapi.UserUser) int {
	if u == nil || u.StarredRepositories == nil {
		return 0
	}
	return u.StarredRepositories.TotalCount
}

func connTotalGists(u *githubapi.UserUser) int {
	if u == nil || u.Gists == nil {
		return 0
	}
	return u.Gists.TotalCount
}

func connTotalDiscussionsStarted(u *githubapi.UserUser) int {
	if u == nil || u.DiscussionsStarted == nil {
		return 0
	}
	return u.DiscussionsStarted.TotalCount
}

func connTotalDiscussionsComments(u *githubapi.UserUser) int {
	if u == nil || u.DiscussionsComments == nil {
		return 0
	}
	return u.DiscussionsComments.TotalCount
}

func connTotalDiscussionAnswers(u *githubapi.UserUser) int {
	if u == nil || u.DiscussionAnswers == nil {
		return 0
	}
	return u.DiscussionAnswers.TotalCount
}

// recentDaysFromWeeks flattens the merged contribution-calendar weeks
// into a chronological day list and returns the trailing n days (mirrors
// upstream `core/index.mjs` `slice(-14)`).
func recentDaysFromWeeks(weeks []plugins.ContributionWeek, n int) []plugins.ContributionDay {
	if len(weeks) == 0 {
		return nil
	}
	days := make([]plugins.ContributionDay, 0, len(weeks)*7)
	for _, w := range weeks {
		days = append(days, w.Days...)
	}
	if len(days) == 0 {
		return nil
	}
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	return days
}

// weeksFromIsocalendar converts a windowed UserIsocalendar calendar
// payload into the plugins.ContributionWeek shape the shared calendar
// accumulator collects.
func weeksFromIsocalendar(weeks []*githubapi.UserIsocalendarUserContributionsCollectionContributionCalendarWeeksContributionCalendarWeek) []plugins.ContributionWeek {
	if len(weeks) == 0 {
		return nil
	}
	out := make([]plugins.ContributionWeek, 0, len(weeks))
	for _, w := range weeks {
		if w == nil {
			continue
		}
		days := make([]plugins.ContributionDay, 0, len(w.ContributionDays))
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
		out = append(out, plugins.ContributionWeek{
			FirstDay: w.FirstDay,
			Days:     days,
		})
	}
	return out
}

// repositoryFromUserNode / repositoryFromOrgNode mirror the base helpers
// of the same name, converting the genqlient repository node into the
// plugins.Repository the per-node accumulator carries.
func repositoryFromUserNode(n *userRepoNode) plugins.Repository {
	r := plugins.Repository{
		NameWithOwner: n.NameWithOwner,
		Description:   derefString(n.Description),
		URL:           n.Url,
		Visibility:    visibilityFromIsPrivate(n.IsPrivate),
		IsFork:        n.IsFork,
		Stars:         n.StargazerCount,
		Forks:         n.ForkCount,
		CreatedAt:     n.CreatedAt,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.Issues != nil {
		r.Issues = n.Issues.TotalCount
	}
	if n.PullRequests != nil {
		r.PullRequests = n.PullRequests.TotalCount
	}
	if n.LicenseInfo != nil {
		r.License = &plugins.RepositoryLicense{
			Name:     n.LicenseInfo.Name,
			SpdxID:   derefString(n.LicenseInfo.SpdxId),
			Nickname: derefString(n.LicenseInfo.Nickname),
		}
	}
	if n.PrimaryLanguage != nil {
		r.Language = &plugins.LanguageStat{
			Name:  n.PrimaryLanguage.Name,
			Color: derefString(n.PrimaryLanguage.Color),
		}
	}
	if n.Languages != nil {
		for _, e := range n.Languages.Edges {
			if e == nil || e.Node == nil {
				continue
			}
			r.Languages = append(r.Languages, plugins.LanguageStat{
				Name:  e.Node.Name,
				Color: derefString(e.Node.Color),
				Size:  e.Size,
			})
		}
	}
	return r
}

func repositoryFromOrgNode(n *orgRepoNode) plugins.Repository {
	r := plugins.Repository{
		NameWithOwner: n.NameWithOwner,
		Description:   derefString(n.Description),
		URL:           n.Url,
		Visibility:    visibilityFromIsPrivate(n.IsPrivate),
		IsFork:        n.IsFork,
		Stars:         n.StargazerCount,
		Forks:         n.ForkCount,
		CreatedAt:     n.CreatedAt,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.Issues != nil {
		r.Issues = n.Issues.TotalCount
	}
	if n.PullRequests != nil {
		r.PullRequests = n.PullRequests.TotalCount
	}
	if n.LicenseInfo != nil {
		r.License = &plugins.RepositoryLicense{
			Name:     n.LicenseInfo.Name,
			SpdxID:   derefString(n.LicenseInfo.SpdxId),
			Nickname: derefString(n.LicenseInfo.Nickname),
		}
	}
	if n.PrimaryLanguage != nil {
		r.Language = &plugins.LanguageStat{
			Name:  n.PrimaryLanguage.Name,
			Color: derefString(n.PrimaryLanguage.Color),
		}
	}
	if n.Languages != nil {
		for _, e := range n.Languages.Edges {
			if e == nil || e.Node == nil {
				continue
			}
			r.Languages = append(r.Languages, plugins.LanguageStat{
				Name:  e.Node.Name,
				Color: derefString(e.Node.Color),
				Size:  e.Size,
			})
		}
	}
	return r
}

func visibilityFromIsPrivate(isPrivate bool) string {
	if isPrivate {
		return "private"
	}
	return "public"
}
