package plugins

// AggregationMode tags the per-plugin Result with which aggregation
// path it took: "user" when computing over data.User (the M2/M4
// classic-template behavior) or "repo" when computing over the
// single-repository data synthesized by internal/dataprovider's
// repository-mode Provider (M7; the legacy base.runRepository
// pipeline was removed in #623).
//
// Plugins call this helper from their Run() to keep the per-plugin
// branching contract in one place. Each of the 6 reused plugins (activity, contributors,
// languages, people, sponsors, stargazers) imports plugins
// already; this constant + helper costs zero extra import edges.
const (
	ModeUser = "user"
	ModeRepo = "repo"
)

// AggregationMode returns ModeRepo when the engine has populated a
// single-repository payload (Data.Repo != nil), ModeUser otherwise.
// Plugins assign the returned value to Result.Mode at the end of
// their Run().
func AggregationMode(d *Data) string {
	if d != nil && d.RepoRef() != nil {
		return ModeRepo
	}
	return ModeUser
}
