package config_test

import (
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/assets"
	"github.com/mjun0812/github-metrics/internal/config"
)

// TestEmbeddedMetadataLoads ensures the //go:embed bundle in the
// "assets" package is wired into LoadMetadata exactly the same way the
// engine will consume it at runtime.
func TestEmbeddedMetadataLoads(t *testing.T) {
	t.Parallel()

	fsys := assets.FS()
	ml, err := config.LoadMetadata(fsys, config.LoadOptions{
		VersionPath: "version.txt",
	})
	if err != nil {
		t.Fatalf("LoadMetadata(embedded): %v", err)
	}

	if len(ml.Plugins) != len(adoptedPlugins) {
		t.Fatalf("Plugins count = %d, want %d", len(ml.Plugins), len(adoptedPlugins))
	}
	if len(ml.Templates) != len(adoptedTemplates) {
		t.Fatalf("Templates count = %d, want %d", len(ml.Templates), len(adoptedTemplates))
	}
	if ml.Package == nil || ml.Package.UpstreamVersion == "" {
		t.Fatalf("Package.UpstreamVersion missing")
	}
}

func TestEmbeddedMetadataLoadsUnder200ms(t *testing.T) {
	t.Parallel()

	if raceEnabled {
		t.Skip("race detector overhead invalidates the 200ms budget (SC-003); covered by the non-race test jobs")
	}

	fsys := assets.FS()

	const budget = 200 * time.Millisecond
	start := time.Now()
	if _, err := config.LoadMetadata(fsys, config.LoadOptions{
		VersionPath: "version.txt",
	}); err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > budget {
		t.Fatalf("embedded load took %v, budget %v (SC-003)", elapsed, budget)
	}
	t.Logf("embedded load elapsed: %v", elapsed)
}

func BenchmarkLoadMetadata_Embedded(b *testing.B) {
	fsys := assets.FS()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := config.LoadMetadata(fsys, config.LoadOptions{
			VersionPath: "version.txt",
		}); err != nil {
			b.Fatalf("LoadMetadata: %v", err)
		}
	}
}
