// Package integration_test exercises the metrics-action and metrics-cli
// binaries end-to-end. The build is performed once in TestMain to keep the
// individual cases independent and fast.
package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	actionBin string
	cliBin    string
)

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

	actionBin = filepath.Join(tmp, "metrics-action"+exeSuffix())
	cliBin = filepath.Join(tmp, "metrics-cli"+exeSuffix())

	for _, b := range []struct {
		out, pkg string
	}{
		{actionBin, "./cmd/metrics-action"},
		{cliBin, "./cmd/metrics-cli"},
	} {
		cmd := exec.Command("go", "build", "-o", b.out, b.pkg) //nolint:gosec // package paths are constants
		cmd.Dir = repoRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: build %s: %v\n", b.pkg, err)
			return 2
		}
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

// runBin runs the given binary with args and returns stdout, stderr, exit code.
// The child env strips GITHUB_ACTIONS so the binary dispatches to CLI mode
// regardless of whether the test itself runs under GitHub Actions (where
// GITHUB_ACTIONS=true is baked into os.Environ).
func runBin(t *testing.T, bin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = stripGitHubActionsEnv(os.Environ())
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if !asExitError(err, &exitErr) {
			t.Fatalf("run %s %v: unexpected error: %v", bin, args, err)
		}
		exitCode = exitErr.ExitCode()
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// stripGitHubActionsEnv removes GITHUB_ACTIONS (and the related
// CI / RUNNER_OS marker) so the spawned binary doesn't route to
// Action mode when the test happens to run on a GitHub Actions
// runner. Without this, every integration test inherits
// GITHUB_ACTIONS=true and hits action.Run (which requires INPUT_*)
// instead of action.RunCLI (which the tests are exercising).
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

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

func TestBinariesHelpExitsZeroWithUsage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		bin  string
	}{
		{"metrics-action", actionBin},
		{"metrics-cli", cliBin},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"_--help", func(t *testing.T) {
			t.Parallel()
			stdout, _, code := runBin(t, tc.bin, "--help")
			if code != 0 {
				t.Fatalf("%s --help exit code = %d, want 0", tc.name, code)
			}
			if !strings.Contains(stdout, "Usage:") {
				t.Fatalf("%s --help stdout missing 'Usage:'\ngot: %q", tc.name, stdout)
			}
		})
		t.Run(tc.name+"_default", func(t *testing.T) {
			t.Parallel()
			stdout, _, code := runBin(t, tc.bin)
			if code != 0 {
				t.Fatalf("%s (no args) exit code = %d, want 0", tc.name, code)
			}
			if !strings.Contains(stdout, "Usage:") {
				t.Fatalf("%s (no args) stdout missing 'Usage:'\ngot: %q", tc.name, stdout)
			}
		})
	}
}

func TestBinariesVersionPrintsVersionString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		bin  string
	}{
		{"metrics-action", actionBin},
		{"metrics-cli", cliBin},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"_--version", func(t *testing.T) {
			t.Parallel()
			stdout, _, code := runBin(t, tc.bin, "--version")
			if code != 0 {
				t.Fatalf("%s --version exit code = %d, want 0", tc.name, code)
			}
			// The version is overridden via -ldflags at release time. The
			// integration test does not pass ldflags, so the default
			// "dev" string applies.
			if got := strings.TrimSpace(stdout); got != "dev" {
				t.Fatalf("%s --version stdout = %q, want %q", tc.name, got, "dev")
			}
		})
	}
}

func TestBinariesUnknownFlagExitsNonZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		bin  string
	}{
		{"metrics-action", actionBin},
		{"metrics-cli", cliBin},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"_unknown_flag", func(t *testing.T) {
			t.Parallel()
			_, _, code := runBin(t, tc.bin, "--nope")
			if code == 0 {
				t.Fatalf("%s --nope exit code = 0, want non-zero (flag parse error)", tc.name)
			}
		})
	}
}
