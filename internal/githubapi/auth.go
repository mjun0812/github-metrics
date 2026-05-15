// Package githubapi wraps GitHub REST and GraphQL access, the rate
// tracker, and the MOCKED_TOKEN safety guard. See
// specs/001-project-foundation/contracts/github-api.md for the
// authoritative shape of each exported type.
package githubapi

import (
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TokenKind enumerates the credential shapes the project recognizes.
// See contracts/github-api.md §2.1 for the mapping rules.
type TokenKind int

const (
	// TokenUnknown is returned for empty or unrecognized credentials.
	// Callers must treat it as a hard input error.
	TokenUnknown TokenKind = iota
	// TokenClassic covers the canonical "classic" personal access
	// tokens (ghp_, gho_, ghu_, ghs_, ghr_).
	TokenClassic
	// TokenFineGrained matches GitHub's newer fine-grained PATs. We
	// reject these early because upstream behavior depends on classic
	// scopes that fine-grained tokens cannot replicate yet.
	TokenFineGrained
	// TokenNone is the explicit "no token" sentinel for offline /
	// public-only runs.
	TokenNone
	// TokenMocked is the sentinel test helpers use to wire a fake
	// transport. mockedGuardRoundTripper panics on real GitHub
	// hostnames when this kind is active (FR-017).
	TokenMocked
)

// String returns a stable human-readable label for logging.
func (k TokenKind) String() string {
	switch k {
	case TokenClassic:
		return "classic"
	case TokenFineGrained:
		return "fine-grained"
	case TokenNone:
		return "none"
	case TokenMocked:
		return "mocked"
	default:
		return "unknown"
	}
}

// classicPrefixes lists every prefix that maps to TokenClassic. They
// share the format `gh<role>_<random>`.
var classicPrefixes = []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"}

// ClassifyToken inspects raw and returns the TokenKind it belongs to.
// An empty string yields TokenUnknown so callers can branch on it the
// same way as on a malformed value.
func ClassifyToken(raw string) TokenKind {
	switch raw {
	case "":
		return TokenUnknown
	case "NOT_NEEDED":
		return TokenNone
	case "MOCKED_TOKEN":
		return TokenMocked
	}
	if strings.HasPrefix(raw, "github_pat_") {
		return TokenFineGrained
	}
	for _, p := range classicPrefixes {
		if strings.HasPrefix(raw, p) {
			return TokenClassic
		}
	}
	return TokenUnknown
}

// ValidateToken returns nil when raw is acceptable for outbound calls,
// or a typed *InputError describing why it cannot be used. Centralized
// here so REST and GraphQL constructors share one decision.
func ValidateToken(raw string) error {
	kind := ClassifyToken(raw)
	switch kind {
	case TokenClassic, TokenNone, TokenMocked:
		return nil
	case TokenFineGrained:
		return xerrors.NewInputError("token", errFineGrained)
	default:
		return xerrors.NewInputError("token", errUnknownTokenShape)
	}
}

// Sentinel error values keep the strings out of the hot path while
// still surfacing useful detail via errors.As(InputError).
var (
	errFineGrained       = stringError("github_pat_ (fine-grained) tokens are not supported in M1; use a classic ghp_/gho_/ghs_ token or NOT_NEEDED")
	errUnknownTokenShape = stringError("token does not match any recognized prefix (ghp_/gho_/ghu_/ghs_/ghr_), NOT_NEEDED, or MOCKED_TOKEN")
)

// stringError lets us declare error sentinels without depending on
// errors.New (which would conflict with internal/errors alias names if
// we ever consolidate the imports).
type stringError string

func (e stringError) Error() string { return string(e) }
