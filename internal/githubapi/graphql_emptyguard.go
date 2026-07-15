package githubapi

import (
	"context"
	"errors"

	"github.com/Khan/genqlient/graphql"
)

// ErrEmptyGraphQLResponse marks a GraphQL exchange in which GitHub
// returned HTTP 200 with an explicit "data": null envelope and no
// errors array. GitHub emits this shape when the request was blocked
// by a secondary rate limit (or the abuse detection heuristic) without
// populating a structured GraphQL error, so the stock genqlient client
// treats the response as a successful empty payload and every downstream
// plugin renders a zero-height card (see #732).
//
// Callers may match with errors.Is; the wrapper in emptyDataGuardClient
// returns *emptyGraphQLResponseError whose Unwrap chains to this
// sentinel.
var ErrEmptyGraphQLResponse = errors.New("github graphql: empty response (data: null with no errors)")

// emptyGraphQLResponseError carries the operation name alongside the
// sentinel so log lines identify which query was swallowed.
type emptyGraphQLResponseError struct {
	OpName string
}

func (e *emptyGraphQLResponseError) Error() string {
	if e.OpName == "" {
		return ErrEmptyGraphQLResponse.Error()
	}
	return ErrEmptyGraphQLResponse.Error() + " (op=" + e.OpName + ")"
}

func (e *emptyGraphQLResponseError) Unwrap() error { return ErrEmptyGraphQLResponse }

// emptyDataGuardClient wraps an inner graphql.Client and rejects
// responses whose JSON envelope decoded to "data": null with no errors.
//
// Rationale (#732): genqlient's default client only returns an error
// when the HTTP status is non-2xx or the "errors" array is non-empty.
// GitHub's secondary rate limit path replies with HTTP 200 and
// `{"data": null}` (sometimes even without an errors array), which
// leaves the caller's pre-allocated response struct zero-valued and no
// error to inspect — the plugin sees "empty user" and emits an empty
// card. Wrapping the client here surfaces the swallowed condition to
// every generated call site without touching genqlient's codegen output.
type emptyDataGuardClient struct {
	inner graphql.Client
}

// newEmptyDataGuardClient wraps inner. inner must be non-nil.
func newEmptyDataGuardClient(inner graphql.Client) graphql.Client {
	return &emptyDataGuardClient{inner: inner}
}

// MakeRequest delegates to the inner client and then inspects
// resp.Data. genqlient's stock client sets Response.Data (typed `any`)
// to nil when the JSON envelope holds `"data": null`; the caller's
// original pointer stays intact because Go passes interface values by
// copy. We restore that pointer and return a wrapped
// ErrEmptyGraphQLResponse so plugins receive a real error instead of a
// silently-zeroed response.
func (g *emptyDataGuardClient) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	original := resp.Data
	if err := g.inner.MakeRequest(ctx, req, resp); err != nil {
		return err
	}
	if resp.Data == nil {
		resp.Data = original
		opName := ""
		if req != nil {
			opName = req.OpName
		}
		return &emptyGraphQLResponseError{OpName: opName}
	}
	return nil
}
