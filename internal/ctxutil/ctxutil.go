// Package ctxutil provides typed accessors for request-scoped values
// (currently: the GitHub login the engine is operating on) so that
// log records and downstream code can correlate work to a single user
// or organization.
package ctxutil

import "context"

type ctxKey int

const (
	loginKey ctxKey = iota
)

// WithLogin returns a copy of ctx that carries the given GitHub login.
// An empty login is stored as-is; callers that want to clear should not call WithLogin.
func WithLogin(ctx context.Context, login string) context.Context {
	return context.WithValue(ctx, loginKey, login)
}

// LoginFromContext extracts the GitHub login previously set with WithLogin.
// The second return value reports whether the context carried a value.
func LoginFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(loginKey).(string)
	return v, ok
}
