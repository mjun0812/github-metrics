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

func (f *fakePlugin) Name() string { return f.name }
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

func TestRegister_InvalidPanics(t *testing.T) {
	for name, register := range map[string]func(){
		"duplicate name": func() {
			plugins.Register(&fakePlugin{name: "alpha"})
			plugins.Register(&fakePlugin{name: "alpha"})
		},
		"empty name": func() { plugins.Register(&fakePlugin{name: ""}) },
	} {
		t.Run(name, func(t *testing.T) {
			resetRegistry(t)
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic")
				}
			}()
			register()
		})
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
