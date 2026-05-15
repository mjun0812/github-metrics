// Command sync-fixtures captures the upstream lowlighter/metrics
// output for a given test case (e.g. octocat) and writes the JSON to
// tests/fixtures/upstream/<login>.json. The captured fixture is the
// source of truth for the SC-001 / FR-018 upstream-compatibility test.
//
// Approach:
//   - Read the requested input from ./org_repo/tests/cases/<login>.yml
//     (the upstream maintains canonical test inputs there).
//   - Spawn the upstream Node test runner (`npm test --silent --
//     --grep "<login>"`) inside ./org_repo to materialize a metrics.json
//     output. Cache the captured file under tests/fixtures/upstream/.
//
// Exit codes:
//
//	0  fixture written (or absent-but-soft-skipped)
//	1  invocation / I/O error
//	2  org_repo missing — soft skip path (CI without the upstream
//	   checkout simply does not regenerate the fixture)
//
// The companion test in tests/compatibility/json_test.go gracefully
// skips when the fixture is absent.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// safeLogin restricts the --user flag to a GitHub-login-shaped value
// so the path joins below cannot escape tests/fixtures/upstream/.
var safeLogin = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

func main() {
	user := flag.String("user", "octocat", "test case login (matches ./org_repo/tests/cases/<user>.yml)")
	full := flag.Bool("full", false, "enable all 21 adopted plugins (M4) via METRICS_FIXTURE_FULL=1 env to upstream npm test")
	flag.Parse()

	if !safeLogin.MatchString(*user) {
		fmt.Fprintf(os.Stderr, "sync-fixtures: --user %q is not a valid login\n", *user)
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: %v\n", err)
		os.Exit(1)
	}
	orgRepo := filepath.Join(repoRoot, "org_repo")
	if _, statErr := os.Stat(orgRepo); statErr != nil {
		fmt.Fprintln(os.Stderr, "sync-fixtures: ./org_repo not present; skipping (CI-friendly soft pass).")
		os.Exit(2)
	}
	caseFile := filepath.Join(orgRepo, "tests", "cases", fmt.Sprintf("%s.yml", *user))
	if _, statErr := os.Stat(caseFile); statErr != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: no upstream test case for %q at %s\n", *user, caseFile)
		os.Exit(1)
	}

	fixturesDir := filepath.Join(repoRoot, "tests", "fixtures", "upstream")
	if mkErr := os.MkdirAll(fixturesDir, 0o750); mkErr != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: %v\n", mkErr)
		os.Exit(1)
	}
	dest := filepath.Join(fixturesDir, fmt.Sprintf("%s.json", *user))

	// Invoke upstream Node test runner. We rely on `npm test` to
	// materialize a metrics.json artifact under
	// ./org_repo/tests/artifacts/<user>/metrics.json. The exact path
	// depends on upstream conventions — refer to the README in
	// ./org_repo/tests/ when adjusting.
	cmd := buildNpmCommand(orgRepo, *user, *full)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runErr := cmd.Run(); runErr != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: upstream `npm test` failed: %v\n", runErr)
		fmt.Fprintln(os.Stderr, "             ensure `cd org_repo && npm install` ran first.")
		os.Exit(1)
	}

	src := filepath.Join(orgRepo, "tests", "artifacts", *user, "metrics.json")
	if _, statErr := os.Stat(src); statErr != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: expected upstream artifact at %s; not found.\n", src)
		fmt.Fprintln(os.Stderr, "             upstream conventions may have changed; inspect ./org_repo/tests/.")
		os.Exit(1)
	}
	body, readErr := os.ReadFile(src) //nolint:gosec // src is the upstream-controlled artifact path
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "sync-fixtures: %v\n", readErr)
		os.Exit(1)
	}
	// dest is rooted at the project's tests/fixtures/upstream/ and the
	// filename component is constrained by safeLogin above.
	if writeErr := os.WriteFile(dest, body, 0o600); writeErr != nil { //nolint:gosec // dest path is constrained
		fmt.Fprintf(os.Stderr, "sync-fixtures: %v\n", writeErr)
		os.Exit(1)
	}
	fmt.Printf("sync-fixtures: wrote %s (%d bytes)\n", dest, len(body))
}

// buildNpmCommand assembles the `npm test --grep <user>` invocation
// for the upstream test runner. When full is true the
// METRICS_FIXTURE_FULL=1 env var is appended so the upstream YAML
// loader knows to enable every adopted M4 plugin. Extracted to allow
// unit tests to assert the env / argv shape without spawning npm.
func buildNpmCommand(orgRepo, user string, full bool) *exec.Cmd {
	cmd := exec.Command("npm", "test", "--silent", "--", "--grep", user) //nolint:gosec // user passed safeLogin check upstream
	cmd.Dir = orgRepo
	cmd.Env = os.Environ()
	if full {
		cmd.Env = append(cmd.Env, "METRICS_FIXTURE_FULL=1")
	}
	return cmd
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
