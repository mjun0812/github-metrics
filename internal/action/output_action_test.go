package action

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistry_Validate_SupportedAreOK(t *testing.T) {
	t.Parallel()
	r := DefaultRegistry()
	for _, s := range r.Supported {
		if err := r.Validate(s); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", s, err)
		}
	}
}

func TestRegistry_Validate_UnsupportedYieldsConfigError(t *testing.T) {
	t.Parallel()
	r := DefaultRegistry()
	cases := []string{
		"gist",
		"markdown commit",
		"markdown pull-request",
		"markdown gist",
		"gist pull-request",
	}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			err := r.Validate(value)
			if err == nil {
				t.Fatalf("expected error for %q", value)
			}
			var ce *ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("err type = %T, want *ConfigError", err)
			}
			if ce.Key != "output_action" || ce.Value != value {
				t.Errorf("ConfigError(key=%q, value=%q), want output_action / %q", ce.Key, ce.Value, value)
			}
			if !strings.Contains(ce.Msg, "not supported") {
				t.Errorf("message should explain 'not supported'; got %q", ce.Msg)
			}
		})
	}
}

func TestRegistry_Validate_UnknownValueShowsSupported(t *testing.T) {
	t.Parallel()
	r := DefaultRegistry()
	err := r.Validate("foo-bar-baz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Supported:") {
		t.Errorf("unknown-value message should list Supported values; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("Supported list should include 'commit'; got %q", err.Error())
	}
}
