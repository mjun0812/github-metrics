// Package config holds settings, metadata, and inputs loaders for
// github-metrics. The Token type in this file is the opaque wrapper for
// credentials that the rest of the codebase must use instead of bare
// strings, so leaking a raw token via fmt.Sprintf, log output, or JSON
// marshalling becomes impossible by construction.
package config

// Token is an opaque holder for a GitHub credential. Its zero value is
// safe to use and reports as empty in every observable surface
// (Stringer, GoStringer, MarshalJSON). Callers retrieve the underlying
// secret with [Token.Reveal] only at the network boundary
// (internal/githubapi).
type Token struct {
	raw string
}

// NewToken wraps a raw credential string. Empty input yields a zero-
// value token which subsequent calls treat as "absent".
func NewToken(raw string) Token {
	return Token{raw: raw}
}

// String reports a placeholder so accidental fmt.Print or slog logging
// cannot leak the credential. Empty tokens return an empty string so
// "(provided)" never appears for absent credentials.
func (t Token) String() string {
	if t.raw == "" {
		return ""
	}
	return "(provided)"
}

// GoString mirrors String so %#v formatting is also safe.
func (t Token) GoString() string { return t.String() }

// MarshalJSON ensures encoding/json output never exposes the raw value;
// it surfaces the same placeholder used by String.
func (t Token) MarshalJSON() ([]byte, error) {
	if t.raw == "" {
		return []byte(`""`), nil
	}
	return []byte(`"(provided)"`), nil
}

// Reveal returns the raw credential. Call sites are intentionally
// limited to internal/githubapi and test helpers — grep for it during
// code review to keep the surface narrow.
func (t Token) Reveal() string { return t.raw }

// Present reports whether the token carries a non-empty credential.
func (t Token) Present() bool { return t.raw != "" }
