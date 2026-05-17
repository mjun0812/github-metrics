package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"
)

// Plugin is the contract every github-metrics data source implements.
// See specs/001-project-foundation/contracts/plugin-interface.md §1
// for the authoritative description.
type Plugin interface {
	Name() string
	Metadata() *config.PluginMetadata
	Run(ctx context.Context, pc *PluginContext) (any, error)
}

// PluginContext groups everything a Run call needs. Engine constructs
// one per request and mutates Data only — every other field is treated
// as read-only by plugin code.
type PluginContext struct {
	Settings   *config.Settings
	Inputs     map[string]any
	Logger     *slog.Logger
	HTTPClient *httpx.Client
	REST       *githubapi.REST
	GraphQL    *githubapi.GraphQL
	Data       *Data
	Metadata   *config.MetadataLoader
	Imports    PluginImports
	// Render carries the engine's renderer. P3 chromedp-dependent
	// plugins (topics / starlists) type-assert this to `*render.Browser`
	// to obtain a navigation surface; the type assertion failing (nil
	// or *render.FakeRenderer) is the documented skip path per
	// specs/004-m4-github-plugins/contracts/plugin-p3-heavy.md §3.4.
	Render render.Renderer
}

// PluginImports lets a plugin read another plugin's published result
// without depending on its package directly.
type PluginImports interface {
	Get(name string) (any, bool)
}

// staticImports is the trivial PluginImports that reads from Data.
type staticImports struct{ data *Data }

// Get implements PluginImports.
func (s staticImports) Get(name string) (any, bool) { return s.data.GetPlugin(name) }

// NewImports returns a PluginImports backed by the given Data.
func NewImports(d *Data) PluginImports { return staticImports{data: d} }

// Registry is the package-level plugin store. Concrete plugin packages
// call Register from their init() functions; the engine resolves
// plugins by name at runtime.
var (
	registryMu sync.RWMutex
	registry   = map[string]Plugin{}
)

// Register adds p to the registry. Duplicate names panic so that two
// init()s racing for the same plugin slug are caught at startup, not
// at request time.
func Register(p Plugin) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := p.Name()
	if name == "" {
		panic("plugins: Register called with empty name")
	}
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("plugins: %q registered twice", name))
	}
	registry[name] = p
}

// Get returns the registered Plugin. The second return value reports
// presence so callers can distinguish "missing" from "registered but
// nil" (which is forbidden by the contract anyway).
func Get(name string) (Plugin, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[name]
	return p, ok
}

// Each iterates registered plugins in sort.Strings(name) order so test
// expectations stay deterministic. fn may return an error to stop.
func Each(fn func(name string, p Plugin) error) error {
	registryMu.RLock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	clone := make(map[string]Plugin, len(registry))
	for k, v := range registry {
		clone[k] = v
	}
	registryMu.RUnlock()

	sort.Strings(keys)
	for _, k := range keys {
		if err := fn(k, clone[k]); err != nil {
			return err
		}
	}
	return nil
}

// TB matches the relevant subset of testing.TB so this file does not
// import "testing" at package level (which would pull the test
// runtime into production binaries).
type TB interface {
	Helper()
	Cleanup(func())
	Fatalf(format string, args ...any)
}

// RegisterForTest temporarily replaces the registry entry for p.Name()
// with p, calling t.Cleanup to restore the previous value (or remove
// the entry) once the test completes.
func RegisterForTest(t TB, p Plugin) {
	t.Helper()
	registryMu.Lock()
	prev, hadPrev := registry[p.Name()]
	registry[p.Name()] = p
	registryMu.Unlock()
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		if hadPrev {
			registry[p.Name()] = prev
		} else {
			delete(registry, p.Name())
		}
	})
}

// Reset clears the registry. Intended for tests that need a clean
// slate; production code MUST NOT call it.
func Reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Plugin{}
}
