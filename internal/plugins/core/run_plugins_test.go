package core_test

import (
	"context"
	"errors"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/core"
)

type stubPlugin struct {
	name string
	run  func(ctx context.Context, pc *plugins.PluginContext) (any, error)
}

func (s *stubPlugin) Name() string                     { return s.name }
func (s *stubPlugin) Metadata() *config.PluginMetadata { return nil }
func (s *stubPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	return s.run(ctx, pc)
}

type fakeTB struct{ cleanupFn func() }

func (f *fakeTB) Helper()                           {}
func (f *fakeTB) Cleanup(fn func())                 { f.cleanupFn = fn }
func (f *fakeTB) Fatalf(format string, args ...any) {}

// registerStubs registers each stub and arranges cleanup so that
// subsequent tests start from the original registry (which still
// contains the core plugin from its package init).
func registerStubs(t *testing.T, stubs ...*stubPlugin) {
	t.Helper()
	tb := &fakeTB{}
	for _, s := range stubs {
		plugins.RegisterForTest(tb, s)
	}
	t.Cleanup(tb.cleanupFn)
}

func TestRunPlugins_AggregatesSuccessErrorPanic(t *testing.T) {
	successCalls := atomic.Int32{}
	registerStubs(
		t,
		&stubPlugin{
			name: "stub-success",
			run: func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
				successCalls.Add(1)
				return "ok-payload", nil
			},
		},
		&stubPlugin{
			name: "stub-error",
			run: func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
				return nil, errors.New("boom")
			},
		},
		&stubPlugin{
			name: "stub-panic",
			run: func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
				panic("kaboom")
			},
		},
	)

	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}}

	if err := core.RunPlugins(context.Background(), pc, 3); err != nil {
		t.Fatalf("RunPlugins: %v", err)
	}

	if successCalls.Load() != 1 {
		t.Errorf("expected stub-success to run once, got %d", successCalls.Load())
	}
	if v, ok := pc.Data.GetPlugin("stub-success"); !ok || v != "ok-payload" {
		t.Errorf("stub-success result = %v (ok=%v)", v, ok)
	}
	if v, ok := pc.Data.GetPlugin("stub-error"); !ok || v == nil {
		t.Errorf("stub-error not recorded")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("stub-error should be stored as error, got %T", v)
	}
	if v, ok := pc.Data.GetPlugin("stub-panic"); !ok {
		t.Errorf("stub-panic not recorded")
	} else {
		var retry *xerrors.RetryableError
		err, isErr := v.(error)
		if !isErr || !xerrors.As(err, &retry) {
			t.Errorf("stub-panic should wrap into *RetryableError, got %T (%v)", v, v)
		}
	}
}

func TestRunPlugins_ParallelOneSerializes(t *testing.T) {
	// With parallel=1 plugins must execute serially. We assert that by
	// ensuring no two plugins overlap.
	concurrent := atomic.Int32{}
	maxObserved := atomic.Int32{}
	noteRun := func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
		n := concurrent.Add(1)
		defer concurrent.Add(-1)
		for {
			cur := maxObserved.Load()
			if n <= cur {
				break
			}
			if maxObserved.CompareAndSwap(cur, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		return "ok", nil
	}
	registerStubs(
		t,
		&stubPlugin{name: "stub-a", run: noteRun},
		&stubPlugin{name: "stub-b", run: noteRun},
		&stubPlugin{name: "stub-c", run: noteRun},
	)

	pc := &plugins.PluginContext{Data: plugins.NewData()}
	if err := core.RunPlugins(context.Background(), pc, 1); err != nil {
		t.Fatalf("RunPlugins: %v", err)
	}
	if maxObserved.Load() != 1 {
		t.Fatalf("parallel=1 should serialize plugins; maxObserved = %d", maxObserved.Load())
	}
}

func TestRunPlugins_ParallelZeroUsesGOMAXPROCS(t *testing.T) {
	// parallel<=0 must fall back to GOMAXPROCS. The exact value is
	// runtime-dependent; assert that the call still completes
	// successfully and runs every registered stub.
	visited := atomic.Int32{}
	stubs := make([]*stubPlugin, 0, 4)
	for _, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		stubs = append(stubs, &stubPlugin{
			name: "fan-" + name,
			run: func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
				visited.Add(1)
				return nil, nil
			},
		})
	}
	registerStubs(t, stubs...)

	pc := &plugins.PluginContext{Data: plugins.NewData()}
	if err := core.RunPlugins(context.Background(), pc, 0); err != nil {
		t.Fatalf("RunPlugins: %v", err)
	}
	if visited.Load() != 4 {
		t.Fatalf("visited = %d, want 4", visited.Load())
	}
}

func TestRunPlugins_NilContextErrors(t *testing.T) {
	if err := core.RunPlugins(context.Background(), nil, 1); err == nil {
		t.Fatalf("expected error for nil PluginContext")
	}
}

func TestRunPlugins_CancelledContextPropagates(t *testing.T) {
	registerStubs(t, &stubPlugin{
		name: "slow",
		run: func(ctx context.Context, pc *plugins.PluginContext) (any, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return "late", nil
			}
		},
	})
	pc := &plugins.PluginContext{Data: plugins.NewData()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := core.RunPlugins(ctx, pc, 1); err == nil {
		// Per the contract a per-plugin failure (here ctx error) is
		// recorded in Data.Plugins, but the outer error is also
		// allowed when ctx is already cancelled before the plugin
		// returns. We accept either.
		if v, ok := pc.Data.GetPlugin("slow"); ok {
			if _, isErr := v.(error); !isErr {
				t.Fatalf("slow plugin recorded value should be error, got %T", v)
			}
		}
	}
}

// helper to keep imports tight
var _ = sort.Strings
