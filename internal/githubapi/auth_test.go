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
		{raw: "gho_AAAAA", want: githubapi.TokenClassic},
		{raw: "ghu_AAAAA", want: githubapi.TokenClassic},
		{raw: "ghs_AAAAA", want: githubapi.TokenClassic},
		{raw: "ghr_AAAAA", want: githubapi.TokenClassic},
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

func TestTokenKindString(t *testing.T) {
	t.Parallel()

	cases := map[githubapi.TokenKind]string{
		githubapi.TokenClassic:     "classic",
		githubapi.TokenFineGrained: "fine-grained",
		githubapi.TokenNone:        "none",
		githubapi.TokenMocked:      "mocked",
		githubapi.TokenUnknown:     "unknown",
	}
	for k, want := range cases {
		if k.String() != want {
			t.Errorf("%d.String() = %q, want %q", k, k.String(), want)
		}
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

func TestValidateToken_RejectsFineGrainedAsInputError(t *testing.T) {
	t.Parallel()

	err := githubapi.ValidateToken("github_pat_secret")
	if err == nil {
		t.Fatalf("expected error")
	}
	var ie *xerrors.InputError
	if !xerrors.As(err, &ie) {
		t.Fatalf("error is not *InputError: %T", err)
	}
	if ie.Field != "token" {
		t.Fatalf("InputError.Field = %q, want %q", ie.Field, "token")
	}
}

func TestValidateToken_RejectsUnknownAsInputError(t *testing.T) {
	t.Parallel()

	err := githubapi.ValidateToken("notatoken")
	var ie *xerrors.InputError
	if !xerrors.As(err, &ie) {
		t.Fatalf("error is not *InputError: %T", err)
	}
	if ie.Field != "token" {
		t.Fatalf("InputError.Field = %q", ie.Field)
	}
}
