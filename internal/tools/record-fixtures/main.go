// Command record-fixtures captures real GitHub GraphQL responses for
// a fixed set of accounts/repos and vendors them under
// tests/fixtures/ as replay fixtures for the layer-2 data tests.
//
// Why record instead of hand-authoring: a hand-written mock body
// tends to be internally consistent (e.g. followers.totalCount equals
// len(nodes)), which masks aggregation bugs like #470 where the card
// must show the true total (59) but the code counts the fetched page
// (24). A response recorded from the real API carries the genuine
// asymmetry, so the bug cannot hide.
//
// This tool hits the live API and therefore needs a token; it is NOT
// run in CI. Re-record intentionally when the upstream query changes
// or the pinned values need refreshing:
//
//	GITHUB_TOKEN=$(gh auth token) go run ./internal/tools/record-fixtures
//
// Each recording POSTs the SAME query file the production code uses
// (internal/githubapi/queries/<file>) so the fixture matches the wire
// shape genqlient expects. Note the circularity caveat: a field the
// active query does not request will be absent from the recording
// too — layer 1 (query field-presence) guards that separately.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// recording describes one fixture to capture.
type recording struct {
	// name is for log output only.
	name string
	// queryFile is relative to internal/githubapi/queries/.
	queryFile string
	// operationName must match the query's operation (genqlient
	// dispatches the replay mux by this).
	operationName string
	// variables are the GraphQL variables for this capture. Volatile
	// values (login, page size) are pinned here so re-records are
	// reproducible.
	variables map[string]any
	// out is the destination relative to tests/fixtures/.
	out string
}

// recordings is the manifest. Add entries as more layer-2 data tests
// need real fixtures.
var recordings = []recording{
	{
		// #470: mjun0812 has 59 followers but the people query fetches
		// only `first: 24`, so the recording pins totalCount=59 with a
		// 24-node array — the asymmetry that exposes the len(nodes)
		// vs totalCount bug.
		name:          "people followers (mjun0812)",
		queryFile:     "user_followers.graphql",
		operationName: "UserFollowers",
		variables:     map[string]any{"login": "mjun0812", "first": 24, "size": 28},
		out:           "github/recordings/user_followers_mjun0812.json",
	},
}

const graphqlEndpoint = "https://api.github.com/graphql"

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "record-fixtures: GITHUB_TOKEN is required")
		fmt.Fprintln(os.Stderr, "  hint: GITHUB_TOKEN=$(gh auth token) go run ./internal/tools/record-fixtures")
		os.Exit(2)
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "record-fixtures: locate repo root: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	for _, rec := range recordings {
		if err := capture(client, token, root, rec); err != nil {
			fmt.Fprintf(os.Stderr, "record-fixtures: %s: %v\n", rec.name, err)
			os.Exit(1)
		}
		fmt.Printf("recorded %s -> tests/fixtures/%s\n", rec.name, rec.out)
	}
}

func capture(client *http.Client, token, root string, rec recording) error {
	queryPath := filepath.Join(root, "internal", "githubapi", "queries", rec.queryFile)
	query, err := os.ReadFile(queryPath) //nolint:gosec // fixed query path
	if err != nil {
		return fmt.Errorf("read query: %w", err)
	}

	reqBody, err := json.Marshal(map[string]any{
		"query":         string(query),
		"variables":     rec.variables,
		"operationName": rec.operationName,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, graphqlEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "github-metrics-record-fixtures")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	// Reject responses carrying GraphQL errors so we never vendor a
	// broken fixture.
	var probe struct {
		Errors []json.RawMessage `json:"errors"`
		Data   json.RawMessage   `json:"data"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if len(probe.Errors) > 0 {
		return fmt.Errorf("response has GraphQL errors: %s", body)
	}
	if len(probe.Data) == 0 {
		return fmt.Errorf("response has no data: %s", body)
	}

	// Pretty-print for human-reviewable fixtures.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		return fmt.Errorf("indent: %w", err)
	}
	pretty.WriteByte('\n')

	outPath := filepath.Join(root, "tests", "fixtures", rec.out)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(outPath, pretty.Bytes(), 0o644); err != nil { //nolint:gosec // test fixture
		return fmt.Errorf("write fixture: %w", err)
	}
	return nil
}

// repoRoot walks up from the working directory to the go.mod marker.
func repoRoot() (string, error) {
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
