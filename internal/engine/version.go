package engine

import "sync"

// version is the build-time-injected release identifier. cmd/* mains
// pass it in through ldflags: `-X github.com/.../internal/engine.version=v0.1.0`.
// Tests substitute via [SetVersionForTest].
//
// Keeping this in a package-level variable rather than a per-binary
// var lets every place that emits a version string (metadata footer,
// User-Agent, --version flag) read the same source.
var (
	versionMu sync.RWMutex
	version   = "dev"
)

// Version returns the current build version.
func Version() string {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return version
}

// TB matches the minimal testing.TB subset we need without forcing
// production callers to import "testing".
type TB interface {
	Helper()
	Cleanup(func())
}

// SetVersionForTest swaps the package-level version under a test
// scope. The original value is restored via t.Cleanup so individual
// tests stay isolated.
func SetVersionForTest(t TB, v string) {
	t.Helper()
	versionMu.Lock()
	prev := version
	version = v
	versionMu.Unlock()
	t.Cleanup(func() {
		versionMu.Lock()
		defer versionMu.Unlock()
		version = prev
	})
}

// SetVersion is the production setter, called by cmd mains at
// initialization when they want to override the linker-injected value
// (e.g. when the binary embeds version metadata from another source).
func SetVersion(v string) {
	versionMu.Lock()
	defer versionMu.Unlock()
	version = v
}
