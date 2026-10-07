package errors_test

import (
	stderrors "errors"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

func TestUnwrapPreservesChain(t *testing.T) {
	t.Parallel()

	root := stderrors.New("network unreachable")
	wrapped := xerrors.NewRetryableError(root)
	if !xerrors.Is(wrapped, root) {
		t.Fatalf("errors.Is did not see through wrapped chain")
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
