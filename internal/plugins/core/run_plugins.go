package core

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"

	"golang.org/x/sync/errgroup"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// RunPlugins executes every plugin registered in [plugins] that is not
// "core" itself, in parallel up to the supplied concurrency cap, and
// records each result (or recovered error) under pc.Data.Plugins.
// parallel <= 0 falls back to runtime.GOMAXPROCS(0) so callers can pass
// the user-supplied value verbatim.
//
// The function never returns a non-nil error for an individual plugin
// failure: the failure is recorded inside pc.Data.Plugins so the
// engine's aggregation logic can decide whether to abort the request.
// A non-nil error from RunPlugins itself indicates infrastructure
// trouble (context cancellation, errgroup internals).
//
// Non-fatal errors that a plugin records via Data.AppendError (e.g. the
// M4 indepth-GraphQL degraded path or the repositories paging
// batch-halving failure) remain on Data.Errors and are merged into
// Result.Errors by engine.Compute's collectPluginErrors. RunPlugins
// itself does not touch Data.Errors so the mutex-protected accumulator
// stays in one place.
func RunPlugins(ctx context.Context, pc *plugins.PluginContext, parallel int) error {
	if pc == nil {
		return fmt.Errorf("core.RunPlugins: nil PluginContext")
	}
	if parallel <= 0 {
		parallel = runtime.GOMAXPROCS(0)
	}

	names := pluginNamesExcludingCore()

	// Design decision: RunPlugins does NOT prefetch based on Plugin.Requires().
	//
	// Rationale: the dataprovider.Provider uses golang.org/x/sync/singleflight
	// to collapse concurrent in-flight calls and caches both success and error
	// outcomes permanently, so the first plugin to call Provider.Repositories()
	// pays the network cost; all subsequent callers return the cached result
	// without hitting the network again. A prefetch goroutine would buy at most
	// a few milliseconds of overlap against the plugin setup overhead, at the
	// cost of extra goroutines, a more complex error-routing path, and context
	// cancellation races. The declared Requires() is purely documentary and
	// exists for drift-detection tests only. Do NOT add prefetch here in the
	// future without re-evaluating this trade-off.

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(parallel)

	for _, name := range names {
		name := name
		p, ok := plugins.Get(name)
		if !ok {
			continue
		}
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					recovered := xerrors.NewRetryableError(fmt.Errorf("plugin %q panicked: %v", name, r))
					pc.Data.SetPlugin(name, recovered)
					// Do not propagate panic-derived error to errgroup;
					// per-plugin failures stay isolated.
					err = nil
				}
			}()
			result, runErr := p.Run(gctx, pc)
			if runErr != nil {
				pc.Data.SetPlugin(name, runErr)
				return nil
			}
			pc.Data.SetPlugin(name, result)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// errgroup propagates non-nil errors from goroutines or
		// cancellation from gctx. In our usage individual plugin
		// errors are not propagated, so the only remaining cause is
		// ctx cancellation.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return err
	}
	return nil
}

// pluginNamesExcludingCore returns the alphabetical list of registered
// plugin names with the "core" entry removed. core wires the parallel
// runner itself, so the engine drives it directly outside the errgroup.
func pluginNamesExcludingCore() []string {
	var names []string
	_ = plugins.Each(func(name string, _ plugins.Plugin) error {
		if name == Name {
			return nil
		}
		names = append(names, name)
		return nil
	})
	sort.Strings(names)
	return names
}
