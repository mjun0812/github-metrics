package config_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
)

func TestToken_StringHidesRaw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "non-empty returns placeholder", raw: "ghp_secret", want: "(provided)"},
		{name: "empty returns empty", raw: "", want: ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tk := config.NewToken(tc.raw)
			if got := tk.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
			if got := tk.GoString(); got != tc.want {
				t.Fatalf("GoString() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToken_RawNeverLeaksThroughFormatters(t *testing.T) {
	t.Parallel()

	raw := "ghp_super_secret_value_42"
	tk := config.NewToken(raw)

	// Run the formatters that gosimple flags ("%s") through a separate
	// variable so the lint rule does not strip the test out from under
	// us — we explicitly *want* to exercise the formatter path.
	stringerVerb := "%s" //nolint:gosimple // intentionally exercising Stringer via fmt
	formatted := []string{
		fmt.Sprintf("%v", tk),
		fmt.Sprintf(stringerVerb, tk),
		fmt.Sprintf("%+v", tk),
		fmt.Sprintf("%#v", tk),
	}
	for i, s := range formatted {
		if strings.Contains(s, raw) {
			t.Fatalf("formatter[%d] leaked raw token: %q", i, s)
		}
	}
}

func TestToken_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "non-empty marshals to placeholder", raw: "ghs_xxx", want: `"(provided)"`},
		{name: "empty marshals to empty string", raw: "", want: `""`},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tk := config.NewToken(tc.raw)
			b, err := json.Marshal(tk)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if got := string(b); got != tc.want {
				t.Fatalf("Marshal = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToken_MarshalJSONInsideStructDoesNotLeak(t *testing.T) {
	t.Parallel()

	type payload struct {
		Token config.Token `json:"token"`
		Login string       `json:"login"`
	}
	p := payload{Token: config.NewToken("ghp_secret_42"), Login: "octocat"}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "ghp_secret_42") {
		t.Fatalf("struct marshal leaked token: %s", b)
	}
	if !strings.Contains(string(b), `"token":"(provided)"`) {
		t.Fatalf("expected provided placeholder, got %s", b)
	}
	if !strings.Contains(string(b), `"login":"octocat"`) {
		t.Fatalf("non-token field missing: %s", b)
	}
}

func TestToken_RevealReturnsRaw(t *testing.T) {
	t.Parallel()

	tk := config.NewToken("ghp_xyz")
	if tk.Reveal() != "ghp_xyz" {
		t.Fatalf("Reveal() = %q, want %q", tk.Reveal(), "ghp_xyz")
	}

	empty := config.NewToken("")
	if empty.Reveal() != "" {
		t.Fatalf("empty.Reveal() = %q, want empty", empty.Reveal())
	}
}

func TestToken_Present(t *testing.T) {
	t.Parallel()

	if !config.NewToken("anything").Present() {
		t.Fatalf("non-empty token should report Present()=true")
	}
	if config.NewToken("").Present() {
		t.Fatalf("empty token should report Present()=false")
	}
	var zero config.Token
	if zero.Present() {
		t.Fatalf("zero-value token should report Present()=false")
	}
}
