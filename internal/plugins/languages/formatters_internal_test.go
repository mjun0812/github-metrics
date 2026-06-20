package languages

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 kB"},
		{1536, "1.5 kB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 3 / 2, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 2, "2.0 GB"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := formatBytes(tc.in); got != tc.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.0%"},
		{0.123, "12.3%"},
		{0.5, "50.0%"},
		{1.0, "100.0%"},
		{0.0001, "0.0%"}, // rounding
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := formatPercent(tc.in); got != tc.want {
				t.Errorf("formatPercent(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDetailIncludes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		details []string
		needle  string
		want    bool
	}{
		{"nil slice", nil, "lines", false},
		{"empty slice", []string{}, "lines", false},
		{"first hit", []string{"lines", "bytes-size"}, "lines", true},
		{"middle hit", []string{"bytes-size", "lines", "percentage"}, "lines", true},
		{"miss", []string{"bytes-size"}, "lines", false},
		{"case-sensitive miss", []string{"Lines"}, "lines", false},
		{"substring is not a match", []string{"line"}, "lines", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := detailIncludes(tc.details, tc.needle); got != tc.want {
				t.Errorf("detailIncludes(%v, %q) = %v, want %v", tc.details, tc.needle, got, tc.want)
			}
		})
	}
}

func TestHasRecentSection(t *testing.T) {
	t.Parallel()

	mk := func(populate func(d *plugins.Data)) *templates.PartialContext {
		d := plugins.NewData()
		if populate != nil {
			populate(d)
		}
		return &templates.PartialContext{Data: d}
	}

	cases := []struct {
		name string
		pc   *templates.PartialContext
		want bool
	}{
		{"nil pc", nil, false},
		{"nil Data", &templates.PartialContext{}, false},
		{"no plugin payload", mk(nil), false},
		{
			"recent skipped",
			mk(func(d *plugins.Data) {
				d.SetPlugin(RecentName, &RecentResult{Skipped: true, Favorites: []plugins.LanguageStat{{Name: "Go", Size: 1}}})
			}),
			false,
		},
		{
			"recent has favorites",
			mk(func(d *plugins.Data) {
				d.SetPlugin(RecentName, &RecentResult{Favorites: []plugins.LanguageStat{{Name: "Go", Size: 1}}})
			}),
			true,
		},
		{
			"recent has empty favorites + no indepth",
			mk(func(d *plugins.Data) {
				d.SetPlugin(RecentName, &RecentResult{Favorites: nil})
			}),
			false,
		},
		{
			"indepth has bytes",
			mk(func(d *plugins.Data) {
				d.SetPlugin(IndepthName, &IndepthResult{Total: LanguageBytes{Bytes: map[string]int64{"Go": 100}}})
			}),
			true,
		},
		{
			"indepth skipped",
			mk(func(d *plugins.Data) {
				d.SetPlugin(IndepthName, &IndepthResult{Skipped: true, Total: LanguageBytes{Bytes: map[string]int64{"Go": 100}}})
			}),
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := hasRecentSection(tc.pc); got != tc.want {
				t.Errorf("hasRecentSection: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIndepthBytesByLanguage(t *testing.T) {
	t.Parallel()

	t.Run("nil pc returns empty map", func(t *testing.T) {
		t.Parallel()
		if got := indepthBytesByLanguage(nil); len(got) != 0 {
			t.Errorf("nil pc: got %v", got)
		}
	})
	t.Run("nil Data returns empty map", func(t *testing.T) {
		t.Parallel()
		if got := indepthBytesByLanguage(&templates.PartialContext{}); len(got) != 0 {
			t.Errorf("nil Data: got %v", got)
		}
	})
	t.Run("missing plugin returns empty map", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		if got := indepthBytesByLanguage(pc); len(got) != 0 {
			t.Errorf("missing plugin: got %v", got)
		}
	})
	t.Run("skipped result returns empty map", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		pc.Data.SetPlugin(IndepthName, &IndepthResult{Skipped: true, Total: LanguageBytes{Bytes: map[string]int64{"Go": 1}}})
		if got := indepthBytesByLanguage(pc); len(got) != 0 {
			t.Errorf("skipped: got %v", got)
		}
	})
	t.Run("populated map round-trips", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		pc.Data.SetPlugin(IndepthName, &IndepthResult{Total: LanguageBytes{Bytes: map[string]int64{"Go": 4096, "Python": 1024}}})
		got := indepthBytesByLanguage(pc)
		if got["Go"] != 4096 || got["Python"] != 1024 || len(got) != 2 {
			t.Errorf("populated: got %v", got)
		}
	})
}

func TestIndepthLinesByLanguage(t *testing.T) {
	t.Parallel()

	t.Run("nil pc returns empty map", func(t *testing.T) {
		t.Parallel()
		if got := indepthLinesByLanguage(nil); len(got) != 0 {
			t.Errorf("nil pc: got %v", got)
		}
	})
	t.Run("missing plugin returns empty map", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		if got := indepthLinesByLanguage(pc); len(got) != 0 {
			t.Errorf("missing plugin: got %v", got)
		}
	})
	t.Run("wrong type returns empty map", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		pc.Data.SetPlugin(IndepthName, "not an *IndepthResult")
		if got := indepthLinesByLanguage(pc); len(got) != 0 {
			t.Errorf("wrong type: got %v", got)
		}
	})
	t.Run("populated map round-trips", func(t *testing.T) {
		t.Parallel()
		pc := &templates.PartialContext{Data: plugins.NewData()}
		pc.Data.SetPlugin(IndepthName, &IndepthResult{Total: LanguageBytes{Lines: map[string]int64{"Go": 200, "Python": 50}}})
		got := indepthLinesByLanguage(pc)
		if got["Go"] != 200 || got["Python"] != 50 || len(got) != 2 {
			t.Errorf("populated: got %v", got)
		}
	})
}

func TestWriteIndepthSection(t *testing.T) {
	t.Parallel()

	t.Run("no-op when indepth absent", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		pc := &templates.PartialContext{Data: plugins.NewData()}
		writeIndepthSection(&b, pc)
		if b.Len() != 0 {
			t.Errorf("expected no output, got %q", b.String())
		}
	})

	t.Run("happy path sorted by bytes desc then name", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		pc := &templates.PartialContext{Data: plugins.NewData()}
		pc.Data.SetPlugin(IndepthName, &IndepthResult{
			Total: LanguageBytes{Bytes: map[string]int64{
				"Python": 1000,
				"Go":     3000, // largest first
				"Rust":   1000, // tied with Python; alphabetical
			}},
		})
		writeIndepthSection(&b, pc)
		out := b.String()
		if !strings.Contains(out, `class="languages-indepth-wrapper"`) {
			t.Errorf("missing wrapper svg class: %q", out)
		}
		if !strings.Contains(out, `<g class="languages-indepth">`) {
			t.Errorf("missing inner g class: %q", out)
		}
		// Order: Go (3000) → Python (1000) → Rust (1000)
		goIdx := strings.Index(out, `data-language="Go"`)
		pyIdx := strings.Index(out, `data-language="Python"`)
		rsIdx := strings.Index(out, `data-language="Rust"`)
		if goIdx < 0 || pyIdx < 0 || rsIdx < 0 {
			t.Fatalf("missing one of Go/Python/Rust in: %q", out)
		}
		if goIdx >= pyIdx || pyIdx >= rsIdx {
			t.Errorf("expected Go < Python < Rust by offset, got Go=%d Python=%d Rust=%d", goIdx, pyIdx, rsIdx)
		}
		if !strings.Contains(out, `data-bytes="3000"`) {
			t.Errorf("missing data-bytes=3000: %q", out)
		}
	})
}

func TestWriteDetailsRows(t *testing.T) {
	t.Parallel()

	bars := []plugins.LanguageStat{
		{Name: "Go", Color: "#00ADD8", Size: 4000, Value: 0.8},
		{Name: "Python", Color: "#3572A5", Size: 1000, Value: 0.2},
	}

	t.Run("two columns when details has <= 2 entries", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		pc := &templates.PartialContext{Data: plugins.NewData()}
		writeDetailsRows(&b, bars, []string{"bytes-size", "percentage"}, pc)
		out := b.String()
		// Two-column layout emits two <section> wrappers.
		if strings.Count(out, "<section>") != 2 {
			t.Errorf("expected 2 sections (two-col layout), got %d in: %s", strings.Count(out, "<section>"), out)
		}
		if !strings.Contains(out, `data-language="Go"`) || !strings.Contains(out, `data-language="Python"`) {
			t.Errorf("missing language rows: %s", out)
		}
		if !strings.Contains(out, "3.9 kB") {
			t.Errorf("expected bytes-size column to be rendered: %s", out)
		}
		if !strings.Contains(out, "80.0%") {
			t.Errorf("expected percentage column to be rendered: %s", out)
		}
		if strings.Contains(out, "lines") {
			t.Errorf("unexpected lines column rendered: %s", out)
		}
	})

	t.Run("single column when details has > 2 entries", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		pc := &templates.PartialContext{Data: plugins.NewData()}
		writeDetailsRows(&b, bars, []string{"lines", "bytes-size", "percentage"}, pc)
		out := b.String()
		if strings.Count(out, "<section>") != 1 {
			t.Errorf("expected 1 section (single-col layout), got %d in: %s", strings.Count(out, "<section>"), out)
		}
	})

	t.Run("indepth bytes override the bars size", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		pc := &templates.PartialContext{Data: plugins.NewData()}
		// bars[0].Size = 4000 but indepth reports 5MB. The rendered
		// bytes-size column should reflect the indepth value.
		pc.Data.SetPlugin(IndepthName, &IndepthResult{
			Total: LanguageBytes{
				Bytes: map[string]int64{"Go": 5 * 1024 * 1024},
				Lines: map[string]int64{"Go": 200},
			},
		})
		writeDetailsRows(&b, bars, []string{"lines", "bytes-size"}, pc)
		out := b.String()
		if !strings.Contains(out, "5.0 MB") {
			t.Errorf("expected indepth-derived bytes-size 5.0 MB, got: %s", out)
		}
		// FormatCount uses k-suffix shortening for >=1000; keep value
		// below the cutoff so the literal byte count survives.
		if !strings.Contains(out, "200 lines") {
			t.Errorf("expected indepth-derived lines column (200 lines), got: %s", out)
		}
	})
}
