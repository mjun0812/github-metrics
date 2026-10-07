// Package integration_test exercises the metrics-cli binary end-to-end.
// The build is performed once in TestMain to keep the individual cases
// independent and fast.
package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var actionBin string

func TestMain(m *testing.M) {
	// Use an indirection so that defer-based cleanup runs before we exit.
	// gocritic flags `defer os.RemoveAll(tmp)` followed by `os.Exit` because
	// the defer would never fire on the error paths.
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "metrics-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: create tempdir: %v\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: locate repo root: %v\n", err)
		return 2
	}

	actionBin = filepath.Join(tmp, "metrics-cli"+exeSuffix())
	cmd := exec.Command("go", "build", "-o", actionBin, "./cmd/metrics-cli") //nolint:gosec // package path is a constant
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build ./cmd/metrics-cli: %v\n", err)
		return 2
	}

	return m.Run()
}

// findRepoRoot walks upward from the test working directory until it finds
// the go.mod file. This keeps the test self-locating regardless of where
// `go test` is invoked from.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// stripGitHubActionsEnv removes GITHUB_ACTIONS from the child env.
// The marker has no behavioural effect after the v3.0 mode unification
// (#646) — the binary no longer consults it — but stripping it keeps
// the test invocation environment minimal so any future regression
// that re-introduces env-based dispatch is caught here.
func stripGitHubActionsEnv(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "GITHUB_ACTIONS=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
