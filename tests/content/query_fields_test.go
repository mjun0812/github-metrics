package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Layer 1 of the verification suite: GraphQL query field-presence.
//
// Several #463–#472 bugs are "the card never shows X" where the root
// cause is that our active GraphQL query never *requests* X — so no
// amount of template work can surface it, and a recorded-response
// fixture (layer 2) recorded from our own query would reproduce the
// gap silently. This test pins the field set our queries under
// internal/githubapi/queries/ must request, witnessed by the upstream
// queries vendored under assets/plugins/*/queries/.
//
// It is deliberately at the source-of-truth layer: when a content
// test (layer 3) goes red, this test tells you whether the data was
// never fetched (query gap, fix here) or fetched-but-not-rendered
// (template gap, query stays green).

// queryFieldReq declares the fields one active query must request.
type queryFieldReq struct {
	// file is relative to internal/githubapi/queries/.
	file  string
	issue string
	// required field selectors that must appear in the query text.
	required []string
	// note explains the witness / intent for failure triage.
	note string
}

var queryFieldReqs = []queryFieldReq{
	{
		// #469: starred repo cards lack license / PR info because the
		// query omits them. Upstream assets/plugins/stars/queries/
		// stars.graphql requests both.
		file:     "user_starred.graphql",
		issue:    "#469",
		required: []string{"licenseInfo", "pullRequests"},
		note:     "witnessed by assets/plugins/stars/queries/stars.graphql",
	},
	{
		// #466: the "created <date>" line is absent from the
		// repositories card because the query omits createdAt.
		// (licenseInfo IS already requested here — so the missing
		// license in the render is a template gap, not a query gap;
		// we lock licenseInfo in as a regression guard.)
		file:     "user_repositories.graphql",
		issue:    "#466",
		required: []string{"createdAt", "licenseInfo"},
		note:     "witnessed by assets/plugins/repositories/queries/repository.graphql",
	},
	{
		// #472 regression guard: reactions ARE fetched correctly, so
		// this passes — pinning it documents that an empty reactions
		// card is a render/empty-guard bug, not a query gap.
		file:     "user_reactions.graphql",
		issue:    "#472",
		required: []string{"reactions", "content"},
		note:     "reactions data is fetched; empty card is a render-layer bug",
	},
}

func readQuery(t *testing.T, file string) string {
	t.Helper()
	p := filepath.Join(repoRoot(t), "internal", "githubapi", "queries", file)
	b, err := os.ReadFile(p) //nolint:gosec // fixed query path under the repo
	if err != nil {
		t.Fatalf("read query %s: %v", file, err)
	}
	return string(b)
}

// TestQueryRequestsRequiredFields enforces layer 1: every required
// field must be present in the active GraphQL query.
func TestQueryRequestsRequiredFields(t *testing.T) {
	for _, req := range queryFieldReqs {
		t.Run(req.file, func(t *testing.T) {
			query := readQuery(t, req.file)
			for _, field := range req.required {
				if !strings.Contains(query, field) {
					t.Errorf("%s: query %s does not request %q\n"+
						"  %s\n"+
						"  → the field is never fetched; add it to the query (no template change can surface it)",
						req.issue, req.file, field, req.note)
				}
			}
		})
	}
}
