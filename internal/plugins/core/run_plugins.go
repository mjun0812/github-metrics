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
//
// LAZY-ONLY POLICY (issue #604 decision #5): RunPlugins intentionally
// does NOT prefetch Provider data based on Plugin.Requires(). The
// Provider already collapses concurrent callers via singleflight and
// caches both successes and errors for the lifetime of the request, so
// the first plugin that asks for a given key triggers the fetch and
// every later caller observes the cached outcome. A prefetch goroutine
// would add error-routing complexity for marginal benefit. Requires()
// is purely declarative — its only consumers are per-plugin
// counting-mock tests that catch drift between declared and actual
// reads. Do not "optimize" this back into a prefetch loop.
func RunPlugins(ctx context.Context, pc *plugins.PluginContext, parallel int) error {
	if pc == nil {
		return fmt.Errorf("core.RunPlugins: nil PluginContext")
	}
	if parallel <= 0 {
		parallel = runtime.GOMAXPROCS(0)
	}

	names := pluginNamesExcludingCore()

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
// plugin names with the "core" entry removed. The core plugin wires
// the parallel runner itself, so the engine drives it directly outside
// the errgroup.
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
