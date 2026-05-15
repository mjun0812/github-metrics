package githubapi_test

import (
	"context"
	"encoding/json"
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
