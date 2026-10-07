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
		{1023, "1023 B"},
		{1024, "1.0 kB"},
		{1024 * 1024 * 3 / 2, "1.5 MB"},
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
		{0.123, "12.3%"},
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
		writeDetailsRows(&b, bars, []string{"bytes-size", "percentage"}, pc, 0)
		out := b.String()
		// #409 Phase B7: two-column layout places the second language
		// (Python, index 1) in the right column, so its color-dot icon is
		// translated to x=248 (240 + 8px field inset).
		if !strings.Contains(out, `transform="translate(248,`) {
			t.Errorf("expected second column (translate x=248) in two-col layout: %s", out)
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
		writeDetailsRows(&b, bars, []string{"lines", "bytes-size", "percentage"}, pc, 0)
		out := b.String()
		// Single-column layout keeps every language in column 0, so no
		// row is translated to the right column (x=248).
		if strings.Contains(out, `transform="translate(248,`) {
			t.Errorf("expected single-col layout (no x=248 column): %s", out)
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
		writeDetailsRows(&b, bars, []string{"lines", "bytes-size"}, pc, 0)
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
