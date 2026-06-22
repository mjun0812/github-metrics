package plugins

// DataKey identifies a single Provider method call. Every plugin declares
// which keys it needs via Plugin.Requires() so tooling and tests can assert
// the declared dependency set equals the set of methods actually called at
// runtime.
//
// The canonical mapping is:
//
//	KeyProfile        -> Provider.Profile(ctx)   -- user OR organization
//	KeyUser           -> Provider.User(ctx)       -- user account only
//	KeyOrganization   -> Provider.Organization(ctx) -- org account only
//	KeyRepositories   -> Provider.Repositories(ctx)
//	KeyCommitCalendar -> Provider.CommitCalendar(ctx)
//
// Most plugins declare KeyProfile (account-kind-agnostic). KeyUser /
// KeyOrganization are escape hatches reserved for plugins that genuinely
// require exactly one account kind; none of the current adopted plugins
// use them, but they are provided for future use.
type DataKey string

const (
	// KeyProfile maps to Provider.Profile(ctx). Use when a plugin works
	// with both user and organisation accounts interchangeably.
	KeyProfile DataKey = "profile"

	// KeyUser maps to Provider.User(ctx). Fails when the account is an
	// organisation. Prefer KeyProfile unless the plugin is user-only.
	KeyUser DataKey = "user"

	// KeyOrganization maps to Provider.Organization(ctx). Fails when the
	// account is a user. Prefer KeyProfile unless the plugin is org-only.
	KeyOrganization DataKey = "organization"

	// KeyRepositories maps to Provider.Repositories(ctx).
	KeyRepositories DataKey = "repositories"

	// KeyCommitCalendar maps to Provider.CommitCalendar(ctx).
	KeyCommitCalendar DataKey = "commit_calendar"
)
