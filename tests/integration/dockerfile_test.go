//go:build docker_smoke

// Package integration_test docker_smoke verifies the M10 production
// Dockerfile at deploy/Dockerfile. The test builds the image,
// invokes `metrics-action --help` inside a container, and asserts
// the M10 image-size budget per
// specs/008-m10-release-distribution/contracts/dockerfile.md §5.
//
// Gated by the `docker_smoke` build tag so default `make test` skips
// it (chromium + apt install is slow, and not every contributor has
// docker installed). Invoke via `make docker-smoke` or
// `go test -tags=docker_smoke ./tests/integration/...`.
//
// Skips automatically if the docker binary is not on PATH.
package integration_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	// dockerSmokeImageTag is the local-only tag the test builds into.
	// Distinct from the M10 release tag so a polluted local cache from
	// dev iterations does not bleed into release verification.
	dockerSmokeImageTag = "github-metrics:m10-smoke"

	// dockerSmokeSizeBudgetBytes mirrors FR-006 / SC-003: ≤ 900 MB
	// per platform after build. The 900 MB ceiling reflects the v1.0
	// plan-phase escalation documented in research.md R-003 §"Plan-
	// phase risk": the bookworm-slim + chromium + Noto CJK fonts
	// combination measures ~830 MB on the GitHub-hosted ubuntu-latest
	// runner (CI 2026-05-18). Dropping CJK fonts would save ~80 MB
	// but breaks rendering for CJK repository names — not acceptable
	// for v1.0.
	dockerSmokeSizeBudgetBytes = 900 * 1024 * 1024

	// dockerBuildTimeout caps the build step. arm64-via-QEMU builds
	// can take 6-8 min; native amd64 build is typically 2-3 min. The
	// docker_smoke test runs on the host arch only — so the upper
	// bound is generous but not infinite.
	dockerBuildTimeout = 10 * time.Minute

	// dockerRunTimeout caps `metrics-action --help` invocation. Help
	// output is sub-second; this is just a generous safety net.
	dockerRunTimeout = 30 * time.Second
)

// TestDockerfile_BuildRunHelp builds the deploy/Dockerfile image and
// runs `metrics-action --help` inside it.
//
// Verifies M10 acceptance criteria from
// contracts/dockerfile.md §5:
//   - image builds cleanly
//   - `metrics-action --help` exits 0
//   - help output contains either "Usage:" or "metrics-action"
//   - image size is ≤ 900 MB (per FR-006 escalation — see contracts/dockerfile.md §1 Note)
func TestDockerfile_BuildRunHelp(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not on PATH; skipping docker_smoke test")
	}

	root := repoRootFromCWD(t)

	// 1. Build
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), dockerBuildTimeout)
	defer cancelBuild()
	buildCmd := exec.CommandContext(buildCtx, "docker", "build",
		"-f", "deploy/Dockerfile",
		"-t", dockerSmokeImageTag,
		".")
	buildCmd.Dir = root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker build failed: %v\n---\n%s", err, string(out))
	}
	t.Logf("built image %s", dockerSmokeImageTag)

	// 2. Run --help
	runCtx, cancelRun := context.WithTimeout(context.Background(), dockerRunTimeout)
	defer cancelRun()
	runCmd := exec.CommandContext(runCtx, "docker", "run", "--rm",
		dockerSmokeImageTag, "--help")
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run --help failed: %v\n---\n%s", err, string(out))
	}
	helpOut := string(out)
	if !strings.Contains(helpOut, "Usage:") && !strings.Contains(helpOut, "metrics-action") {
		t.Errorf("help output missing expected markers; got:\n%s", helpOut)
	}

	// 3. Size budget assertion
	inspectCmd := exec.Command("docker", "image", "inspect", dockerSmokeImageTag,
		"--format", "{{json .Size}}")
	sizeOut, err := inspectCmd.Output()
	if err != nil {
		t.Fatalf("docker image inspect failed: %v", err)
	}
	var sizeBytes int64
	if err := json.Unmarshal(sizeOut, &sizeBytes); err != nil {
		t.Fatalf("decode image size %q: %v", string(sizeOut), err)
	}
	sizeMB := sizeBytes / 1024 / 1024
	t.Logf("image size: %d MB (budget: %d MB)", sizeMB, dockerSmokeSizeBudgetBytes/1024/1024)
	if sizeBytes > dockerSmokeSizeBudgetBytes {
		t.Errorf("image size %d bytes (%d MB) exceeds budget %d MB (FR-006)",
			sizeBytes, sizeMB, dockerSmokeSizeBudgetBytes/1024/1024)
	}
}

// repoRootFromCWD walks up from the test's CWD until it finds go.mod.
// Tests run with CWD set to their package directory, so the test
// package is several levels deep under the repo root.
func repoRootFromCWD(t *testing.T) string {
	t.Helper()
	dir, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(dir))
}
