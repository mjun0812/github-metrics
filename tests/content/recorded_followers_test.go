package content

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/people"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// Layer 2 of the verification suite: replay a recorded real-API
// response through the actual plugin + partial, and assert the
// rendered content. This is the layer that catches aggregation /
// parsing bugs the hand-authored inline mocks cannot — because the
// recorded fixture carries the genuine data shape.
//
// The fixture under tests/fixtures/github/recordings/ is produced by
// `internal/tools/record-fixtures` from the live API; re-record only
// when intentionally refreshing the pinned values.

var followersLabelRE = regexp.MustCompile(`\d+\s+followers?`)

// TestRecordedFollowersCount pins the people plugin's follower label
// to the true total (#470). The recorded fixture has
// followers.totalCount=59 with a 24-node page (the production query
// fetches first:24). The card must read "59 followers"; the current
// code renders len(nodes)=24 because partial.go counts the fetched
// slice instead of totalCount. A hand-written mock with
// totalCount==len(nodes) could never distinguish the two — only the
// recorded asymmetry does.
func TestRecordedFollowersCount(t *testing.T) {
	mux := mocks.NewGraphQLMux(t)
	mux.OnFile("UserFollowers", "github/recordings/user_followers_mjun0812.json")

	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{"user": "mjun0812", "plugin_people": true}),
	)

	out, err := people.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("people.Run: %v", err)
	}

	data := plugins.NewData()
	data.SetPlugin(people.Name, out)
	rendered, _, err := people.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("people.Partial: %v", err)
	}

	label := followersLabelRE.FindString(rendered)
	if !strings.Contains(rendered, "59 followers") {
		t.Errorf("#470: followers label = %q, want \"59 followers\"\n"+
			"  the recorded fixture has totalCount=59 but a 24-node page;\n"+
			"  the card is counting len(nodes) (= the first:24 limit) instead of totalCount\n"+
			"  → render the count from Followers.TotalCount, not the fetched slice",
			label)
	}
}
