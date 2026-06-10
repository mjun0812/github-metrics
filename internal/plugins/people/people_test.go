package people_test

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/people"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

type fixedMux struct {
	mu   sync.Mutex
	body string
}

func (f *fixedMux) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	body := f.body
	f.mu.Unlock()
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// capturingMux records the GraphQL request variables of the last call
// so tests can assert the `first` / `size` arguments sent upstream.
type capturingMux struct {
	mu    sync.Mutex
	body  string
	vars  map[string]any
	count int
}

func (c *capturingMux) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		var payload struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &payload)
		c.vars = payload.Variables
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Request:    req,
	}, nil
}

func (c *capturingMux) lastVariables() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.vars
}

func newGQL(t *testing.T, body string) *githubapi.GraphQL {
	t.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: &fixedMux{body: body}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return gql
}

const followersBody = `{"data":{"user":{
	"followers":{"totalCount":2,"nodes":[
		{"login":"alice","name":"Alice","avatarUrl":"a"},
		{"login":"bob","name":null,"avatarUrl":"b"}
	]},
	"following":{"totalCount":1,"nodes":[
		{"login":"carol","name":"Carol","avatarUrl":"c"}
	]}
}}}`

func TestRun_DefaultTypes(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_people": true},
		GraphQL: newGQL(t, followersBody),
	}
	out, _ := people.Plugin.Run(context.Background(), pc)
	r := out.(*people.Result)
	if len(r.Types["followers"]) != 2 {
		t.Errorf("followers len = %d, want 2", len(r.Types["followers"]))
	}
	if len(r.Types["following"]) != 1 {
		t.Errorf("following len = %d, want 1", len(r.Types["following"]))
	}
}

// TestRun_DefaultSizeAndLimit pins the upstream metadata.yml defaults
// (plugin_people_size: 28, plugin_people_limit: 24) so a future
// hardcode regression (issue #446) is caught. The limit is asserted via
// the GraphQL `first` argument captured from the request body.
func TestRun_DefaultSizeAndLimit(t *testing.T) {
	t.Parallel()
	cap := &capturingMux{body: followersBody}
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: cap, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_people": true},
		GraphQL: gql,
	}
	out, _ := people.Plugin.Run(context.Background(), pc)
	r := out.(*people.Result)
	if r.Size != 28 {
		t.Errorf("default Size = %d, want 28 (upstream metadata default)", r.Size)
	}
	if got := cap.lastVariables()["first"]; got != float64(24) {
		t.Errorf("GraphQL first = %v, want 24 (upstream limit default)", got)
	}
	if got := cap.lastVariables()["size"]; got != float64(28) {
		t.Errorf("GraphQL size = %v, want 28 (avatarUrl(size:) bound)", got)
	}
}

// TestRun_SizeInputClamped verifies plugin_people_size is honored and
// clamped to the metadata.yml [min 8, max 64] range.
func TestRun_SizeInputClamped(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   any
		want int
	}{
		{"honored", 40, 40},
		{"below min clamps to 8", 2, 8},
		{"above max clamps to 64", 999, 64},
		{"string parsed", "32", 32},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pc := &plugins.PluginContext{
				Data:    plugins.NewData(),
				Inputs:  map[string]any{"user": "octocat", "plugin_people": true, "plugin_people_size": tc.in},
				GraphQL: newGQL(t, followersBody),
			}
			out, _ := people.Plugin.Run(context.Background(), pc)
			r := out.(*people.Result)
			if r.Size != tc.want {
				t.Errorf("Size = %d, want %d", r.Size, tc.want)
			}
		})
	}
}

// TestPartial_HonorsResultSize confirms the avatar <img> width/height
// attributes follow Result.Size rather than a hardcoded value.
func TestPartial_HonorsResultSize(t *testing.T) {
	t.Parallel()
	r := &people.Result{
		Size: 28,
		Types: map[string][]people.Person{
			"followers": {{Login: "alice", Name: "Alice", AvatarURL: "a"}},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(people.Name, r)
	got, err := people.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, `width="28" height="28"`) {
		t.Errorf("partial avatar should be 28x28; got:\n%s", got)
	}
	if strings.Contains(got, `width="64"`) {
		t.Errorf("partial must not hardcode 64px; got:\n%s", got)
	}
}

func TestRun_UnknownTypeIgnored(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_people": true, "plugin_people_types": "followers,bogus"},
		GraphQL: newGQL(t, followersBody),
	}
	out, _ := people.Plugin.Run(context.Background(), pc)
	r := out.(*people.Result)
	if _, ok := r.Types["bogus"]; ok {
		t.Errorf("unknown type should not appear in Types")
	}
}

func TestRun_OtherKnownTypeEmpty(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_people": true, "plugin_people_types": "contributors"},
		GraphQL: newGQL(t, followersBody),
	}
	out, _ := people.Plugin.Run(context.Background(), pc)
	r := out.(*people.Result)
	if contributors, ok := r.Types["contributors"]; !ok || len(contributors) != 0 {
		t.Errorf("contributors should be empty slot; got %+v", r.Types)
	}
}

func TestRun_RepositoryTypesFetchREST(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/contributors", http.StatusOK, `[
		{"login":"alice","avatar_url":"https://avatars.example/alice.png"},
		{"login":"bob","avatar_url":"https://avatars.example/bob.png"}
	]`)
	rest.OnBody("/repos/octocat/hello-world/stargazers", http.StatusOK, `[
		{"login":"carol","avatar_url":"https://avatars.example/carol.png"}
	]`)
	rest.OnBody("/repos/octocat/hello-world/subscribers", http.StatusOK, `[
		{"login":"dave","avatar_url":"https://avatars.example/dave.png"}
	]`)
	d := plugins.NewData()
	d.SetRepo(&plugins.Repo{
		Owner:        "octocat",
		Name:         "hello-world",
		Contributors: 42,
		Stargazers:   100,
		Watchers:     77,
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(d),
		mocks.WithInputs(map[string]any{
			"plugin_people":       true,
			"plugin_people_types": "contributors,stargazers,watchers",
		}),
	)

	out, err := people.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*people.Result)
	if r.Mode != plugins.ModeRepo {
		t.Errorf("Mode = %q, want %q", r.Mode, plugins.ModeRepo)
	}
	for typ, want := range map[string]string{
		"contributors": "alice",
		"stargazers":   "carol",
		"watchers":     "dave",
	} {
		if got := loginAt(r, typ, 0); got != want {
			t.Errorf("%s[0].Login = %q, want %q (all=%+v)", typ, got, want, r.Types[typ])
		}
	}
	for typ, want := range map[string]int{
		"contributors": 42,
		"stargazers":   100,
		"watchers":     77,
	} {
		if got := r.Counts[typ]; got != want {
			t.Errorf("Counts[%s] = %d, want %d", typ, got, want)
		}
	}
	for _, path := range []string{
		"/repos/octocat/hello-world/contributors",
		"/repos/octocat/hello-world/stargazers",
		"/repos/octocat/hello-world/subscribers",
	} {
		if got := rest.Calls(path); got != 1 {
			t.Errorf("REST calls %s = %d, want 1", path, got)
		}
	}
}

func TestRun_ShuffleDeterministic(t *testing.T) {
	t.Parallel()
	in := map[string]any{
		"user":                       "octocat",
		"plugin_people":              true,
		"plugin_people_shuffle":      true,
		"plugin_people_shuffle_seed": 42,
	}
	pc1 := &plugins.PluginContext{Data: plugins.NewData(), Inputs: in, GraphQL: newGQL(t, followersBody)}
	pc2 := &plugins.PluginContext{Data: plugins.NewData(), Inputs: in, GraphQL: newGQL(t, followersBody)}
	out1, _ := people.Plugin.Run(context.Background(), pc1)
	out2, _ := people.Plugin.Run(context.Background(), pc2)
	if loginAt(out1, "followers", 0) != loginAt(out2, "followers", 0) {
		t.Errorf("same seed should yield same shuffle")
	}
}

func TestRun_NilGraphQL_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{"user": "octocat", "plugin_people": true}}
	out, _ := people.Plugin.Run(context.Background(), pc)
	r := out.(*people.Result)
	if !r.Skipped {
		t.Errorf("nil GraphQL should yield Skipped")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &people.Result{Types: map[string][]people.Person{
		"followers": {{Login: "alice", Name: "Alice", AvatarURL: "a"}},
		"following": {{Login: "carol", Name: "Carol", AvatarURL: "c"}},
	}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "people.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}

// TestPartial_RepositoryGolden renders the repository-context people
// partial (contributors / stargazers / watchers) and compares it
// against the golden SVG fragment. The DOM structure mirrors upstream
// docs/original_examples/metrics.plugin.people.repository.svg
// (per-type <section> with <h2 class="field"> header + avatar list);
// concrete avatar URLs / counts differ from upstream by design.
func TestPartial_RepositoryGolden(t *testing.T) {
	r := &people.Result{
		Mode: plugins.ModeRepo,
		Types: map[string][]people.Person{
			"contributors": {
				{Login: "alice", Name: "Alice", AvatarURL: "https://avatars.example/alice.png"},
				{Login: "bob", Name: "Bob", AvatarURL: "https://avatars.example/bob.png"},
			},
			"stargazers": {
				{Login: "carol", Name: "Carol", AvatarURL: "https://avatars.example/carol.png"},
			},
			"watchers": {
				{Login: "dave", Name: "Dave", AvatarURL: "https://avatars.example/dave.png"},
			},
		},
	}
	data := plugins.NewData()
	data.SetRepo(&plugins.Repo{Owner: "octocat", Name: "hello-world"})
	data.SetPlugin(people.Name, r)
	pc := &templates.PartialContext{Data: data}

	got, err := people.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}

	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "people_repository.svg")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", werr)
		}
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	// DOM contract spot-checks mirroring upstream people.repository.svg:
	for _, marker := range []string{
		`data-section="people"`,
		`<h2 class="field">`,
		`data-type="contributors"`,
		`data-type="stargazers"`,
		`data-type="watchers"`,
		`2 contributors`,
		`1 stargazer`,
		`1 watcher`,
		`https://avatars.example/alice.png`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

// TestPartial_HeadingPrefersCountsOverFetchedLen pins #470/#517: the
// per-type section header must show the true total (Result.Counts[type],
// e.g. GraphQL followers.totalCount) rather than the fetched-and-clipped
// slice length. The regression class is reverting to a len(list)-based
// heading once the avatar list is capped below the real count.
func TestPartial_HeadingPrefersCountsOverFetchedLen(t *testing.T) {
	t.Parallel()
	r := &people.Result{
		Counts: map[string]int{"contributors": 42},
		Types: map[string][]people.Person{
			"contributors": {
				{Login: "alice", AvatarURL: "https://avatars.example/alice.png"},
				{Login: "bob", AvatarURL: "https://avatars.example/bob.png"},
			},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(people.Name, r)
	got, err := people.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, ">42 contributors</h2>") {
		t.Fatalf("heading must show true total '42 contributors'; got:\n%s", got)
	}
	if strings.Contains(got, ">2 contributors</h2>") {
		t.Fatalf("heading must not fall back to fetched len '2 contributors'; got:\n%s", got)
	}
}

func loginAt(out any, typ string, idx int) string {
	r, ok := out.(*people.Result)
	if !ok || r == nil {
		return ""
	}
	list := r.Types[typ]
	if idx >= len(list) {
		return ""
	}
	return list[idx].Login
}
