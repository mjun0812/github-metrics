package geo

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// Location is the resolved lat/lng of a place name.
type Location struct {
	// Lat is the latitude in degrees, positive north.
	Lat float64
	// Lng is the longitude in degrees, positive east.
	Lng float64
	// CountryCode is the ISO 3166-1 alpha-2 country code the source row
	// was tagged with (empty for country-centroid fallbacks that
	// resolved by a synonym only).
	CountryCode string
}

//go:embed cities15000.tsv
var citiesTSV string

// Geocoder resolves free-form location strings to (lat, lng) pairs from
// the embedded GeoNames cities15000 dataset with a country-centroid
// fallback. Instances are safe for concurrent use after construction.
type Geocoder struct {
	// cities maps normalized city name → highest-population entry.
	cities map[string]Location
	// countries maps normalized country name → country centroid.
	countries map[string]Location
}

var (
	defaultOnce sync.Once
	defaultGeo  *Geocoder
)

// Default returns the process-wide geocoder, lazily loading the embedded
// datasets on first use.
func Default() *Geocoder {
	defaultOnce.Do(func() {
		defaultGeo = New()
	})
	return defaultGeo
}

// New builds a Geocoder from the embedded datasets. The returned value
// is ready for concurrent Lookup calls.
func New() *Geocoder {
	g := &Geocoder{
		cities:    make(map[string]Location, 40000),
		countries: make(map[string]Location, 512),
	}
	g.loadCities()
	g.loadCountryCentroids()
	g.loadCityAliases()
	return g
}

// cityAliases lists common short-hand or nickname → canonical city
// name mappings so bare "NYC" or "LA" strings resolve. The right-hand
// value must exist in the embedded cities table (it is re-normalized
// through the same lookup path).
var cityAliases = map[string]string{
	"NYC":            "New York City",
	"New York":       "New York City",
	"NY":             "New York City",
	"LA":             "Los Angeles",
	"SF":             "San Francisco",
	"Bay Area":       "San Francisco",
	"D.C.":           "Washington",
	"DC":             "Washington",
	"Washington DC":  "Washington",
	"Washington, DC": "Washington",
	"HK":             "Hong Kong",
	"Bengaluru":      "Bangalore",
	"Bombay":         "Mumbai",
	"Peking":         "Beijing",
}

// loadCityAliases folds each alias into the city index using the row
// currently registered under the canonical target key.
func (g *Geocoder) loadCityAliases() {
	for alias, target := range cityAliases {
		key := normalizeKey(alias)
		if key == "" {
			continue
		}
		if _, ok := g.cities[key]; ok {
			continue
		}
		if loc, ok := g.cities[normalizeKey(target)]; ok {
			g.cities[key] = loc
		}
	}
}

// loadCities parses the embedded TSV and keeps, per normalized key, the
// row with the largest population. Every city row contributes both its
// primary (UTF-8) name and its ASCII fold, so unicode inputs match the
// same entry as their ASCII spelling.
func (g *Geocoder) loadCities() {
	// Track the winning population per key so subsequent lower-population
	// rows do not clobber a match.
	pop := make(map[string]int64, len(g.cities))
	for _, line := range strings.Split(citiesTSV, "\n") {
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 6 {
			continue
		}
		name := cols[0]
		ascii := cols[1]
		lat, err := strconv.ParseFloat(cols[2], 64)
		if err != nil {
			continue
		}
		lng, err := strconv.ParseFloat(cols[3], 64)
		if err != nil {
			continue
		}
		cc := cols[4]
		popCur, _ := strconv.ParseInt(cols[5], 10, 64)
		loc := Location{Lat: lat, Lng: lng, CountryCode: cc}
		for _, raw := range []string{name, ascii} {
			key := normalizeKey(raw)
			if key == "" {
				continue
			}
			if prev, ok := pop[key]; ok && prev >= popCur {
				continue
			}
			pop[key] = popCur
			g.cities[key] = loc
		}
	}
}

// Lookup resolves a free-form location string to a Location. It matches
// the whole input first, then progressively strips leading tokens so
// entries like "Building 3, Tokyo, Japan" fall through to "Tokyo, Japan"
// and finally "Japan" (country centroid). Returns ok=false when no
// fragment matches.
func (g *Geocoder) Lookup(location string) (Location, bool) {
	loc := strings.TrimSpace(location)
	if loc == "" {
		return Location{}, false
	}
	// Try whole-string match first.
	if l, ok := g.lookupFragment(loc); ok {
		return l, true
	}
	// Then try each comma-separated fragment (rightmost first, since
	// "City, Country" strings put the more specific token on the left
	// and the coarser fallback on the right).
	parts := splitLocationParts(loc)
	for i := 0; i < len(parts); i++ {
		if l, ok := g.lookupFragment(parts[i]); ok {
			return l, true
		}
	}
	// Rightmost-to-leftmost as a country-fallback pass: reversed order
	// so a bare "Country" wins even if a mid-string fragment collides
	// with an obscure hamlet.
	for i := len(parts) - 1; i >= 0; i-- {
		if l, ok := g.countries[normalizeKey(parts[i])]; ok {
			return l, true
		}
	}
	return Location{}, false
}

// lookupFragment resolves a single fragment (no comma splitting) against
// the city index and then the country-centroid table.
func (g *Geocoder) lookupFragment(fragment string) (Location, bool) {
	key := normalizeKey(fragment)
	if key == "" {
		return Location{}, false
	}
	if l, ok := g.cities[key]; ok {
		return l, true
	}
	if l, ok := g.countries[key]; ok {
		return l, true
	}
	return Location{}, false
}

// splitLocationParts breaks a raw location on commas / slashes / pipes
// and returns trimmed non-empty fragments in original order.
func splitLocationParts(loc string) []string {
	f := func(r rune) bool {
		switch r {
		case ',', '/', '|', ';':
			return true
		}
		return false
	}
	raw := strings.FieldsFunc(loc, f)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeKey returns the case-folded, whitespace-collapsed, marker-free
// form of s so unicode variants, punctuation, and stray whitespace all
// map to the same lookup key. Emoji and other non-letter runes are
// stripped so profile locations like "🇯🇵 Tokyo" match "Tokyo".
func normalizeKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '\'':
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			// Drop punctuation, emoji, symbols — they should not affect
			// the match key.
		}
	}
	return strings.TrimSpace(b.String())
}
