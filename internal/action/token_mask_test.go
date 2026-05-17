package action

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
)

const realPATSentinel = "ghp_realToken_DO_NOT_LEAK_12345"

// TestTokenMask_BannerOutput confirms the banner uses the masked
// stringer (config.Token.String()), not the raw PAT.
func TestTokenMask_BannerOutput(t *testing.T) {
	t.Parallel()
	tok := config.NewToken(realPATSentinel)
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{
		Version:     "test",
		TokenMasked: tok.String(), // <- callers MUST pass tok.String(), not tok.Reveal()
	})
	if strings.Contains(buf.String(), realPATSentinel) {
		t.Errorf("banner leaked raw PAT: %q", buf.String())
	}
}

// TestTokenMask_ErrorWrap verifies error wrapping (fmt.Errorf %w)
// does not surface a config.Token's raw value. config.Token does
// not directly appear in error chains by design — the caller is
// expected to wrap meaningful context only.
func TestTokenMask_ErrorWrap(t *testing.T) {
	t.Parallel()
	tok := config.NewToken(realPATSentinel)
	err := fmt.Errorf("compute failed: token=%s: %w", tok, errors.New("underlying"))
	if strings.Contains(err.Error(), realPATSentinel) {
		t.Errorf("error chain leaked raw PAT: %v", err)
	}
}

// TestTokenMask_SlogField verifies slog.Info("...", "token", tok)
// (i.e., tok as a structured field value) does not surface the raw
// PAT. config.Token's Stringer is invoked when the handler calls
// Sprint on the field value.
func TestTokenMask_SlogField(t *testing.T) {
	t.Parallel()
	tok := config.NewToken(realPATSentinel)
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, nil)
	logger := slog.New(h)
	logger.Info("compute failed", "token", tok)
	if strings.Contains(buf.String(), realPATSentinel) {
		t.Errorf("slog field leaked raw PAT: %s", buf.String())
	}
}

// TestTokenMask_StringerFormat checks the Stringer mask pattern is
// non-empty + does NOT include the raw value. The exact mask string
// is set by config; this test pins the contract (= mask is used).
func TestTokenMask_StringerFormat(t *testing.T) {
	t.Parallel()
	tok := config.NewToken(realPATSentinel)
	s := tok.String()
	if s == "" {
		t.Errorf("Stringer returned empty; expected mask")
	}
	if strings.Contains(s, realPATSentinel) {
		t.Errorf("Stringer returned raw PAT: %q", s)
	}
}
