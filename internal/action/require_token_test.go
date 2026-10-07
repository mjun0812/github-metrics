package action

import (
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
)

// TestRequireTokenUnlessMocked anchors the pre-deps token gate added
// in #647 / PR #648. The helper is the user-facing diagnostic source
// for "no token configured"; it must surface tokenMissingMsg
// (canonical text) when the token is empty AND use_mocked_data is
// off, short-circuit cleanly otherwise. Cap-1 review SHOULD-FIX #5.
func TestRequireTokenUnlessMocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		token   string
		mocked  bool
		wantErr bool
	}{
		{name: "missing_no_mock_fails", token: "", mocked: false, wantErr: true},
		{name: "missing_with_mock_ok", token: "", mocked: true, wantErr: false},
		{name: "present_no_mock_ok", token: "ghp_abc", mocked: false, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := requireTokenUnlessMocked(&Invocation{
				Token:         config.NewToken(tc.token),
				UseMockedData: tc.mocked,
			})
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			var inputErr *InputError
			if !errors.As(err, &inputErr) {
				t.Fatalf("expected *InputError, got %T: %v", err, err)
			}
			if inputErr.Key != "token" {
				t.Errorf("InputError.Key = %q, want token", inputErr.Key)
			}
		})
	}
}
