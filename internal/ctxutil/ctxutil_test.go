package ctxutil_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/ctxutil"
)

func TestLoginRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantValue string
		wantOk    bool
	}{
		{name: "set non-empty login", input: "octocat", wantValue: "octocat", wantOk: true},
		{name: "set empty login is still considered set", input: "", wantValue: "", wantOk: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := ctxutil.WithLogin(context.Background(), tc.input)
			got, ok := ctxutil.LoginFromContext(ctx)
			if ok != tc.wantOk {
				t.Fatalf("LoginFromContext ok = %v, want %v", ok, tc.wantOk)
			}
			if got != tc.wantValue {
				t.Fatalf("LoginFromContext value = %q, want %q", got, tc.wantValue)
			}
		})
	}
}

func TestLoginFromBackgroundIsAbsent(t *testing.T) {
	t.Parallel()

	got, ok := ctxutil.LoginFromContext(context.Background())
	if ok {
		t.Fatalf("LoginFromContext on background returned ok=true, value=%q", got)
	}
	if got != "" {
		t.Fatalf("LoginFromContext on background returned value=%q, want empty", got)
	}
}

func TestNestedWithLoginOverrides(t *testing.T) {
	t.Parallel()

	ctx := ctxutil.WithLogin(context.Background(), "first")
	ctx = ctxutil.WithLogin(ctx, "second")
	got, ok := ctxutil.LoginFromContext(ctx)
	if !ok || got != "second" {
		t.Fatalf("nested WithLogin: got (%q, %v), want (%q, true)", got, ok, "second")
	}
}
