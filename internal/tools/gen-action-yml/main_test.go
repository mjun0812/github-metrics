package main

import (
	"os"
	"strings"
	"testing"
)

// TestGenerate_MatchesCommittedActionYML is the action.yml drift gate:
// the committed action.yml must equal the generator output under the
// same conditions as `make gen-action-yml` (no VERSION, so the
// local-Dockerfile image line). Re-run `make gen-action-yml` when this
// fails.
func TestGenerate_MatchesCommittedActionYML(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	committed, err := os.ReadFile("../../../action.yml")
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	if body != string(committed) {
		t.Errorf("action.yml is out of date with the generator; run `make gen-action-yml` and commit the result")
	}
}

// TestGenerate_VersionedImageRef confirms a semver VERSION env value
// emits the docker:// reference pinned to the published GHCR tag.
func TestGenerate_VersionedImageRef(t *testing.T) {
	t.Parallel()
	body, err := generate("../../../assets", "v1.0.0")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want := "image: 'docker://ghcr.io/mjun0812/github-metrics:v1.0.0'"
	if !strings.Contains(body, want) {
		t.Errorf("generated action.yml missing versioned image ref %q", want)
	}
	if strings.Contains(body, "image: 'Dockerfile'") {
		t.Errorf("generated action.yml still contains local Dockerfile fallback when VERSION set")
	}
}

// TestGenerate_Deterministic confirms two consecutive runs produce
// byte-identical output (drift gate prerequisite). Tests both the
// pre-release path (empty VERSION) and the pinned-release path
// (VERSION=v1.0.0) so neither toggle introduces randomness.
func TestGenerate_Deterministic(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"", "v1.0.0"} {
		a, err := generate("../../../assets", version)
		if err != nil {
			t.Fatal(err)
		}
		b, err := generate("../../../assets", version)
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Errorf("two runs with version=%q produced different output", version)
		}
	}
}
