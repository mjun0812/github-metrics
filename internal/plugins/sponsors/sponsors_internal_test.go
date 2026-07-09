package sponsors

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// TestPopulateFromGraphQL_PastTotalUsesServerCounts asserts the past
// sponsor total is derived from the server-side totalCounts (past
// connection = all sponsorships, minus active) instead of the fetched
// page length, which undercounts once past sponsors exceed one page or
// the newest page is dominated by active sponsorships.
func TestPopulateFromGraphQL_PastTotalUsesServerCounts(t *testing.T) {
	t.Parallel()
	out := &Result{}
	resp := &githubapi.ViewerSponsorsResponse{
		Viewer: &githubapi.ViewerSponsorsViewerUser{
			Active: &githubapi.ViewerSponsorsViewerUserActiveSponsorshipConnection{TotalCount: 5},
			Past:   &githubapi.ViewerSponsorsViewerUserPastSponsorshipConnection{TotalCount: 20},
		},
	}
	populateFromGraphQL(out, resp, true)
	if out.Count.Past.Total != 15 {
		t.Errorf("Count.Past.Total = %d, want 15 (past totalCount 20 - active totalCount 5)", out.Count.Past.Total)
	}
	if out.Count.Active.Total != 5 {
		t.Errorf("Count.Active.Total = %d, want 5", out.Count.Active.Total)
	}
}
