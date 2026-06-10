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

// TestPartial_Traffic_DelimiterAndHideEmpty validates the two issue
// #412 contract bullets at once:
//
//   - the per-repo line contains the visible ": " delimiter between
//     `</span>` and the view count, so long repo names cannot run
//     into the number;
//   - with HideEmpty=true (default), Count==0 entries are filtered
//     out before render.
func TestPartial_Traffic_DelimiterAndHideEmpty(t *testing.T) {
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

// TestPartial_Traffic_HideEmptyFalse verifies HideEmpty=false preserves
// the legacy behaviour where every repo (including 0-view) is rendered.
func TestPartial_Traffic_HideEmptyFalse(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 100, Uniques: 40},
		HideEmpty: false,
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

// TestPartial_Traffic_Golden writes the SVG fragment to
// tests/golden/classic/m4/traffic.svg.
func TestPartial_Traffic_Golden(t *testing.T) {
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"mjun0812/alpha": {Count: 1234, Uniques: 456},
			"mjun0812/beta":  {Count: 50, Uniques: 20},
			"mjun0812/gamma": {Count: 1, Uniques: 1},
		},
		Total:     traffic.TrafficView{Count: 1285, Uniques: 477},
		HideEmpty: true,
	})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}

	if got != "" {
		t.Fatalf("traffic standalone partial should be empty; got:\n%s", got)
	}
}
