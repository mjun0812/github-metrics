package repositories_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/repositories"
	"github.com/mjun0812/github-metrics/internal/templates"
)

func newPC(t *testing.T, r *repositories.Result) *templates.PartialContext {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin(repositories.Name, r)
	return &templates.PartialContext{Data: data}
}

func render(t *testing.T, r *repositories.Result) string {
	t.Helper()
	got, _, err := repositories.Partial(context.Background(), newPC(t, r))
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return got
}

// TestPartial_PinnedAppendedAfterFeatured pins #555: when `r.Pinned`
// holds repos distinct from `r.Featured`, they are appended after the
// featured cards inside the same `largeable-flex-wrap` section.
func TestPartial_PinnedAppendedAfterFeatured(t *testing.T) {
	t.Parallel()
	got := render(t, &repositories.Result{
		Featured: []plugins.Repository{
			{NameWithOwner: "octocat/featured-a", URL: "https://github.com/octocat/featured-a", Stars: 50},
		},
		Pinned: []plugins.Repository{
			{NameWithOwner: "octocat/pinned-x", URL: "https://github.com/octocat/pinned-x", Stars: 20},
			{NameWithOwner: "octocat/pinned-y", URL: "https://github.com/octocat/pinned-y", Stars: 10},
		},
	})
	if n := strings.Count(got, `<g class="repository"`); n != 3 {
		t.Fatalf("want 3 repository cards (1 featured + 2 pinned), got %d:\n%s", n, got)
	}
	// Order: featured first, pinned after.
	fi := strings.Index(got, "octocat/featured-a")
	pi := strings.Index(got, "octocat/pinned-x")
	if fi < 0 || pi < 0 || fi >= pi {
		t.Errorf("expected featured-a to appear before pinned-x; featured idx=%d pinned idx=%d:\n%s", fi, pi, got)
	}
}

// TestPartial_PinnedPartialOverlap covers the realistic case where some
// pinned repos overlap with featured (high stars) and others do not.
// The dedup keeps the featured ordering and appends only the distinct
// pinned items.
func TestPartial_PinnedPartialOverlap(t *testing.T) {
	t.Parallel()
	got := render(t, &repositories.Result{
		Featured: []plugins.Repository{
			{NameWithOwner: "octocat/popular", URL: "https://github.com/octocat/popular", Stars: 100},
			{NameWithOwner: "octocat/second", URL: "https://github.com/octocat/second", Stars: 50},
		},
		Pinned: []plugins.Repository{
			{NameWithOwner: "octocat/popular", URL: "https://github.com/octocat/popular", Stars: 100}, // dup
			{NameWithOwner: "octocat/pet-project", URL: "https://github.com/octocat/pet-project", Stars: 3},
		},
	})
	if n := strings.Count(got, `<g class="repository"`); n != 3 {
		t.Fatalf("want 3 cards (popular + second + pet-project), got %d:\n%s", n, got)
	}
	if strings.Count(got, "octocat/popular") != 2 {
		t.Errorf("popular should appear exactly twice in output (URL + label):\n%s", got)
	}
	if !strings.Contains(got, "octocat/pet-project") {
		t.Errorf("missing distinct pinned repo pet-project:\n%s", got)
	}
}
