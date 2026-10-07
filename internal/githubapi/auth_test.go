package githubapi_test

import (
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
)

func TestClassifyToken_TableCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want githubapi.TokenKind
	}{
		{raw: "ghp_AAAAA", want: githubapi.TokenClassic},
		{raw: "github_pat_AAAAAA", want: githubapi.TokenFineGrained},
		{raw: "NOT_NEEDED", want: githubapi.TokenNone},
		{raw: "MOCKED_TOKEN", want: githubapi.TokenMocked},
		{raw: "", want: githubapi.TokenUnknown},
		{raw: "junk", want: githubapi.TokenUnknown},
		{raw: "ghp", want: githubapi.TokenUnknown}, // prefix without underscore
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got := githubapi.ClassifyToken(tc.raw)
			if got != tc.want {
				t.Fatalf("ClassifyToken(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestValidateToken_AcceptsClassicNoneAndMocked(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"ghp_AAAA", "gho_AAAA", "NOT_NEEDED", "MOCKED_TOKEN"} {
		if err := githubapi.ValidateToken(raw); err != nil {
			t.Errorf("ValidateToken(%q) = %v, want nil", raw, err)
		}
	}
}

func TestValidateToken_RejectsFineGrainedAndUnknownAsInputError(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"github_pat_secret", "notatoken"} {
		var ie *xerrors.InputError
		if err := githubapi.ValidateToken(raw); !xerrors.As(err, &ie) {
			t.Fatalf("ValidateToken(%q): error is not *InputError: %v", raw, err)
		}
		if ie.Field != "token" {
			t.Fatalf("ValidateToken(%q): InputError.Field = %q, want %q", raw, ie.Field, "token")
		}
	}
}
