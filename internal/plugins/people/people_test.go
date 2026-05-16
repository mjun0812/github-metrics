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
