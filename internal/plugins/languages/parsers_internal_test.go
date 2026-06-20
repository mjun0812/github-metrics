package languages

import (
	"testing"
	"time"
)

func TestParseThreshold(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"0%", 0},
		{"5%", 0.05},
		{"  5%  ", 0.05},
		{"100%", 1.0},
		{"0.05", 0.05},
		{"0.5", 0.5},
		{"abc", 0},
		{"%", 0}, // percent sign with empty numeric → ParseFloat fails → 0
		{"-5%", -0.05},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := parseThreshold(tc.in); got != tc.want {
				t.Errorf("parseThreshold(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDurationLoose(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want time.Duration
		ok   bool
	}{
		{"time.Duration", 5 * time.Second, 5 * time.Second, true},
		{"int seconds", 30, 30 * time.Second, true},
		{"int64 seconds", int64(45), 45 * time.Second, true},
		{"float64 seconds", 7.5, 7500 * time.Millisecond, true},
		{"string 30s", "30s", 30 * time.Second, true},
		{"string 5min translates to 5m", "5min", 5 * time.Minute, true},
		{"string 7.5min translates to 7m30s", "7.5min", 7*time.Minute + 30*time.Second, true},
		{"string trimmed", "  30s  ", 30 * time.Second, true},
		{"string unparsable", "garbage", 0, false},
		{"string 5minutes confuses replacer", "5minutes", 0, false},
		{"nil", nil, 0, false},
		{"unsupported bool", true, 0, false},
		{"unsupported slice", []int{1}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseDurationLoose(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Errorf("parseDurationLoose(%v) = (%v, %v), want (%v, %v)",
					tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestSplitPair(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, sep             string
		wantLeft, wantRight string
		wantOK              bool
	}{
		{"a:b", ":", "a", "b", true},
		{"a : b", ":", "a", "b", true},
		{"a::b", ":", "a", ":b", true},
		{":b", ":", "", "", false},
		{"a:", ":", "", "", false},
		{"no-sep", ":", "", "", false},
		{"", ":", "", "", false},
		{"=foo=bar", "=", "", "", false}, // empty left
		{"key=value", "=", "key", "value", true},
		{"k -> v", " -> ", "k", "v", true}, // multi-char separator
	}
	for _, tc := range cases {
		t.Run(tc.in+"|"+tc.sep, func(t *testing.T) {
			t.Parallel()
			l, r, ok := splitPair(tc.in, tc.sep)
			if l != tc.wantLeft || r != tc.wantRight || ok != tc.wantOK {
				t.Errorf("splitPair(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, tc.sep, l, r, ok, tc.wantLeft, tc.wantRight, tc.wantOK)
			}
		})
	}
}
