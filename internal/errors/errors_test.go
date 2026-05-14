package errors_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

func TestErrorsAsIdentifiesEachType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		target any
		want   bool
	}{
		{name: "InputError", err: xerrors.NewInputError("token", nil), target: new(*xerrors.InputError), want: true},
		{name: "NotFoundError", err: xerrors.NewNotFoundError("login", nil), target: new(*xerrors.NotFoundError), want: true},
		{name: "ForbiddenError", err: xerrors.NewForbiddenError("scope", nil), target: new(*xerrors.ForbiddenError), want: true},
		{name: "UnsupportedFormatError", err: xerrors.NewUnsupportedFormatError("png", nil), target: new(*xerrors.UnsupportedFormatError), want: true},
		{name: "RetryableError", err: xerrors.NewRetryableError(nil), target: new(*xerrors.RetryableError), want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := xerrors.As(tc.err, tc.target)
			if got != tc.want {
				t.Fatalf("errors.As(%T) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestErrorsAsRejectsCrossType(t *testing.T) {
	t.Parallel()

	err := xerrors.NewInputError("user", nil)
	var nf *xerrors.NotFoundError
	if xerrors.As(err, &nf) {
		t.Fatalf("errors.As crossed types: %T → %T", err, nf)
	}
}

func TestUnwrapPreservesChain(t *testing.T) {
	t.Parallel()

	root := stderrors.New("network unreachable")
	wrapped := xerrors.NewRetryableError(root)

	if !xerrors.Is(wrapped, root) {
		t.Fatalf("errors.Is did not see through wrapped chain")
	}
}

func TestErrorMessagesIncludeFieldAndCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"input no cause", xerrors.NewInputError("token", nil), `invalid input "token"`},
		{"input with cause", xerrors.NewInputError("token", fmt.Errorf("empty")), `invalid input "token": empty`},
		{"not found no cause", xerrors.NewNotFoundError("user", nil), `not found "user"`},
		{"forbidden no cause", xerrors.NewForbiddenError("scope", nil), `forbidden (scope)`},
		{"unsupported format no cause", xerrors.NewUnsupportedFormatError("pdf", nil), `unsupported format "pdf"`},
		{"retryable no cause", xerrors.NewRetryableError(nil), `retryable`},
		{"retryable with cause", xerrors.NewRetryableError(fmt.Errorf("503")), `retryable: 503`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNilReceiversReturnEmptyString(t *testing.T) {
	t.Parallel()

	cases := []error{
		(*xerrors.InputError)(nil),
		(*xerrors.NotFoundError)(nil),
		(*xerrors.ForbiddenError)(nil),
		(*xerrors.UnsupportedFormatError)(nil),
		(*xerrors.RetryableError)(nil),
	}
	for i, e := range cases {
		if e.Error() != "" {
			t.Fatalf("case %d: nil receiver returned %q, want empty", i, e.Error())
		}
	}
}
