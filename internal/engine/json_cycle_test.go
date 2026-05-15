package engine_test

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// node is a tiny struct used to build pathological self-referencing
// payloads. Marshal must replace the second visit with "[Circular]".
type node struct {
	Name string
	Self *node
	Peer *node
}

func TestMarshal_SelfReference(t *testing.T) {
	t.Parallel()

	n := &node{Name: "loop"}
	n.Self = n

	data := plugins.NewData()
	data.SetPlugin("self-ref", n)

	body, err := engine.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal panicked or errored on a self-referencing payload: %v", err)
	}
	if !strings.Contains(string(body), `"[Circular]"`) {
		t.Fatalf("expected [Circular] sentinel for self-ref; got: %s", body)
	}
	if !strings.Contains(string(body), `"loop"`) {
		t.Fatalf("expected the non-cyclic field to survive: %s", body)
	}
}

func TestMarshal_MutualReference(t *testing.T) {
	t.Parallel()

	a := &node{Name: "a"}
	b := &node{Name: "b"}
	a.Peer = b
	b.Peer = a

	data := plugins.NewData()
	data.SetPlugin("a-b", a)

	body, err := engine.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"[Circular]"`) {
		t.Fatalf("expected [Circular] sentinel for mutual ref; got: %s", body)
	}
}

func TestMarshal_CycleInSlice(t *testing.T) {
	t.Parallel()

	type bag struct {
		Items []*bag
	}
	b := &bag{}
	b.Items = []*bag{b}

	data := plugins.NewData()
	data.SetPlugin("bag", b)

	body, err := engine.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"[Circular]"`) {
		t.Fatalf("expected [Circular] sentinel for slice cycle: %s", body)
	}
}
