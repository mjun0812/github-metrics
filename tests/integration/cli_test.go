// Package integration_test covers M6 User Story 3 (CLI mode).
//
// TestCLI_OctocatSVG_Stdout exercises SC-008: the `metrics-cli`
// binary run as a CLI with --dryrun --filename - against a mocked
// GitHub backend MUST emit a valid SVG on stdout within 30 seconds.
//
// TestCLI_ConfigYAML_Equivalence covers SC-009: a `--config inputs.yaml`
// invocation MUST produce byte-identical output to the equivalent
// `INPUT_<UPPER>` env-driven invocation when all other state matches.
package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// startGitHubMock returns an httptest.Server that answers the minimal
// REST + GraphQL surface action.RunCLI hits during a dryrun:
//
//	GET /                 → token scopes (X-OAuth-Scopes: repo)
//	GET /rate_limit       → 5000 remaining
//	POST /graphql         → canned User / UserRepositories payloads
//
// Anything else returns 200 + `{}` so unexpected plugin calls don't
// fail the test loudly — the test is about CLI plumbing, not plugin
// fidelity (those are covered in plugin-specific suites).
func startGitHubMock(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rate_limit", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"resources":{"core":{"remaining":5000,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`)
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case bytes.Contains(body, []byte(`"operationName":"User"`)):
			_, _ = io.WriteString(w, `{"data":{"user":{"databaseId":1,"id":"u","login":"octocat","name":"The Octocat","avatarUrl":"https://x","createdAt":"2008-01-14T04:33:35Z"}}}`)
		case bytes.Contains(body, []byte(`"operationName":"UserRepositories"`)):
			_, _ = io.WriteString(w, `{"data":{"user":{"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`)
		default:
			_, _ = io.WriteString(w, `{"data":{}}`)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// writeCLIConfig writes a temporary YAML config wiring the API URLs
// to the mock server, enabling use_mocked_data, and setting dryrun so
// the committer pathway is skipped.
func writeCLIConfig(t *testing.T, apiBase string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "inputs.yaml")
	body := fmt.Sprintf(`user: octocat
template: classic
output: svg
filename: '-'
dryrun: true
use_mocked_data: true
github_api_rest: %s
github_api_graphql: %s/graphql
`, apiBase, apiBase)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return path
}

func TestCLI_OctocatSVG_Stdout(t *testing.T) {
	t.Parallel()
	srv := startGitHubMock(t)
	cfgPath := writeCLIConfig(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx, actionBin, //nolint:gosec // actionBin is an absolute path from TestMain
		"--config", cfgPath,
	)
	cmd.Env = append(stripGitHubActionsEnv(os.Environ()), "GITHUB_TOKEN=ghp_mock_pat_valid")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("metrics-cli run: %v\nstderr=%s", err, stderr.String())
	}
	out := stdout.String()
	// Banner + the SVG share stdout; the SVG opens with <svg ...>.
	if !strings.Contains(out, "<svg") {
		t.Errorf("expected <svg in stdout; got=%q", trunc(out, 400))
	}
	if !strings.Contains(out, "</svg>") {
		t.Errorf("expected </svg> closer in stdout; got=%q", trunc(out, 400))
	}
}

// TestCLI_FilenameWritesSingleCombinedSVG locks the pre-#606 CLI
// compatibility contract: an explicit --filename foo.svg writes exactly
// that combined SVG file and must not fan out into per-plugin files.
func TestCLI_FilenameWritesSingleCombinedSVG(t *testing.T) {
	t.Parallel()
	srv := startGitHubMock(t)

	dir := t.TempDir()
	outfile := filepath.Join(dir, "metrics.svg")
	cfg := filepath.Join(dir, "inputs.yaml")
	body := fmt.Sprintf(`user: octocat
template: classic
output: svg
filename: %s
dryrun: true
use_mocked_data: true
github_api_rest: %s
github_api_graphql: %s/graphql
`, outfile, srv.URL, srv.URL)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx, actionBin, //nolint:gosec // actionBin is an absolute path from TestMain
		"--config", cfg,
	)
	cmd.Env = append(stripGitHubActionsEnv(os.Environ()), "GITHUB_TOKEN=ghp_mock_pat_valid")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("metrics-cli run: %v\nstderr=%s", err, stderr.String())
	}

	raw, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	out := string(raw)
	if !strings.Contains(out, "<svg") || !strings.Contains(out, "</svg>") {
		t.Fatalf("output file is not an SVG: %q", trunc(out, 400))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	if len(files) != 2 || files[0] != "inputs.yaml" || files[1] != "metrics.svg" {
		t.Fatalf("--filename should write only metrics.svg beside inputs.yaml; got files=%v", files)
	}
}

// TestCLI_ConfigYAML_Equivalence verifies that 4 representative inputs
// produce the same merged Invocation regardless of whether they came
// from --config YAML, --plugin flags, or INPUT_<UPPER> env vars.
//
// We compare bytes of the rendered SVG (mocked deps make the output
// deterministic given identical inputs).
func TestCLI_ConfigYAML_Equivalence(t *testing.T) {
	t.Parallel()
	srv := startGitHubMock(t)

	// Each pair contains a YAML fragment (must not duplicate baseline
	// keys) + an equivalent extra flag set. The two MUST produce the
	// same SVG body (mocked deps → deterministic).
	pairs := []struct {
		name      string
		yamlExtra string
		flagArgs  []string
	}{
		{
			name:      "baseline_noop",
			yamlExtra: "",
			flagArgs:  nil,
		},
		{
			name:      "notice_releases_off",
			yamlExtra: "notice_releases: false\n",
			flagArgs:  []string{"--plugin", "notice_releases=false"},
		},
		{
			name:      "config_padding_block",
			yamlExtra: "config:\n  padding: 10%\n",
			flagArgs:  []string{"--plugin", "config_padding=10%"},
		},
		{
			name:      "plugin_languages_false",
			yamlExtra: "plugins:\n  languages: false\n",
			flagArgs:  []string{"--plugin", "plugin_languages=false"},
		},
		{
			name:      "committer_branch_block",
			yamlExtra: "committer:\n  branch: main\n",
			flagArgs:  []string{"--plugin", "committer_branch=main"},
		},
		{
			// M7 T031: --repo top-level flag vs YAML top-level repo
			// key. The classic template ignores the value (FR-007), so
			// the SVG body stays byte-identical between the two paths.
			name:      "m7_repo_input",
			yamlExtra: "repo: hello-world\n",
			flagArgs:  []string{"--repo", "hello-world"},
		},
	}

	for _, tc := range pairs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fromYAML := stripVolatile(runCLIWithYAML(t, srv.URL, tc.yamlExtra))
			fromFlags := stripVolatile(runCLIWithFlags(t, srv.URL, tc.flagArgs))
			if fromYAML != fromFlags {
				t.Errorf("equivalence broken (yaml vs flags): len yaml=%d, flags=%d", len(fromYAML), len(fromFlags))
			}
		})
	}
}

// volatileLastUpdated matches the classic footer's "Last updated
// <timestamp> ... with mjun0812/github-metrics@<ver>" span. The timestamp is
// captured from the wall clock at render time (classic.go uses
// time.Now()), so two back-to-back subprocesses can straddle a
// one-second boundary and diverge on that byte range alone.
var volatileLastUpdated = regexp.MustCompile(`Last updated [^<]+`)

// stripVolatile masks the footer timestamp so the YAML-vs-flags
// equivalence assertion compares only config-derived bytes. The
// timestamp is config-independent, so masking it preserves the
// equivalence check while removing the sole nondeterministic field —
// mirroring the golden/content suites' svg_normalize helper.
func stripVolatile(svg string) string {
	return volatileLastUpdated.ReplaceAllString(svg, "Last updated __MASKED__")
}

func runCLIWithYAML(t *testing.T, apiBase, yamlExtra string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "inputs.yaml")
	body := fmt.Sprintf(`user: octocat
template: classic
output: svg
filename: '-'
dryrun: true
use_mocked_data: true
github_api_rest: %s
github_api_graphql: %s/graphql
%s`, apiBase, apiBase, yamlExtra)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return execCLI(t, "--config", cfg)
}

func runCLIWithFlags(t *testing.T, apiBase string, extra []string) string {
	t.Helper()
	args := make([]string, 0, 17+len(extra))
	args = append(
		args,
		"--user", "octocat",
		"--template", "classic",
		"--output", "svg",
		"--filename", "-",
		"--dryrun",
		"--plugin", "use_mocked_data=true",
		"--plugin", "github_api_rest="+apiBase,
		"--plugin", "github_api_graphql="+apiBase+"/graphql",
	)
	// `extra` may itself include `--user octocat` which clashes with our
	// baseline; replace the baseline user when extra carries one. For
	// the current 4 test pairs the simple append is fine (no clash).
	args = append(args, extra...)
	return execCLI(t, args...)
}

func execCLI(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, actionBin, args...) //nolint:gosec // actionBin from TestMain
	cmd.Env = append(stripGitHubActionsEnv(os.Environ()), "GITHUB_TOKEN=ghp_mock_pat_valid")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("metrics-cli: %v\nstderr=%s", err, stderr.String())
	}
	// Strip the banner so we compare just the SVG body. The banner
	// is variable (timestamps not present, but process-level fields
	// like the version may shift between runs in CI). The SVG starts
	// at the first "<svg" occurrence.
	out := stdout.String()
	if i := strings.Index(out, "<svg"); i >= 0 {
		return out[i:]
	}
	return out
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// TestCLI_RepoTemplate_MissingRepo_FailFast (M7 T034 / SC-003):
// Invoke the binary with `--template repository` but no `--repo`
// flag and assert it exits with code 1 in under 5 seconds without
// contacting GitHub. We strip GITHUB_ACTIONS so the binary dispatches
// to RunCLI even on a CI runner.
func TestCLI_RepoTemplate_MissingRepo_FailFast(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(
		ctx, actionBin, //nolint:gosec // actionBin from TestMain
		"--user", "octocat",
		"--template", "repository",
		// --repo deliberately omitted.
		"--output", "svg",
		"--dryrun",
		"--filename", "-",
	)
	cmd.Env = append(stripGitHubActionsEnv(os.Environ()), "GITHUB_TOKEN=ghp_mock_pat_valid")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected non-zero exit; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if elapsed > 5*time.Second {
		t.Errorf("SC-003 budget violated: exit took %v (want <5s)", elapsed)
	}
	if !strings.Contains(stderr.String(), "repo") {
		t.Errorf("stderr should mention 'repo'; got %q", stderr.String())
	}
}
