package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
)

func TestLoad_AbsentFileReturnsDefaultPort(t *testing.T) {
	t.Parallel()

	s, err := config.LoadSettings(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if s == nil {
		t.Fatalf("Load returned nil settings without error")
	}
	if s.Port != 3000 {
		t.Fatalf("default port = %d, want 3000", s.Port)
	}
	if s.Sandbox {
		t.Fatalf("missing-file path should not enable sandbox")
	}
}

func TestLoad_StripsCommentKeys(t *testing.T) {
	t.Parallel()

	s, err := config.LoadSettings("../../tests/fixtures/settings/with_comments.json")
	if err != nil {
		t.Fatalf("Load(with_comments): %v", err)
	}
	if s.Port != 3000 {
		t.Fatalf("Port from upstream example = %d, want 3000", s.Port)
	}
	if s.Templates.Default != "classic" {
		t.Fatalf("Templates.Default = %q, want %q", s.Templates.Default, "classic")
	}
	if !contains(s.Outputs, "svg") {
		t.Fatalf("Outputs missing svg, got %v", s.Outputs)
	}
}

func TestLoad_MinimalRoundTrip(t *testing.T) {
	t.Parallel()

	s, err := config.LoadSettings("../../tests/fixtures/settings/minimal.json")
	if err != nil {
		t.Fatalf("Load(minimal): %v", err)
	}
	if s.Port != 4040 {
		t.Fatalf("Port = %d, want 4040", s.Port)
	}
	if s.Token != "ghp_minimal" {
		t.Fatalf("Token = %q, want ghp_minimal", s.Token)
	}
}

func TestLoad_SandboxOverridesEnforced(t *testing.T) {
	t.Parallel()

	s, err := config.LoadSettings("../../tests/fixtures/settings/sandbox.json")
	if err != nil {
		t.Fatalf("Load(sandbox): %v", err)
	}
	if !s.Sandbox {
		t.Fatalf("sandbox flag not parsed")
	}
	if !s.Optimize.Enabled {
		t.Fatalf("Sandbox MUST force Optimize.Enabled=true, got false")
	}
	if s.Cached != 0 {
		t.Fatalf("Sandbox MUST force Cached=0, got %d", s.Cached)
	}
	if !s.PluginsDefault {
		t.Fatalf("Sandbox MUST force PluginsDefault=true")
	}
	if !s.Extras.Default {
		t.Fatalf("Sandbox MUST force Extras.Default=true")
	}
	if !s.Mocked.Enabled {
		t.Fatalf("Sandbox MUST force Mocked.Enabled=true")
	}
}

func TestStripCommentKeysViaReader(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{
		"//": "first comment",
		"token": "kept",
		"//2": "second comment",
		"nested": {"//": "inner", "value": 42, "//3": "trailing"},
		"list": [{"//": "in-array", "x": 1}, {"y": 2}]
	}`)
	s, err := config.LoadSettingsReader(input)
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if s.Token != "kept" {
		t.Fatalf("Token = %q, want kept", s.Token)
	}
}

func TestOptimizeFlag_AcceptsBoolAndList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw         string
		wantEnabled bool
		wantPasses  []string
	}{
		{raw: `true`, wantEnabled: true},
		{raw: `false`, wantEnabled: false},
		{raw: `["css", "xml"]`, wantEnabled: true, wantPasses: []string{"css", "xml"}},
		{raw: `[]`, wantEnabled: false, wantPasses: []string{}},
		{raw: `null`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			var f config.OptimizeFlag
			if err := json.Unmarshal([]byte(tc.raw), &f); err != nil {
				t.Fatalf("Unmarshal %q: %v", tc.raw, err)
			}
			if f.Enabled != tc.wantEnabled {
				t.Fatalf("Enabled = %v, want %v", f.Enabled, tc.wantEnabled)
			}
			if !equalStrings(f.Passes, tc.wantPasses) {
				t.Fatalf("Passes = %v, want %v", f.Passes, tc.wantPasses)
			}
		})
	}
}

func TestMockFlag_AcceptsBoolAndForce(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw         string
		wantEnabled bool
		wantForce   bool
	}{
		{raw: `true`, wantEnabled: true},
		{raw: `false`},
		{raw: `"force"`, wantEnabled: true, wantForce: true},
		{raw: `null`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			var m config.MockFlag
			if err := json.Unmarshal([]byte(tc.raw), &m); err != nil {
				t.Fatalf("Unmarshal %q: %v", tc.raw, err)
			}
			if m.Enabled != tc.wantEnabled || m.Force != tc.wantForce {
				t.Fatalf("got Enabled=%v Force=%v, want %v / %v",
					m.Enabled, m.Force, tc.wantEnabled, tc.wantForce)
			}
		})
	}
}

func TestNoToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want bool
	}{
		{raw: `{"token": "NOT_NEEDED"}`, want: true},
		{raw: `{"token": "ghp_xxx"}`, want: false},
		{raw: `{"token": ""}`, want: false},
		{raw: `{}`, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			s, err := config.LoadSettingsReader(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatalf("LoadReader: %v", err)
			}
			if got := s.NoToken(); got != tc.want {
				t.Fatalf("NoToken = %v, want %v (raw=%s)", got, tc.want, tc.raw)
			}
		})
	}
}

func TestLoad_InvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(tmp, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := config.LoadSettings(tmp); err == nil {
		t.Fatalf("Load on invalid JSON returned no error")
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
