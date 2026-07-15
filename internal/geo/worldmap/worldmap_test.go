package worldmap

import (
	"strings"
	"testing"
)

func TestRender_EmptyPointsStillRendersBaseMap(t *testing.T) {
	t.Parallel()
	svg, h, err := Render(nil, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if h <= 0 {
		t.Errorf("Render height = %d, want > 0", h)
	}
	if !strings.Contains(svg, "worldmap-countries") {
		t.Errorf("base map group missing from output")
	}
	if strings.Contains(svg, "worldmap-markers") {
		t.Errorf("marker group should not be emitted when no points supplied")
	}
}

func TestRender_MultiplePointsAllEmitted(t *testing.T) {
	t.Parallel()
	pts := []Point{
		{Lat: 35.68, Lng: 139.75, Count: 3, Label: "Tokyo"},
		{Lat: 51.5, Lng: -0.13, Count: 1, Label: "London"},
		{Lat: -33.86, Lng: 151.21, Count: 2, Label: "Sydney"},
	}
	svg, _, err := Render(pts, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	circleCount := strings.Count(svg, "<circle")
	if circleCount != len(pts) {
		t.Errorf("circle count = %d, want %d", circleCount, len(pts))
	}
	for _, p := range pts {
		if !strings.Contains(svg, "<title>"+p.Label+"</title>") {
			t.Errorf("label %q missing from output", p.Label)
		}
	}
}

func TestRender_LargestCountFirst(t *testing.T) {
	t.Parallel()
	svg, _, err := Render([]Point{
		{Lat: 0, Lng: 0, Count: 1, Label: "small"},
		{Lat: 10, Lng: 10, Count: 100, Label: "big"},
	}, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	iBig := strings.Index(svg, "big")
	iSmall := strings.Index(svg, "small")
	if iBig < 0 || iSmall < 0 {
		t.Fatalf("both markers should be present")
	}
	if iBig > iSmall {
		t.Errorf("big-count marker should be emitted before small-count marker (so smaller lands on top)")
	}
}

func TestRender_ExtremesClamped(t *testing.T) {
	t.Parallel()
	// Antimeridian (180) and near-pole (89) must project without NaN
	// and land inside the viewport.
	svg, _, err := Render([]Point{
		{Lat: 89, Lng: 180, Count: 1},
		{Lat: -89, Lng: -180, Count: 1},
	}, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(svg, "NaN") {
		t.Errorf("output contains NaN: %s", svg)
	}
	if strings.Count(svg, "<circle") != 2 {
		t.Errorf("both boundary points should be emitted")
	}
}

func TestRender_HeightMatchesEquirectangularAspect(t *testing.T) {
	t.Parallel()
	_, h, err := Render(nil, Options{Width: 480})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// 480 * 240 / 480 = 240
	if h != 240 {
		t.Errorf("height for width=480 = %d, want 240", h)
	}
}

func TestRender_CustomColors(t *testing.T) {
	t.Parallel()
	svg, _, err := Render([]Point{{Lat: 0, Lng: 0, Count: 1}}, Options{
		LandFill:     "#ff0000",
		LandStroke:   "#00ff00",
		MarkerFill:   "#0000ff",
		MarkerStroke: "#ffff00",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"#ff0000", "#00ff00", "#0000ff", "#ffff00"} {
		if !strings.Contains(svg, want) {
			t.Errorf("custom color %s missing from output", want)
		}
	}
}
