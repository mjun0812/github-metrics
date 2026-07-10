package mocks_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestNewPluginContext_Defaults(t *testing.T) {
	t.Parallel()
	pc := mocks.NewPluginContext(t)
	if got := pc.Inputs["user"]; got != "octocat" {
		t.Errorf("default Inputs[user] = %v, want octocat", got)
	}
	if pc.Data == nil {
		t.Errorf("default Data is nil")
	}
	if pc.Logger == nil {
		t.Errorf("default Logger is nil")
	}
	if pc.GraphQL != nil {
		t.Errorf("GraphQL should be nil without WithGraphQL option")
	}
	if pc.REST != nil {
		t.Errorf("REST should be nil without WithREST option")
	}
}

func TestNewPluginContext_WithGraphQLAndREST(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	rest := mocks.NewRESTMux(t)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(gql),
		mocks.WithREST(rest),
	)
	if pc.GraphQL == nil {
		t.Errorf("GraphQL client missing after WithGraphQL")
	}
	if pc.REST == nil {
		t.Errorf("REST client missing after WithREST")
	}
}

func TestNewPluginContext_WithInputs_Overrides(t *testing.T) {
	t.Parallel()
	pc := mocks.NewPluginContext(
		t,
		mocks.WithInputs(map[string]any{"user": "alice", "repo": "x"}),
	)
	if pc.Inputs["user"] != "alice" {
		t.Errorf("Inputs[user] = %v, want alice", pc.Inputs["user"])
	}
	if pc.Inputs["repo"] != "x" {
		t.Errorf("Inputs[repo] = %v, want x", pc.Inputs["repo"])
	}
}

func TestNewPluginContext_WithData_Overrides(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.User = &plugins.User{Login: "preset"}
	pc := mocks.NewPluginContext(t, mocks.WithData(d))
	if pc.Data == nil || pc.Data.User == nil || pc.Data.User.Login != "preset" {
		t.Errorf("custom Data not propagated; got %+v", pc.Data)
	}
}
