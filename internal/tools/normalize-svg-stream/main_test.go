package main

import (
	"bytes"
	"testing"
)

// applyMasks mirrors main() body for hermetic testing.
func applyMasks(raw []byte) []byte {
	out := lastUpdatedRE.ReplaceAll(raw, []byte("Last updated __MASKED__"))
	out = versionRE.ReplaceAll(out, []byte("github-metrics@__MASKED__"))
	return out
}

func TestMasksDynamic(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><text>Last updated 2026-05-19T12:34:56Z</text><text>foo/github-metrics@1.2.3-abc</text></svg>`)
	out := applyMasks(in)
	if bytes.Contains(out, []byte("2026-05-19T12:34:56Z")) {
		t.Errorf("Last-updated timestamp not masked: %s", out)
	}
	if bytes.Contains(out, []byte("1.2.3-abc")) {
		t.Errorf("version string not masked: %s", out)
	}
	if bytes.Count(out, []byte("__MASKED__")) != 2 {
		t.Errorf("expected 2 __MASKED__ tokens, got %d: %s", bytes.Count(out, []byte("__MASKED__")), out)
	}
}

func TestIdempotent(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><g transform="translate(5,5)"><text>Last updated 2026-05-19</text><text>repo/github-metrics@v1.0.1</text></g></svg>`)
	first := applyMasks(in)
	second := applyMasks(first)
	if !bytes.Equal(first, second) {
		t.Errorf("not idempotent:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestPassThrough_NoDynamic(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><text>foo</text></svg>`)
	out := applyMasks(in)
	if !bytes.Equal(in, out) {
		t.Errorf("input without dynamic strings should be unchanged:\nin:  %s\nout: %s", in, out)
	}
}
