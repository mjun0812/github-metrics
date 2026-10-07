package action

import (
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

// ---------------------------------------------------------------------------
// intInput
// ---------------------------------------------------------------------------

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

func TestDurationSecInput_StringNumeric(t *testing.T) {
	t.Parallel()
	got := durationSecInput(map[string]any{"d": "30"}, "d", time.Second)
	if got != 30*time.Second {
		t.Errorf("string \"30\": got %v, want 30s", got)
	}
}

func TestStringInput_EmptyFallsBack(t *testing.T) {
	t.Parallel()
	if got := stringInput(map[string]any{"x": ""}, "x", "fallback"); got != "fallback" {
		t.Errorf("empty string = %q, want fallback", got)
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
		{"string true", "true", true},
		{"string 1", "1", true},
		{"string YES uppercase", "YES", true},
		{"string false", "false", false},
		{"string empty", "", false},
		{"int 1", 1, true},
		{"int 0", 0, false},
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

// ---------------------------------------------------------------------------
// InputError.Error()
// ---------------------------------------------------------------------------
