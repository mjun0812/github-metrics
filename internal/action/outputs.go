package action

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// SetOutput writes a `key=value` Action output line to the file at
// $GITHUB_OUTPUT (@actions/core v2 contract). When value contains a
// newline, a heredoc form is used with a random EOF delimiter that
// is guaranteed not to appear in value. See research R-003.
//
// The function appends to the file (does not truncate). Returns
// nil + a warning-equivalent no-op when GITHUB_OUTPUT is unset
// (CLI mode without GitHub Actions); callers that require strict
// behavior should check GITHUB_OUTPUT themselves.
func SetOutput(key, value string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		// Not running under GitHub Actions runner — no-op.
		return nil
	}
	return appendOutput(path, key, value)
}

// appendOutput is the exported behavior of SetOutput minus the
// $GITHUB_OUTPUT lookup; useful for tests that pin the destination.
func appendOutput(path, key, value string) error {
	if strings.Contains(key, "=") || strings.Contains(key, "\n") {
		return fmt.Errorf("action.SetOutput: invalid key %q (must not contain '=' or newline)", key)
	}
	var line string
	if strings.ContainsAny(value, "\n\r") {
		delim, err := uniqueDelimiter(value)
		if err != nil {
			return fmt.Errorf("action.SetOutput: %w", err)
		}
		// Heredoc form per @actions/core v2.
		line = fmt.Sprintf("%s<<%s\n%s\n%s\n", key, delim, value, delim)
	} else {
		line = fmt.Sprintf("%s=%s\n", key, value)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path is supplied by the GITHUB_OUTPUT env var (or test-controlled) — opening it for append is the documented @actions/core v2 protocol
	if err != nil {
		return fmt.Errorf("action.SetOutput: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("action.SetOutput: write %s: %w", path, err)
	}
	return nil
}

// uniqueDelimiter returns "EOF_<8 hex>" that is guaranteed not to
// appear in value. The 32-bit random suffix gives 2^32 collision
// space; we re-roll up to 4 times before giving up.
func uniqueDelimiter(value string) (string, error) {
	for i := 0; i < 4; i++ {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", fmt.Errorf("rand: %w", err)
		}
		candidate := "EOF_" + hex.EncodeToString(b[:])
		if !strings.Contains(value, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique heredoc delimiter")
}
