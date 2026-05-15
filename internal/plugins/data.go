// Package plugins defines the plugin contract that every github-metrics
// data source implements, together with the in-memory Data structure
// the engine builds up as it executes plugins. Concrete plugins live in
// internal/plugins/<name>/; the orchestration that walks them lives in
// internal/plugins/core/.
package plugins

import "sync"

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

	Account  AccountKind
	User     *User
	Config   ComputedConfig
	Computed Computed
	Plugins  map[string]any
	Errors   []error
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

// User is a stub for the GraphQL-derived account payload. The real
// fields land with the GraphQL client (T036) and the base plugin
// (T054). M1 keeps it open so the engine can plumb a placeholder.
type User struct {
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
type Computed struct {
	Commits      int
	Repositories ComputedRepositories
}

// ComputedRepositories carries the repository-level totals base/run
// produces.
type ComputedRepositories struct {
	Count        int
	Stargazers   int
	Forks        int
	Releases     int
	Watchers     int
	Issues       int
	PullRequests int
	Languages    map[string]int
}
