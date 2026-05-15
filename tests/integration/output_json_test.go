package integration_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// updateGolden allows `go test -update` to regenerate golden files.
// The same flag is reused across every golden test file.
var updateGolden = flag.Bool("update", false, "regenerate golden test fixtures from current output")

// TestComputeJSON_OctocatGolden runs engine.Compute against the
// mocked-GraphQL deps from the M1 foundation tests and compares the
// resulting JSON payload to tests/golden/json/octocat.json. The
// golden file is checked into git; -update regenerates it.
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

	goldenPath := filepath.Join("..", "golden", "json", "octocat.json")
	if *updateGolden {
		if mkErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750); mkErr != nil {
			t.Fatalf("mkdir golden: %v", mkErr)
		}
		var box any
		if dErr := json.Unmarshal(res.Output, &box); dErr != nil {
			t.Fatalf("re-decode for indent: %v", dErr)
		}
		pretty, mErr := json.MarshalIndent(box, "", "  ")
		if mErr != nil {
			t.Fatalf("indent: %v", mErr)
		}
		pretty = append(pretty, '\n')
		if wErr := os.WriteFile(goldenPath, pretty, 0o600); wErr != nil {
			t.Fatalf("write golden: %v", wErr)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if !jsonEq(t, want, res.Output) {
		t.Fatalf("JSON output diverged from golden\n--- want hash ---\n%s\n--- got hash ---\n%s\n--- got ---\n%s",
			hashOf(want), hashOf(res.Output), string(res.Output))
	}
}

func jsonEq(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("decode A: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("decode B: %v", err)
	}
	ac, _ := json.Marshal(av)
	bc, _ := json.Marshal(bv)
	return string(ac) == string(bc)
}

func hashOf(b []byte) string {
	h := md5.Sum(b) //nolint:gosec // not security-sensitive; used for human diff hints
	return hex.EncodeToString(h[:])
}
