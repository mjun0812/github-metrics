package geo

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestGeocoder_KnownCities(t *testing.T) {
	t.Parallel()
	g := New()
	cases := []struct {
		in       string
		lat, lng float64
	}{
		{"Tokyo", 35.68, 139.75},
		{"London", 51.5, -0.13},
		{"Paris", 48.85, 2.35},
		{"New York", 40.71, -74.0},
		{"NYC", 40.71, -74.0},
		{"San Francisco", 37.77, -122.42},
		{"São Paulo", -23.55, -46.63},
	}
	for _, c := range cases {
		loc, ok := g.Lookup(c.in)
		if !ok {
			t.Errorf("Lookup(%q) not found", c.in)
			continue
		}
		if !near(loc.Lat, c.lat, 0.5) || !near(loc.Lng, c.lng, 0.5) {
			t.Errorf("Lookup(%q) = (%v,%v), want ~(%v,%v)", c.in, loc.Lat, loc.Lng, c.lat, c.lng)
		}
	}
}

func TestGeocoder_HighestPopulationWins(t *testing.T) {
	t.Parallel()
	g := New()
	// "Paris" (France) vs "Paris" (Texas). The France entry has orders
	// of magnitude more population, so the tie-break must prefer it.
	loc, ok := g.Lookup("Paris")
	if !ok {
		t.Fatalf("Lookup(Paris) not found")
	}
	if loc.Lat < 40 || loc.Lng > 10 {
		t.Errorf("Lookup(Paris) resolved to (%v,%v); expected the French Paris (~48.85, ~2.35)", loc.Lat, loc.Lng)
	}
}

func TestGeocoder_CityCommaCountryPrefersCity(t *testing.T) {
	t.Parallel()
	g := New()
	loc, ok := g.Lookup("Kyoto, Japan")
	if !ok {
		t.Fatalf("Lookup(Kyoto, Japan) not found")
	}
	// Kyoto centroid, not Japan's country centroid (~36N, 138E).
	if !near(loc.Lat, 35.0, 1.0) || !near(loc.Lng, 135.7, 1.0) {
		t.Errorf("Lookup(Kyoto, Japan) = (%v,%v); expected Kyoto (~35, ~135.7)", loc.Lat, loc.Lng)
	}
}

func TestGeocoder_CountryFallback(t *testing.T) {
	t.Parallel()
	g := New()
	// "Nowhereville, Japan" has no city match; "Japan" resolves.
	loc, ok := g.Lookup("Somewhere obscure, Japan")
	if !ok {
		t.Fatalf("Country fallback for Japan did not resolve")
	}
	if loc.Lat < 20 || loc.Lat > 50 {
		t.Errorf("Lookup(...,Japan) lat = %v; expected roughly within Japan", loc.Lat)
	}
	if _, ok := g.Lookup("USA"); !ok {
		t.Errorf("USA alias should resolve")
	}
	if _, ok := g.Lookup("UK"); !ok {
		t.Errorf("UK alias should resolve")
	}
}

func TestGeocoder_UnicodeAndEmoji(t *testing.T) {
	t.Parallel()
	g := New()
	// Emoji + city name. The flag emoji must not block the city match.
	if _, ok := g.Lookup("🇯🇵 Tokyo"); !ok {
		t.Errorf("emoji-prefixed city must still resolve")
	}
	// ASCII fold of a diacritic city name.
	loc1, ok1 := g.Lookup("São Paulo")
	loc2, ok2 := g.Lookup("Sao Paulo")
	if !ok1 || !ok2 {
		t.Fatalf("both diacritic and ASCII spellings must resolve")
	}
	if loc1 != loc2 {
		t.Errorf("diacritic and ASCII spellings should resolve to same entry: %+v vs %+v", loc1, loc2)
	}
}

func TestGeocoder_EmptyAndNoMatch(t *testing.T) {
	t.Parallel()
	g := New()
	if _, ok := g.Lookup(""); ok {
		t.Errorf("empty input should not resolve")
	}
	if _, ok := g.Lookup("   "); ok {
		t.Errorf("whitespace-only input should not resolve")
	}
	if _, ok := g.Lookup("qqqqzzzz-not-a-place"); ok {
		t.Errorf("nonsense input should not resolve")
	}
}

func TestGeocoder_DefaultSingleton(t *testing.T) {
	t.Parallel()
	g1 := Default()
	g2 := Default()
	if g1 != g2 {
		t.Fatalf("Default() must return a singleton")
	}
}

func TestNormalizeKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Tokyo":     "tokyo",
		"  Tokyo  ": "tokyo",
		"São Paulo": "são paulo",
		"New-York":  "new york",
		"🇯🇵 Tokyo":  "tokyo",
		"":          "",
		"...":       "",
		"U.S.A.":    "u s a",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Errorf("normalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}
