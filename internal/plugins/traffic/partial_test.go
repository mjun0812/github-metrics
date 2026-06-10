package traffic_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// TestPartial_Traffic_Skipped verifies a Skipped result yields the
// empty fragment so the section is not rendered at all.
func TestPartial_Traffic_Skipped(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{Skipped: true})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty fragment for Skipped=true; got %q", got)
	}
}

// TestPartial_Traffic_StandaloneEmpty pins the upstream contract:
// traffic data is merged into base.repositories, so the standalone
// partial intentionally returns an empty fragment.
func TestPartial_Traffic_StandaloneEmpty(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/beta":  {Count: 50, Uniques: 20},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 150, Uniques: 60},
		HideEmpty: true,
	})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if got != "" {
		t.Errorf("traffic standalone partial should be empty; got:\n%s", got)
	}
}
