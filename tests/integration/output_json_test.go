package integration_test

import (
	"context"
	"encoding/json"
	"flag"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"
)

// updateGolden is the project-wide `-update` flag declared in this
// file. `internal/testutil/golden.Compare*` looks it up via
// `flag.Lookup("update")`, so every golden test in the project
// picks up the same flag without re-declaring it.
var updateGolden = flag.Bool("update", false, "regenerate golden test fixtures from current output")

// TestComputeJSON_OctocatGolden runs engine.Compute against the
// mocked-GraphQL deps from the M1 foundation tests and compares the
// resulting JSON payload to tests/golden/json/octocat.json via the
// M9 shared `golden.CompareJSON` helper.
func TestComputeJSON_OctocatGolden(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:  "octocat",
		Format: "json",
		// The golden was captured with the v2 default-all section
		// gate. v3.0 (#649) requires explicit chrome_* opt-in for
		// the section set; populate base.Run by enabling at least
		// one of activity / community / repositories so the JSON
		// envelope's base.profile remains populated.
		Inputs: map[string]any{
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "application/json" {
		t.Fatalf("MIME = %q, want application/json", res.MIME)
	}
	if !json.Valid(res.Output) {
		t.Fatalf("Output is not valid JSON: %s", res.Output)
	}

	golden.CompareJSON(t, res.Output, "json/octocat.json")
}

// Keep `updateGolden` referenced — its declaration is the project's
// canonical -update flag (see comment above). Other golden tests
// look it up via flag.Lookup.
var _ = updateGolden
