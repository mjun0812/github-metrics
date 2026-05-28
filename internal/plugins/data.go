// Package plugins defines the plugin contract that every github-metrics
// data source implements, together with the in-memory Data structure
// the engine builds up as it executes plugins. Concrete plugins live in
// internal/plugins/<name>/; the orchestration that walks them lives in
// internal/plugins/core/.
package plugins

import (
	"sync"
	"time"
)

// AccountKind is the high-level shape of the account the engine is
// rendering metrics for. The base plugin branches on this value.
type AccountKind string

// AccountKind values the engine branches on when deciding which plugin
// dispatch path to take (user / organization / repository).
const (
	AccountUser         AccountKind = "user"
	AccountOrganization AccountKind = "organization"
	AccountRepository   AccountKind = "repository"
)

// Data is the central state object passed through engine.Compute. The
// base plugin populates User; core fills Config and zero-initializes
// Computed; downstream plugins append their results under Plugins.
//
// The structure is intentionally minimal in M1: fields land
// incrementally as later phases need them. Access is goroutine-safe
// through the embedded mutex.
type Data struct {
	mu sync.RWMutex

	Account      AccountKind
	User         *User
	Organization *Organization
	Repo         *Repo // M7: populated when AccountKind == AccountRepository; nil otherwise. Guarded by mu when accessed off the base-plugin goroutine — use RepoRef.
	Config       ComputedConfig
	Computed     Computed
	Plugins      map[string]any
	Errors       []error
}

// RepoRef returns a goroutine-safe snapshot of d.Repo. nil when the
// engine did not request the repository template (M2/M4 classic flow)
// or when the base plugin's repository fetch failed. Callers must not
// mutate the returned *Repo — the M7 contract treats it as
// write-once-read-many.
func (d *Data) RepoRef() *Repo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Repo
}

// SetRepo stores the resolved repository payload. Goroutine-safe;
// intended to be called once by the base plugin before plugin
// dispatch fans out.
func (d *Data) SetRepo(r *Repo) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Repo = r
}

// NewData returns a zero-value Data with the Plugins map initialised.
func NewData() *Data {
	return &Data{Plugins: map[string]any{}}
}

// SetPlugin stores result under name. Goroutine-safe.
func (d *Data) SetPlugin(name string, result any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Plugins == nil {
		d.Plugins = map[string]any{}
	}
	d.Plugins[name] = result
}

// GetPlugin returns the entry under name, if any.
func (d *Data) GetPlugin(name string) (any, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.Plugins[name]
	return v, ok
}

// AppendError records a non-fatal error. Goroutine-safe.
func (d *Data) AppendError(err error) {
	if err == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Errors = append(d.Errors, err)
}

// SnapshotErrors returns a copy of the currently recorded non-fatal
// errors. Goroutine-safe; the returned slice is independent of d.Errors
// so callers can extend or sort it without affecting future appenders.
func (d *Data) SnapshotErrors() []error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.Errors) == 0 {
		return nil
	}
	out := make([]error, len(d.Errors))
	copy(out, d.Errors)
	return out
}

// User is the GraphQL-derived account payload populated by the base
// plugin's runUser branch.
//
// 429 Phase 1: CreatedAt / Followers / Following / Watching /
// SponsorshipsAsMaintainer surface the foundational counters that
// upstream `base.header.ejs` / `base.repositories.ejs` render. Zero
// values mean "GraphQL did not return the counter"; the partials hide
// rows whose source is zero so empty accounts do not gain noise lines.
//
// 429 Phase 2: ContributedTo carries the
// `user.repositoriesContributedTo(...).totalCount` counter rendered by
// upstream `base.header.ejs` as "Contributed to N repositories".
type User struct {
	Login                    string
	Name                     string
	AvatarURL                string
	CreatedAt                time.Time
	Followers                int
	Following                int
	Watching                 int
	SponsorshipsAsMaintainer int
	ContributedTo            int
}

// Organization is the organization-account payload populated by the
// base plugin's runOrganization branch. It mirrors the upstream
// `organization(login)` GraphQL fields the project consumes, including
// the paged member list and the paged repositories accumulator.
type Organization struct {
	Login        string
	Name         string
	Description  string
	AvatarURL    string
	MembersCount int
	Members      []OrgMember
}

// OrgMember is a single organization member entry surfaced by
// runOrganization.
type OrgMember struct {
	Login     string
	Name      string
	AvatarURL string
}

// ComputedConfig captures the core plugin's resolved settings.
type ComputedConfig struct {
	Timezone   TimezoneConfig
	Animations bool
	Display    string
	Base64     bool
	DebugFlags []string
}

// TimezoneConfig records the resolved timezone or the error encountered
// while resolving an unknown IANA name (we keep both fields so the
// engine can surface a warning while still rendering in UTC).
type TimezoneConfig struct {
	Name  string
	Error error
}

// Computed aggregates derived counters. Filled gradually by the base
// plugin (repositories) and individual plugins.
//
// RepositoryList is the per-node accumulator the M4 base plugin
// produces via batch-halving paging; downstream P1/P2 plugins
// (languages, repositories, achievements, ...) consume it. Repositories
// holds the totals derived from the same connection (count, stargazers,
// forks, ...), kept separate so M1 / M2 code that only reads totals
// stays compiling.
type Computed struct {
	Commits        int
	Repositories   ComputedRepositories
	RepositoryList []Repository

	// Indepth fields are populated by base.runIndepth when at least one
	// indepth-dependent plugin (isocalendar, calendar, habits, ...) is
	// enabled. They stay zero-value when indepth is not triggered, or
	// when the indepth GraphQL call returned an error (degraded path).
	TotalCommits         int
	TotalIssues          int
	TotalPullRequests    int
	ContributionCalendar *ContributionCalendar
}

// ComputedRepositories carries the repository-level totals base/run
// produces.
//
// 429 Phase 2: Releases / Packages / DiskUsage / LicensePreference
// surface the per-repo aggregate fields upstream `base.repositories.ejs`
// renders. DiskUsage is in KB (GitHub GraphQL convention); the partial
// promotes to MB / GB / TB via format.FormatDiskKB. LicensePreference
// is the top-N license share, sorted by Count descending.
type ComputedRepositories struct {
	Count             int
	Stargazers        int
	Forks             int
	Releases          int
	Packages          int
	DiskUsage         int
	Watchers          int
	Issues            int
	PullRequests      int
	Languages         map[string]int
	LicensePreference []LicenseShare
}

// LicenseShare is one entry in the License-preference top-N. Percent is
// the share of owned repositories whose `licenseInfo.name` matches Name
// (0..100, never normalised against unlicensed repositories — so the
// figures sum to <= 100 when some repos have no license).
type LicenseShare struct {
	Name    string
	Count   int
	Percent float64
}

// Repository is the per-node entry produced by the base plugin's
// paging loop. Field set mirrors the M4 plugin data model.
type Repository struct {
	NameWithOwner string
	Description   string
	URL           string
	Visibility    string
	IsFork        bool
	Stars         int
	Forks         int
	Watchers      int
	Language      *LanguageStat
	// Languages is the per-language byte breakdown for this repository
	// as reported by GraphQL `repository.languages(first: 8).edges`.
	// Each entry has Size (bytes in this repo) and Name/Color from the
	// linguist palette. Downstream plugins (languages standard mode,
	// achievements) aggregate across all repositories.
	Languages []LanguageStat
}

// LanguageStat is the primary-language summary surfaced per repository.
// Plugins that need full per-byte language breakdowns (T-041 languages
// standard mode) compute their own from this and additional fetches.
type LanguageStat struct {
	Name  string
	Color string
	Size  int
	Count int
	Value float64
}

// ContributionCalendar mirrors GitHub's contributionCalendar payload as
// surfaced by base.runIndepth. Downstream plugins (isocalendar,
// calendar) consume it to compute weekly / yearly summaries.
type ContributionCalendar struct {
	TotalContributions int
	Weeks              []ContributionWeek
}

// ContributionWeek holds the seven daily contribution counts for one
// ISO week.
type ContributionWeek struct {
	FirstDay string
	Days     []ContributionDay
}

// ContributionDay is a single calendar day's contribution count.
type ContributionDay struct {
	Date              string
	ContributionCount int
	Weekday           int
	Color             string
}
