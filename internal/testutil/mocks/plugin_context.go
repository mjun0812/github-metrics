package mocks

import (
	"io"
	"log/slog"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// PluginContextOption is the functional-options surface for
// NewPluginContext.
type PluginContextOption func(*pluginContextConfig)

type pluginContextConfig struct {
	graphQLMux *GraphQLMux
	restMux    *RESTMux
	inputs     map[string]any
	data       *plugins.Data
	logger     *slog.Logger
	token      config.Token
}

// WithGraphQL installs the given mux as the PluginContext's GraphQL
// client.
func WithGraphQL(mux *GraphQLMux) PluginContextOption {
	return func(c *pluginContextConfig) { c.graphQLMux = mux }
}

// WithREST installs the given mux as the PluginContext's REST
// client.
func WithREST(mux *RESTMux) PluginContextOption {
	return func(c *pluginContextConfig) { c.restMux = mux }
}

// WithInputs overrides the default inputs map.
func WithInputs(in map[string]any) PluginContextOption {
	return func(c *pluginContextConfig) { c.inputs = in }
}

// WithData overrides the default empty Data envelope.
func WithData(d *plugins.Data) PluginContextOption {
	return func(c *pluginContextConfig) { c.data = d }
}

// NewPluginContext bundles a *GraphQLMux + *RESTMux into a
// *plugins.PluginContext ready for `Plugin.Run`. Defaults:
//
//   - Inputs:   map[string]any{"user": "octocat"}
//   - Data:     plugins.NewData()
//   - Logger:   slog.New(slog.NewTextHandler(io.Discard, nil))
//   - Token:    config.NewToken("MOCKED_TOKEN")
//
// Either WithGraphQL or WithREST (or both) MUST be supplied; without
// them the returned PluginContext has nil clients and most plugins
// will Skipped-out at their early-return guard.
func NewPluginContext(t *testing.T, opts ...PluginContextOption) *plugins.PluginContext {
	t.Helper()
	cfg := pluginContextConfig{
		inputs: map[string]any{"user": "octocat"},
		data:   plugins.NewData(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		token:  config.NewToken("MOCKED_TOKEN"),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	pc := &plugins.PluginContext{
		Inputs: cfg.inputs,
		Logger: cfg.logger,
		Data:   cfg.data,
	}
	if cfg.graphQLMux != nil {
		gql, err := githubapi.NewGraphQL(cfg.token, "http://mock.localhost/graphql",
			httpx.Options{Transport: cfg.graphQLMux, DisableRetries: true})
		if err != nil {
			t.Fatalf("NewPluginContext: NewGraphQL: %v", err)
		}
		pc.GraphQL = gql
	}
	if cfg.restMux != nil {
		rest, err := githubapi.NewREST(cfg.token, "http://mock.localhost",
			httpx.Options{Transport: cfg.restMux, DisableRetries: true})
		if err != nil {
			t.Fatalf("NewPluginContext: NewREST: %v", err)
		}
		pc.REST = rest
	}
	return pc
}
