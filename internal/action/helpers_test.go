package action

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// ---------------------------------------------------------------------------
// accountForTemplate
// ---------------------------------------------------------------------------

func TestAccountForTemplate_Repository(t *testing.T) {
	t.Parallel()
	if got := accountForTemplate("repository"); got != plugins.AccountRepository {
		t.Errorf("accountForTemplate(repository) = %v, want AccountRepository", got)
	}
}

func TestAccountForTemplate_Classic(t *testing.T) {
	t.Parallel()
	if got := accountForTemplate("classic"); got != plugins.AccountUser {
		t.Errorf("accountForTemplate(classic) = %v, want AccountUser", got)
	}
}

func TestAccountForTemplate_Empty(t *testing.T) {
	t.Parallel()
	if got := accountForTemplate(""); got != plugins.AccountUser {
		t.Errorf("accountForTemplate(\"\") = %v, want AccountUser", got)
	}
}

func TestAccountForTemplate_Unknown(t *testing.T) {
	t.Parallel()
	if got := accountForTemplate("anything-else"); got != plugins.AccountUser {
		t.Errorf("accountForTemplate(anything-else) = %v, want AccountUser", got)
	}
}

// ---------------------------------------------------------------------------
// targetOutputPath
// ---------------------------------------------------------------------------

func TestTargetOutputPath_Stdout(t *testing.T) {
	t.Parallel()
	inv := &Invocation{OutputFilename: "-", OutputDir: "/tmp/out"}
	if got := targetOutputPath(inv); got != "-" {
		t.Errorf("targetOutputPath with '-' = %q, want %q", got, "-")
	}
}

func TestTargetOutputPath_Absolute(t *testing.T) {
	t.Parallel()
	inv := &Invocation{OutputFilename: "/absolute/path/out.svg", OutputDir: "/tmp/out"}
	if got := targetOutputPath(inv); got != "/absolute/path/out.svg" {
		t.Errorf("targetOutputPath with absolute = %q, want %q", got, "/absolute/path/out.svg")
	}
}

func TestTargetOutputPath_Relative(t *testing.T) {
	t.Parallel()
	inv := &Invocation{OutputFilename: "github-metrics.svg", OutputDir: "/tmp/out"}
	want := "/tmp/out/github-metrics.svg"
	if got := targetOutputPath(inv); got != want {
		t.Errorf("targetOutputPath with relative = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// writeOutputFile
// ---------------------------------------------------------------------------

func TestWriteOutputFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "out.svg")
	content := []byte("<svg>hello</svg>")
	if err := writeOutputFile(path, content); err != nil {
		t.Fatalf("writeOutputFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestWriteOutputFile_UnwritableDir(t *testing.T) {
	t.Parallel()
	// Use an invalid path that cannot be created.
	// On Darwin/Linux, writing under a file (not dir) causes MkdirAll to fail.
	dir := t.TempDir()
	// Create a regular file where we want a directory.
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Attempt to write under the file (path includes it as a directory component).
	path := filepath.Join(blocker, "sub", "out.svg")
	if err := writeOutputFile(path, []byte("data")); err == nil {
		t.Error("expected error when writing under a file path")
	}
}

// ---------------------------------------------------------------------------
// intInput
// ---------------------------------------------------------------------------

func TestIntInput_Absent(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{}, "x", 42); got != 42 {
		t.Errorf("absent key: got %d, want 42", got)
	}
}

func TestIntInput_Int(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{"x": 7}, "x", 0); got != 7 {
		t.Errorf("int type: got %d, want 7", got)
	}
}

func TestIntInput_Int64(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{"x": int64(99)}, "x", 0); got != 99 {
		t.Errorf("int64 type: got %d, want 99", got)
	}
}

func TestIntInput_Float64(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{"x": float64(3)}, "x", 0); got != 3 {
		t.Errorf("float64 type: got %d, want 3", got)
	}
}

func TestIntInput_StringNumeric(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{"x": "42"}, "x", 0); got != 42 {
		t.Errorf("string \"42\": got %d, want 42", got)
	}
}

func TestIntInput_StringNonNumeric(t *testing.T) {
	t.Parallel()
	if got := intInput(map[string]any{"x": "nope"}, "x", 5); got != 5 {
		t.Errorf("string non-numeric: got %d, want default 5", got)
	}
}

// ---------------------------------------------------------------------------
// durationSecInput
// ---------------------------------------------------------------------------

func TestDurationSecInput_Absent(t *testing.T) {
	t.Parallel()
	def := 2 * time.Second
	if got := durationSecInput(map[string]any{}, "d", def); got != def {
		t.Errorf("absent: got %v, want %v", got, def)
	}
}

func TestDurationSecInput_Int(t *testing.T) {
	t.Parallel()
	// 10 (seconds, per the action.yml contract) → 10s duration
	got := durationSecInput(map[string]any{"d": 10}, "d", time.Second)
	if got != 10*time.Second {
		t.Errorf("int 10: got %v, want 10s", got)
	}
}

func TestDurationSecInput_StringNumeric(t *testing.T) {
	t.Parallel()
	got := durationSecInput(map[string]any{"d": "30"}, "d", time.Second)
	if got != 30*time.Second {
		t.Errorf("string \"30\": got %v, want 30s", got)
	}
}

type testStringer string

func (s testStringer) String() string {
	return fmt.Sprintf("stringer:%s", string(s))
}

func TestStringInput_Edges(t *testing.T) {
	t.Parallel()
	if got := stringInput(map[string]any{"x": ""}, "x", "fallback"); got != "fallback" {
		t.Errorf("empty string = %q, want fallback", got)
	}
	if got := stringInput(map[string]any{"x": testStringer("value")}, "x", "fallback"); got != "stringer:value" {
		t.Errorf("Stringer = %q, want stringer:value", got)
	}
	if got := stringInput(map[string]any{"x": 123}, "x", "fallback"); got != "123" {
		t.Errorf("default fmt = %q, want 123", got)
	}
}

// ---------------------------------------------------------------------------
// isTruthy
// ---------------------------------------------------------------------------

func TestIsTruthy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input any
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string yes", "yes", true},
		{"string 1", "1", true},
		{"string on", "on", true},
		{"string TRUE uppercase", "TRUE", true},
		{"string YES uppercase", "YES", true},
		{"string false", "false", false},
		{"string no", "no", false},
		{"string empty", "", false},
		{"int 1", 1, true},
		{"int 0", 0, false},
		{"int -1", -1, true},
		{"float64 1.0", float64(1.0), true},
		{"float64 0.0", float64(0.0), false},
		{"other type nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTruthy(tc.input); got != tc.want {
				t.Errorf("isTruthy(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pluginFlag.String()
// ---------------------------------------------------------------------------

func TestPluginFlag_String_Nil(t *testing.T) {
	t.Parallel()
	var p *pluginFlag
	if got := p.String(); got != "" {
		t.Errorf("nil pluginFlag.String() = %q, want %q", got, "")
	}
}

func TestPluginFlag_String_Empty(t *testing.T) {
	t.Parallel()
	p := &pluginFlag{m: map[string]string{}}
	if got := p.String(); got != "" {
		t.Errorf("empty pluginFlag.String() = %q, want %q", got, "")
	}
}

func TestPluginFlag_String_Single(t *testing.T) {
	t.Parallel()
	p := &pluginFlag{m: map[string]string{"plugin_languages": "true"}}
	want := "plugin_languages=true"
	if got := p.String(); got != want {
		t.Errorf("single: got %q, want %q", got, want)
	}
}

func TestPluginFlag_String_MultipleIsSorted(t *testing.T) {
	t.Parallel()
	p := &pluginFlag{m: map[string]string{
		"plugin_languages": "true",
		"plugin_activity":  "yes",
	}}
	want := "plugin_activity=yes,plugin_languages=true"
	if got := p.String(); got != want {
		t.Errorf("multiple sorted: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// InputError.Error()
// ---------------------------------------------------------------------------

func TestInputError_Error(t *testing.T) {
	t.Parallel()
	ie := &InputError{Key: "token", Msg: "token is required"}
	if got := ie.Error(); got != "token is required" {
		t.Errorf("InputError.Error() = %q, want %q", got, "token is required")
	}
}
