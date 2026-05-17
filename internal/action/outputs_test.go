package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetOutput_SingleLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "github_output")
	if err := appendOutput(path, "metrics_url", "https://github.com/o/r/blob/main/x.svg"); err != nil {
		t.Fatalf("appendOutput: %v", err)
	}
	body, _ := os.ReadFile(path)
	want := "metrics_url=https://github.com/o/r/blob/main/x.svg\n"
	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
}

func TestSetOutput_Multiline_Heredoc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "github_output")
	value := "line1\nline2\nline3"
	if err := appendOutput(path, "metrics_metadata", value); err != nil {
		t.Fatalf("appendOutput: %v", err)
	}
	body, _ := os.ReadFile(path)
	s := string(body)
	if !strings.HasPrefix(s, "metrics_metadata<<EOF_") {
		t.Errorf("missing heredoc prefix; got %q", s)
	}
	if !strings.Contains(s, "line1\nline2\nline3\n") {
		t.Errorf("missing value lines; got %q", s)
	}
}

func TestSetOutput_AppendsNotTruncates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "github_output")
	if err := appendOutput(path, "a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := appendOutput(path, "b", "2"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "a=1\nb=2\n" {
		t.Errorf("body = %q, want a=1\\nb=2\\n", string(body))
	}
}

func TestSetOutput_NoEnvVarIsNoOp(t *testing.T) {
	// not t.Parallel(): t.Setenv mutates process env which cannot be
	// done in parallel with other tests that also touch env.
	t.Setenv("GITHUB_OUTPUT", "")
	if err := SetOutput("k", "v"); err != nil {
		t.Errorf("expected no-op when GITHUB_OUTPUT unset; got %v", err)
	}
}

func TestSetOutput_RejectsInvalidKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "github_output")
	for _, badKey := range []string{"key=with=eq", "key\nwith\nnl"} {
		if err := appendOutput(path, badKey, "v"); err == nil {
			t.Errorf("expected error for invalid key %q", badKey)
		}
	}
}
