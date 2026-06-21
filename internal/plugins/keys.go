package plugins

// DataKey identifies a Provider data source a plugin reads at Run time.
// Each plugin declares the set of keys it needs via Requires(); a
// per-plugin counting-mock test (internal/plugins/<name>) asserts the
// declared set equals the actually-invoked set so a drive-by addition
// of pc.Provider.CommitCalendar(ctx) without a matching Requires()
// entry fails CI.
//
// Note: DataKey deliberately lives in package plugins (not in
// internal/dataprovider) to avoid a circular import — plugins.Plugin
// embeds Requires() []DataKey, and dataprovider already imports
// plugins for the Provider / Profile / Repository types.
type DataKey string

// DataKey values returned by Plugin.Requires(). The five constants
// mirror the five methods on plugins.Provider one-to-one.
const (
	// KeyProfile -> Provider.Profile(ctx). Account-kind-agnostic;
	// prefer this when the plugin works for both user and org
	// accounts.
	KeyProfile DataKey = "profile"
	// KeyUser -> Provider.User(ctx). Fails on organization accounts;
	// declare only when the plugin genuinely needs the user-specific
	// shape (e.g., contributions counts).
	KeyUser DataKey = "user"
	// KeyOrganization -> Provider.Organization(ctx). Fails on user
	// accounts; declare only when the plugin needs org-specific
	// fields.
	KeyOrganization DataKey = "organization"
	// KeyRepositories -> Provider.Repositories(ctx).
	KeyRepositories DataKey = "repositories"
	// KeyCommitCalendar -> Provider.CommitCalendar(ctx).
	KeyCommitCalendar DataKey = "commit_calendar"
)
