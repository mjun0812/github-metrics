package githubapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// graphqlMockTransport implements http.RoundTripper and serves a
// canned JSON payload. The genqlient client itself POSTs JSON to a
// single endpoint, so route by Method+URL.Path.
type graphqlMockTransport struct {
	body         string
	statusCode   int
	captured     *http.Request
	capturedBody []byte
}

func (g *graphqlMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	g.captured = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		g.capturedBody = b
	}
	status := g.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Body:          io.NopCloser(strings.NewReader(g.body)),
		Header:        h,
		Request:       req,
		ContentLength: int64(len(g.body)),
	}, nil
}

func newGraphQLWithMock(t *testing.T, transport *graphqlMockTransport) *githubapi.GraphQL {
	t.Helper()
	g, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:  transport,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return g
}

func TestGraphQL_User_DecodesPayload(t *testing.T) {
	t.Parallel()

	// Real-shape genqlient response with the User type fields. CreatedAt
	// is parsed via the time.Time binding we configured in genqlient.yaml.
	transport := &graphqlMockTransport{
		body: `{
			"data": {
				"user": {
					"databaseId": 12345,
					"id": "MDQ6VXNlcjEyMzQ1",
					"login": "octocat",
					"name": "The Octocat",
					"location": "San Francisco",
					"createdAt": "2008-01-14T04:33:35Z",
					"avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4",
					"websiteUrl": "https://github.com",
					"twitterUsername": null,
					"email": "",
					"bio": "Mascot",
					"company": "@github"
				}
			}
		}`,
	}
	g := newGraphQLWithMock(t, transport)

	resp, err := g.User(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if resp.User == nil {
		t.Fatalf("response.User is nil")
	}
	if resp.User.Login != "octocat" {
		t.Errorf("login = %q, want octocat", resp.User.Login)
	}
	if resp.User.DatabaseId == nil || *resp.User.DatabaseId != 12345 {
		t.Errorf("databaseId = %v, want 12345", resp.User.DatabaseId)
	}
	if resp.User.CreatedAt.Year() != 2008 {
		t.Errorf("createdAt = %v, want 2008", resp.User.CreatedAt)
	}
	if resp.User.Bio == nil || *resp.User.Bio != "Mascot" {
		t.Errorf("bio = %v, want Mascot", resp.User.Bio)
	}
}

func TestGraphQL_User_SendsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":{"user":null}}`}
	g := newGraphQLWithMock(t, transport)
	_, _ = g.User(context.Background(), "octocat")

	if transport.captured == nil {
		t.Fatalf("transport saw no request")
	}
	auth := transport.captured.Header.Get("Authorization")
	if auth != "bearer ghp_test" {
		t.Errorf("Authorization = %q, want %q", auth, "bearer ghp_test")
	}
	if got := transport.captured.Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Errorf("Accept = %q", got)
	}
}

// TestGraphQL_User_SendsUserAgent guards against regressing to the Go
// default User-Agent ("Go-http-client/1.1"), which GitHub rejects as
// bot/abuse traffic with a 403 (empty {"data":null} body). genqlient's
// transport sends no UA on its own, so graphqlAuthTransport must set it.
func TestGraphQL_User_SendsUserAgent(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":{"user":null}}`}
	g := newGraphQLWithMock(t, transport)
	_, _ = g.User(context.Background(), "octocat")

	if transport.captured == nil {
		t.Fatalf("transport saw no request")
	}
	if got := transport.captured.Header.Get("User-Agent"); got != httpx.DefaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", got, httpx.DefaultUserAgent)
	}
}

func TestGraphQL_User_PassesVariables(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":{"user":null}}`}
	g := newGraphQLWithMock(t, transport)
	_, _ = g.User(context.Background(), "linus")

	var payload struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(transport.capturedBody, &payload); err != nil {
		t.Fatalf("decode captured body: %v\nraw=%s", err, string(transport.capturedBody))
	}
	if payload.Variables["login"] != "linus" {
		t.Fatalf("login variable = %v, want linus", payload.Variables["login"])
	}
	if !strings.Contains(payload.Query, "user(login: $login)") {
		t.Fatalf("query missing parametrized user(login: $login): %s", payload.Query)
	}
}

// TestGraphQL_UserReactions_PaginatesReactionConnection guards against
// the #472 regression: the reactions connection on issues / issue
// comments must carry a `first`/`last` pagination argument. GitHub's
// GraphQL API rejects a bare `reactions { nodes { content } }` selection
// ("You must provide a `first` or `last` value to properly paginate the
// `reactions` connection."), which made UserReactions error at runtime,
// stored the error in the plugin slot, and collapsed the reactions card
// to an empty <foreignObject> body. The local trimmed schema declared
// the field without arguments, masking the requirement, so this asserts
// the emitted operation string keeps the pagination arg.
func TestGraphQL_UserReactions_PaginatesReactionConnection(t *testing.T) {
	t.Parallel()

	op := githubapi.UserReactions_Operation
	if strings.Contains(op, "reactions {") {
		t.Fatalf("UserReactions operation selects reactions without a pagination arg; "+
			"GitHub requires first/last on a connection nodes selection:\n%s", op)
	}
	if n := strings.Count(op, "reactions(last: 100) {"); n != 2 {
		t.Fatalf("UserReactions operation should paginate both reaction connections "+
			"(issues + issueComments) with last: 100, got %d:\n%s", n, op)
	}
}

func TestGraphQL_Organization_DecodesPayload(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{
		body: `{
			"data": {
				"organization": {
					"databaseId": 99,
					"id": "MDEyOk9yZ2FuaXphdGlvbjk5",
					"login": "github",
					"name": "GitHub",
					"location": null,
					"createdAt": "2008-04-10T00:00:00Z",
					"avatarUrl": "https://avatars.githubusercontent.com/u/99?v=4",
					"websiteUrl": null,
					"email": null,
					"description": "How people build software."
				}
			}
		}`,
	}
	g := newGraphQLWithMock(t, transport)

	resp, err := g.Organization(context.Background(), "github")
	if err != nil {
		t.Fatalf("Organization: %v", err)
	}
	if resp.Organization == nil {
		t.Fatalf("response.Organization is nil")
	}
	if resp.Organization.Login != "github" {
		t.Errorf("login = %q", resp.Organization.Login)
	}
	if resp.Organization.Description == nil || !strings.Contains(*resp.Organization.Description, "people build software") {
		t.Errorf("description = %v", resp.Organization.Description)
	}
}

func TestGraphQL_NotNeededTokenOmitsAuth(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":{"user":null}}`}
	g, err := githubapi.NewGraphQL(config.NewToken("NOT_NEEDED"), "", httpx.Options{Transport: transport, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	_, _ = g.User(context.Background(), "octocat")

	if got := transport.captured.Header.Get("Authorization"); got != "" {
		t.Errorf("NOT_NEEDED should not send Authorization, got %q", got)
	}
}

func TestGraphQL_FineGrainedTokenRejected(t *testing.T) {
	t.Parallel()

	_, err := githubapi.NewGraphQL(config.NewToken("github_pat_xxx"), "", httpx.Options{})
	if err == nil {
		t.Fatalf("expected fine-grained token rejection")
	}
}

func TestGraphQL_MockedTokenPanicsOnRealHost(t *testing.T) {
	t.Parallel()

	g, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"https://api.github.com/graphql",
		httpx.Options{MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for MOCKED_TOKEN hitting real GraphQL host")
		}
	}()
	_, _ = g.User(context.Background(), "octocat")
}

func TestGraphQL_ServerErrorPropagates(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{
		statusCode: http.StatusBadRequest,
		body:       `{"errors":[{"message":"Field user does not exist"}]}`,
	}
	g := newGraphQLWithMock(t, transport)

	if _, err := g.User(context.Background(), "noone"); err == nil {
		t.Fatalf("expected error for 400 response")
	}
}

// TestGraphQL_EmptyDataNullReturnsError guards #732: GitHub's secondary
// rate limit path can reply with HTTP 200 and `{"data": null}` and no
// errors envelope. The stock genqlient client treats that as a
// successful zero-valued response, which downstream renders as an empty
// card. emptyDataGuardClient must surface the swallowed condition as
// ErrEmptyGraphQLResponse.
func TestGraphQL_EmptyDataNullReturnsError(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":null}`}
	g := newGraphQLWithMock(t, transport)

	_, err := g.User(context.Background(), "octocat")
	if err == nil {
		t.Fatalf("expected ErrEmptyGraphQLResponse for {\"data\":null}, got nil")
	}
	if !errors.Is(err, githubapi.ErrEmptyGraphQLResponse) {
		t.Fatalf("errors.Is(ErrEmptyGraphQLResponse) = false; err=%v", err)
	}
	// The wrapper should mention the operation name so log lines
	// pinpoint which query was swallowed.
	if !strings.Contains(err.Error(), "User") {
		t.Errorf("error message missing op name; err=%q", err.Error())
	}
}

// TestGraphQL_EmptyDataWithErrorsSurfacesGraphQLErrors verifies that
// when GitHub responds with `{"data": null, "errors": [...]}` (the
// typical GraphQL error envelope, including secondary rate limits that
// do populate a structured error), genqlient's default handling wins
// and the wrapper does not swallow the more informative message.
func TestGraphQL_EmptyDataWithErrorsSurfacesGraphQLErrors(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{
		body: `{"data":null,"errors":[{"message":"API rate limit exceeded","type":"RATE_LIMITED"}]}`,
	}
	g := newGraphQLWithMock(t, transport)

	_, err := g.User(context.Background(), "octocat")
	if err == nil {
		t.Fatalf("expected error for rate-limited response")
	}
	// The genqlient errors path must win over the empty-data guard so
	// the operator sees the GitHub message (and not a generic sentinel).
	if !strings.Contains(err.Error(), "API rate limit exceeded") {
		t.Errorf("error should surface GitHub message; err=%q", err.Error())
	}
	if errors.Is(err, githubapi.ErrEmptyGraphQLResponse) {
		t.Errorf("guard should not wrap when genqlient already returns errors; err=%v", err)
	}
}

// TestGraphQL_ExplicitDataUserNullIsNotAnError guards the legitimate
// "user does not exist" GraphQL shape (`{"data": {"user": null}}`).
// The Data envelope is a non-null object, so emptyDataGuardClient must
// keep its hands off and let the caller observe the null user field.
func TestGraphQL_ExplicitDataUserNullIsNotAnError(t *testing.T) {
	t.Parallel()

	transport := &graphqlMockTransport{body: `{"data":{"user":null}}`}
	g := newGraphQLWithMock(t, transport)

	resp, err := g.User(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error for {\"data\":{\"user\":null}}: %v", err)
	}
	if resp == nil {
		t.Fatalf("resp should be non-nil (envelope decoded successfully)")
	}
	if resp.User != nil {
		t.Errorf("resp.User should be nil, got %+v", resp.User)
	}
}
