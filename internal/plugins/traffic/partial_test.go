package traffic_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// renderTraffic is a test helper that builds a PartialContext holding the
// given Result and returns the rendered fragment.
func renderTraffic(t *testing.T, r *traffic.Result) string {
	t.Helper()
	data := plugins.NewData()
	if r != nil {
		data.SetPlugin(traffic.Name, r)
	}
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return got
}

// TestPartial_Traffic_Skipped verifies a Skipped result yields the
// empty fragment so the section is not rendered at all.
func TestPartial_Traffic_Skipped(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{Skipped: true})
	if got != "" {
		t.Errorf("expected empty fragment for Skipped=true; got %q", got)
	}
}

// TestPartial_Traffic_MissingResult verifies that an absent plugin
// result renders nothing.
func TestPartial_Traffic_MissingResult(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, nil)
	if got != "" {
		t.Errorf("expected empty fragment for missing result; got %q", got)
	}
}

// TestPartial_Traffic_Normal checks the aggregate line and per-repo rows
// render with the shared k/m short form.
func TestPartial_Traffic_Normal(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 1200, Uniques: 40},
			"octocat/beta":  {Count: 50, Uniques: 1},
		},
		Total:     traffic.TrafficView{Count: 1250, Uniques: 41},
		HideEmpty: true,
	})
	// Aggregate line uses k short form and plural "views".
	if !strings.Contains(got, `<span class="label">1.3k views (41 unique)</span>`) {
		t.Errorf("missing/incorrect aggregate line; got:\n%s", got)
	}
	// Per-repo rows.
	if !strings.Contains(got, `<span class="repo">octocat/alpha</span>: 1.2k views (40 unique)`) {
		t.Errorf("missing alpha row; got:\n%s", got)
	}
	// Singular "view" and "unique" when the count is 1.
	if !strings.Contains(got, `<span class="repo">octocat/beta</span>: 50 views (1 unique)`) {
		t.Errorf("missing beta row; got:\n%s", got)
	}
	if !strings.Contains(got, `data-section="traffic"`) || !strings.Contains(got, `Traffic</h2>`) {
		t.Errorf("missing section wrapper/header; got:\n%s", got)
	}
}

// TestPartial_Traffic_SingularView pins "1 view" (singular) on the
// aggregate line.
func TestPartial_Traffic_SingularView(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 1, Uniques: 1},
		},
		Total:     traffic.TrafficView{Count: 1, Uniques: 1},
		HideEmpty: true,
	})
	if !strings.Contains(got, `<span class="label">1 view (1 unique)</span>`) {
		t.Errorf("expected singular 'view'; got:\n%s", got)
	}
}

// TestPartial_Traffic_Ordering verifies descending order by count with a
// name-ascending tiebreak.
func TestPartial_Traffic_Ordering(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/low":   {Count: 10, Uniques: 3},
			"octocat/high":  {Count: 100, Uniques: 20},
			"octocat/mid":   {Count: 50, Uniques: 8},
			"octocat/tie-b": {Count: 50, Uniques: 5},
		},
		Total:     traffic.TrafficView{Count: 210, Uniques: 36},
		HideEmpty: true,
	})
	order := []string{"octocat/high", "octocat/mid", "octocat/tie-b", "octocat/low"}
	prev := -1
	for _, name := range order {
		idx := strings.Index(got, `<span class="repo">`+name+`</span>`)
		if idx == -1 {
			t.Fatalf("row %q not found; got:\n%s", name, got)
		}
		if idx < prev {
			t.Errorf("row %q out of order (idx %d < prev %d); got:\n%s", name, idx, prev, got)
		}
		prev = idx
	}
}

// TestPartial_Traffic_HideEmptyTrue drops Count==0 rows.
func TestPartial_Traffic_HideEmptyTrue(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 100, Uniques: 40},
		HideEmpty: true,
	})
	if strings.Contains(got, "octocat/zero") {
		t.Errorf("HideEmpty=true should drop zero-view repos; got:\n%s", got)
	}
	if !strings.Contains(got, "octocat/alpha") {
		t.Errorf("expected non-zero repo to remain; got:\n%s", got)
	}
}

// TestPartial_Traffic_HideEmptyFalse keeps Count==0 rows.
func TestPartial_Traffic_HideEmptyFalse(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 100, Uniques: 40},
		HideEmpty: false,
	})
	if !strings.Contains(got, `<span class="repo">octocat/zero</span>: 0 views (0 unique)`) {
		t.Errorf("HideEmpty=false should keep zero-view repos; got:\n%s", got)
	}
}

// TestPartial_Traffic_EmptyViews renders the aggregate-only section when
// there are no per-repo rows after filtering.
func TestPartial_Traffic_EmptyViews(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views:     map[string]traffic.TrafficView{},
		Total:     traffic.TrafficView{},
		HideEmpty: true,
	})
	if !strings.Contains(got, `data-section="traffic"`) {
		t.Errorf("expected section wrapper for empty views; got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="label">0 views (0 unique)</span>`) {
		t.Errorf("expected aggregate-only 0 views line; got:\n%s", got)
	}
	if strings.Contains(got, `class="repo"`) {
		t.Errorf("expected no per-repo rows; got:\n%s", got)
	}
}

// TestPartial_Traffic_XMLEscape ensures repo names are XML-escaped.
func TestPartial_Traffic_XMLEscape(t *testing.T) {
	t.Parallel()
	got := renderTraffic(t, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"o&o/a<b": {Count: 5, Uniques: 2},
		},
		Total:     traffic.TrafficView{Count: 5, Uniques: 2},
		HideEmpty: true,
	})
	if strings.Contains(got, "o&o/a<b") {
		t.Errorf("repo name not escaped; got:\n%s", got)
	}
	if !strings.Contains(got, "o&amp;o/a&lt;b") {
		t.Errorf("expected escaped repo name; got:\n%s", got)
	}
}
