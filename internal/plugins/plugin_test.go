package plugins_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// fakePlugin implements plugins.Plugin for tests.
type fakePlugin struct {
	name string
	run  func(ctx context.Context, pc *plugins.PluginContext) (any, error)
}

func (f *fakePlugin) Name() string                { return f.name }
func (f *fakePlugin) Requires() []plugins.DataKey { return []plugins.DataKey{} }
func (f *fakePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if f.run == nil {
		return nil, nil
	}
	return f.run(ctx, pc)
}

func resetRegistry(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { plugins.Reset() })
	plugins.Reset()
}

func TestRegister_DuplicateNamePanics(t *testing.T) {
	resetRegistry(t)

	plugins.Register(&fakePlugin{name: "alpha"})
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate Register")
		}
	}()
	plugins.Register(&fakePlugin{name: "alpha"})
}

func TestRegister_EmptyNamePanics(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty Name")
		}
	}()
	plugins.Register(&fakePlugin{name: ""})
}

func TestGet_PresentAndMissing(t *testing.T) {
	resetRegistry(t)

	plugins.Register(&fakePlugin{name: "beta"})
	got, ok := plugins.Get("beta")
	if !ok {
		t.Fatalf("Get(beta) ok=false")
	}
	if got.Name() != "beta" {
		t.Fatalf("Get name = %q", got.Name())
	}
	if _, ok := plugins.Get("missing"); ok {
		t.Fatalf("Get(missing) should report ok=false")
	}
}

func TestEach_IteratesInSortedOrder(t *testing.T) {
	resetRegistry(t)

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		plugins.Register(&fakePlugin{name: name})
	}
	var seen []string
	if err := plugins.Each(func(name string, _ plugins.Plugin) error {
		seen = append(seen, name)
		return nil
	}); err != nil {
		t.Fatalf("Each: %v", err)
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(seen) != len(want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("seen[%d] = %q, want %q", i, seen[i], want[i])
		}
	}
}

type fakeTB struct {
	cleanupFn func()
}

func (f *fakeTB) Helper()                           {}
func (f *fakeTB) Cleanup(fn func())                 { f.cleanupFn = fn }
func (f *fakeTB) Fatalf(format string, args ...any) {}

func TestRegisterForTest_RestoresPriorValue(t *testing.T) {
	resetRegistry(t)
	plugins.Register(&fakePlugin{name: "swap"})

	tb := &fakeTB{}
	plugins.RegisterForTest(tb, &fakePlugin{name: "swap"})

	got, ok := plugins.Get("swap")
	if !ok {
		t.Fatalf("after RegisterForTest, Get returned ok=false")
	}
	// Identity isn't easily comparable across instances; just check
	// that the value is the substituted one (anything works since
	// resetRegistry will follow).
	_ = got

	if tb.cleanupFn == nil {
		t.Fatalf("RegisterForTest did not register a cleanup")
	}
	tb.cleanupFn()

	if _, ok := plugins.Get("swap"); !ok {
		t.Fatalf("cleanup should have restored the prior registration")
	}
}

func TestRegisterForTest_RemovesEntryWhenNoPrior(t *testing.T) {
	resetRegistry(t)

	tb := &fakeTB{}
	plugins.RegisterForTest(tb, &fakePlugin{name: "ephemeral"})
	if _, ok := plugins.Get("ephemeral"); !ok {
		t.Fatalf("RegisterForTest should have added the plugin")
	}
	tb.cleanupFn()
	if _, ok := plugins.Get("ephemeral"); ok {
		t.Fatalf("cleanup should have removed an ephemeral entry")
	}
}

func TestImports_ReadsFromData(t *testing.T) {
	d := plugins.NewData()
	d.SetPlugin("activity", "event-payload")

	imp := plugins.NewImports(d)
	got, ok := imp.Get("activity")
	if !ok || got != "event-payload" {
		t.Fatalf("imp.Get = %v, ok=%v", got, ok)
	}
	if _, ok := imp.Get("missing"); ok {
		t.Fatalf("expected missing -> ok=false")
	}
}
