package plugins

import "time"

// Repo carries the single-repository payload populated by the base
// plugin when the engine resolves a `repository` template request. It
// is the M7 analogue of [User] for the M2 classic template: nil when
// no repository template was requested, populated exactly once per run
// before plugin dispatch fans out so downstream consumers can read it
// lock-free (the M7 contract guarantees no writes after population).
//
// Field names mirror upstream `template.mjs:13-21` semantics.
type Repo struct {
	// Owner is the repository owner's login (the part before `/` in
	// `<owner>/<name>`).
	Owner string
	// OwnerAvatar is the avatar URL of the owner — sized for the
	// `base.header` partial's avatar slot.
	OwnerAvatar string
	// Name is the repository name (the part after `/`).
	Name string
	// Description is the repo's about/description text (may be empty).
	Description string
	// CreatedAt is `repository.createdAt` — backs the base.header
	// "Created N years ago" field (#464).
	CreatedAt time.Time
	// DiskUsageKB is `repository.diskUsage` (KiB, GitHub GraphQL
	// convention) — backs the base.header "N MB used" field (#464).
	DiskUsageKB int
	// Deployments is `repository.deployments.totalCount` — backs the
	// base.header "Deployed N times" field (#464).
	Deployments int
	// Environments is `repository.environments.totalCount` — backs the
	// base.header "N Environments" field (#464).
	Environments int
	// Calendar holds the trailing contribution days (copied from the
	// user payload) the base.header mini calendar renders (#464).
	Calendar []ContributionDay
	// Stargazers is `repository.stargazerCount`.
	Stargazers int
	// Forks is `repository.forkCount`.
	Forks int
	// Contributors is the REST `listContributors` count (the GraphQL
	// API does not expose this directly).
	Contributors int
	// PrimaryLanguage is the GraphQL `primaryLanguage.name`; may be
	// empty for repos with no detected language.
	PrimaryLanguage string
	// PrimaryLanguageColor is `primaryLanguage.color` — used by the
	// languages partial to render the language dot.
	PrimaryLanguageColor string
	// LicenseName is the GraphQL `licenseInfo.name` (e.g. "MIT
	// License"); empty when no license is declared.
	LicenseName string
	// DefaultBranch is `defaultBranchRef.name` (e.g. "main"); used by
	// the committer when the workflow does not pin one explicitly.
	DefaultBranch string
	// IsArchived flips on for archived repos so partials can either
	// surface a badge or quietly skip the activity section.
	IsArchived bool
	// SponsorshipsAsMaintainer mirrors `template.mjs:21` —
	// `data.repo.sponsorshipsAsMaintainer = data.user.sponsorshipsAsMaintainer`.
	// Copied from the user payload by the base plugin.
	SponsorshipsAsMaintainer int
	// Activity carries the recent commits / open issues / open PRs
	// counts the `base.activity` and `activity` partials consume.
	Activity RepoActivity
}

// RepoActivity is the per-repo activity rollup. Populated alongside
// [Repo] by the base plugin's repository fetch path.
type RepoActivity struct {
	// RecentCommits is the count of commits in the last 30 days,
	// derived from REST `listCommits?per_page=100&since=<30 days ago>`.
	RecentCommits int
	// OpenIssues is the GraphQL `issues(states: OPEN).totalCount`.
	OpenIssues int
	// OpenPullRequests is the GraphQL `pullRequests(states: OPEN).totalCount`.
	OpenPullRequests int
}
