package action

import (
	"errors"
	"strings"
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
		name      string
		token     string
		mocked    bool
		wantErr   bool
		wantInKey string // substring assertion when wantErr
	}{
		{name: "missing_no_mock_fails", token: "", mocked: false, wantErr: true, wantInKey: "token"},
		{name: "missing_with_mock_ok", token: "", mocked: true, wantErr: false},
		{name: "present_no_mock_ok", token: "ghp_abc", mocked: false, wantErr: false},
		{name: "present_with_mock_ok", token: "ghp_abc", mocked: true, wantErr: false},
		{name: "nil_invocation_ok", token: "", mocked: false, wantErr: false}, // covers the inv==nil guard below via special case
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var err error
			if tc.name == "nil_invocation_ok" {
				err = requireTokenUnlessMocked(nil)
			} else {
				inv := &Invocation{
					Token:         config.NewToken(tc.token),
					UseMockedData: tc.mocked,
				}
				err = requireTokenUnlessMocked(inv)
			}

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var inputErr *InputError
				if !errors.As(err, &inputErr) {
					t.Fatalf("expected *InputError, got %T: %v", err, err)
				}
				if inputErr.Key != tc.wantInKey {
					t.Errorf("InputError.Key = %q, want %q", inputErr.Key, tc.wantInKey)
				}
				if !strings.Contains(inputErr.Msg, "GITHUB_TOKEN") {
					t.Errorf("InputError.Msg must reference GITHUB_TOKEN, got %q", inputErr.Msg)
				}
				if !strings.Contains(inputErr.Msg, "use_mocked_data") {
					t.Errorf("InputError.Msg must mention the use_mocked_data escape hatch, got %q", inputErr.Msg)
				}
				if inputErr.Msg != tokenMissingMsg {
					t.Errorf("InputError.Msg drifted from tokenMissingMsg constant")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
