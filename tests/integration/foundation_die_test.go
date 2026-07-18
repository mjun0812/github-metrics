package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// errorStubPlugin is a plugin that always returns the configured
// error. We register it before each die-mode test so the parallel
// runner has something concrete to fail on.
type errorStubPlugin struct {
	name string
	err  error
}

func (p *errorStubPlugin) Name() string                { return p.name }
func (p *errorStubPlugin) Requires() []plugins.DataKey { return []plugins.DataKey{} }
func (p *errorStubPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	return nil, p.err
}

// fakeTB satisfies plugins.TB so the integration test can use
// plugins.RegisterForTest without taking a dependency on the testing
// package internals.
type fakeTB struct{ cleanup func() }

func (f *fakeTB) Helper()                           {}
func (f *fakeTB) Cleanup(fn func())                 { f.cleanup = fn }
func (f *fakeTB) Fatalf(format string, args ...any) {}

func registerErrorPlugin(t *testing.T, name string, err error) {
	t.Helper()
	tb := &fakeTB{}
	plugins.RegisterForTest(tb, &errorStubPlugin{name: name, err: err})
	t.Cleanup(tb.cleanup)
}

// TestEngine_DieTrueShortCircuits is US5 AS5: when Die == true and a
// plugin returns an error, Compute MUST surface that error and stop.
func TestEngine_DieTrueShortCircuits(t *testing.T) {
	want := errors.New("simulated upstream outage")
	registerErrorPlugin(t, "stub-die", want)

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
		Die:      true,
	}, deps)
	if err == nil {
		t.Fatalf("Die=true should have surfaced the plugin error")
	}
	if !errors.Is(err, want) {
		t.Errorf("expected wrapped error to chain to original, got %v", err)
	}
}

// TestEngine_BaseProviderFailureSurfaces verifies the #781 error-
// visibility fix end-to-end: when a split contributionsCollection query
// fails, the base plugin's Provider error is threaded onto Data (not
// swallowed on Result.Error) so it appears in Result.Errors and, under
// plugins_errors_fatal, aborts the run.
func TestEngine_BaseProviderFailureSurfaces(t *testing.T) {
	baseInputs := map[string]any{
		"user":                "octocat",
		"chrome_activity":     "yes",
		"chrome_repositories": "yes",
	}
	failingAggregate := map[string]string{
		"User":                    userOctocat,
		"UserRepositories":        userRepositories250,
		"UserContributionCommits": `{"errors":[{"message":"RESOURCE_LIMITS_EXCEEDED"}]}`,
	}

	t.Run("collected", func(t *testing.T) {
		deps, _ := newEngineDeps(t, failingAggregate)
		res, err := engine.Compute(context.Background(), engine.Request{
			Login:    "octocat",
			Template: "noop",
			Inputs:   baseInputs,
			Die:      false,
		}, deps)
		if err != nil {
			t.Fatalf("Die=false should not surface plugin errors: %v", err)
		}
		if len(res.Errors) == 0 {
			t.Fatalf("Result.Errors should carry the base Provider failure")
		}
	})

	t.Run("fatal", func(t *testing.T) {
		deps, _ := newEngineDeps(t, failingAggregate)
		_, err := engine.Compute(context.Background(), engine.Request{
			Login:    "octocat",
			Template: "noop",
			Inputs:   baseInputs,
			Die:      true,
		}, deps)
		if err == nil {
			t.Fatalf("plugins_errors_fatal should abort the run on a base Provider failure")
		}
	})
}

// TestEngine_DieFalseCollectsErrors mirrors the opposite contract:
// Die == false (the default) leaves the request successful and stashes
// the error in Result.Errors for the caller to inspect.
func TestEngine_DieFalseCollectsErrors(t *testing.T) {
	want := errors.New("transient blip")
	registerErrorPlugin(t, "stub-collect", want)

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
		Die:      false,
	}, deps)
	if err != nil {
		t.Fatalf("Die=false should not surface plugin errors as the outer return: %v", err)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("Result.Errors should contain the stub error")
	}
	if !errors.Is(res.Errors[0], want) {
		t.Errorf("Result.Errors[0] does not wrap the original error: %v", res.Errors[0])
	}
}
