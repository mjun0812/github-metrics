package integration_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"

	// Side-effect import: registers classic.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// BenchmarkCompute_JSON_Octocat measures the per-call cost of running
// engine.Compute against the mocked GraphQL deps and serializing to
// JSON. Target (SC-003) is < 2 s / op on a contributor laptop; in
// practice the call runs in single-digit milliseconds because no real
// network I/O happens.
func BenchmarkCompute_JSON_Octocat(b *testing.B) {
	engine.SetVersionForTest(b, "bench-version")
	deps, _ := newEngineDeps(b, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	req := engine.Request{Login: "octocat", Format: "json"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Compute(context.Background(), req, deps); err != nil {
			b.Fatalf("Compute: %v", err)
		}
	}
}

// BenchmarkCompute_SVG_Classic measures the same Compute path but
// through the classic template's SVG render. Useful as a regression
// signal when partial / template work changes.
func BenchmarkCompute_SVG_Classic(b *testing.B) {
	engine.SetVersionForTest(b, "bench-version")
	deps, _ := newEngineDeps(b, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	req := engine.Request{Login: "octocat", Template: "classic", Format: "svg"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Compute(context.Background(), req, deps); err != nil {
			b.Fatalf("Compute: %v", err)
		}
	}
}
