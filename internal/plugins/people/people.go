// Package people owns the M4 "people" plugin. It fetches followers /
// following lists via GraphQL. Upstream supports additional types
// (sponsors, contributors, stargazers, watchers, members) — the M4
// MVP wires followers + following only; the remaining types are
// recorded with an empty list so consumers can see the slot, and a
// WARN log fires for unknown types per contract §4.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §4
// Data model: specs/004-m4-github-plugins/data-model.md E-025
package people

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "people"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &peoplePlugin{}

func init() {
	plugins.Register(Plugin)
}

type peoplePlugin struct{}

func (p *peoplePlugin) Name() string                     { return Name }
func (p *peoplePlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["people"].
type Result struct {
	Skipped       bool                `json:"skipped,omitempty"`
	SkippedReason string              `json:"-"`
	Mode          string              `json:"mode,omitempty"`
	Types         map[string][]Person `json:"types"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Person mirrors one entry from the upstream people list.
type Person struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

var knownTypes = map[string]bool{
	"followers":    true,
	"following":    true,
	"sponsors":     true, // recognized but stub-empty in M4
	"contributors": true,
	"stargazers":   true,
	"watchers":     true,
	"members":      true,
}

// Run dispatches per requested type. followers/following hit GraphQL;
// other supported types return empty lists. Unknown types are logged
// and skipped.
func (p *peoplePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if !truthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled", Types: map[string][]Person{}}, nil
	}
	if pc.GraphQL == nil {
		return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable", Types: map[string][]Person{}}, nil
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login", Types: map[string][]Person{}}, nil
	}
	types := readCSVDefault(pc.Inputs, "plugin_people_types", []string{"followers", "following"})
	limit := readIntDefault(pc.Inputs, "plugin_people_limit", 26)
	shuffle := readBool(pc.Inputs, "plugin_people_shuffle")

	out := make(map[string][]Person, len(types))
	needFollowers := false
	needFollowing := false
	for _, t := range types {
		t = strings.ToLower(strings.TrimSpace(t))
		if !knownTypes[t] {
			if pc.Logger != nil {
				pc.Logger.Warn("people: unknown type", slog.String("type", t))
			}
			continue
		}
		if t == "followers" {
			needFollowers = true
		}
		if t == "following" {
			needFollowing = true
		}
		out[t] = []Person{}
	}

	if needFollowers || needFollowing {
		resp, err := pc.GraphQL.UserFollowers(ctx, login, limit)
		if err != nil {
			return nil, xerrors.NewRetryableError(fmt.Errorf("people: %w", err))
		}
		if resp != nil && resp.User != nil {
			if needFollowers && resp.User.Followers != nil {
				out["followers"] = followersToPeople(resp.User.Followers.Nodes)
			}
			if needFollowing && resp.User.Following != nil {
				out["following"] = followingToPeople(resp.User.Following.Nodes)
			}
		}
	}

	if shuffle {
		seed := readIntDefault(pc.Inputs, "plugin_people_shuffle_seed", 1)
		// #nosec G404 -- deterministic shuffle for tests
		r := rand.New(rand.NewSource(int64(seed)))
		for k, list := range out {
			for i := len(list) - 1; i > 0; i-- {
				j := r.Intn(i + 1)
				list[i], list[j] = list[j], list[i]
			}
			out[k] = list
		}
	}

	return &Result{Mode: plugins.AggregationMode(pc.Data), Types: out}, nil
}

func followersToPeople(nodes []*githubapi.UserFollowersUserFollowersUserConnectionNodesUser) []Person {
	out := make([]Person, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		name := ""
		if n.Name != nil {
			name = *n.Name
		}
		out = append(out, Person{Login: n.Login, Name: name, AvatarURL: n.AvatarUrl})
	}
	return out
}

func followingToPeople(nodes []*githubapi.UserFollowersUserFollowingUserConnectionNodesUser) []Person {
	out := make([]Person, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		name := ""
		if n.Name != nil {
			name = *n.Name
		}
		out = append(out, Person{Login: n.Login, Name: name, AvatarURL: n.AvatarUrl})
	}
	return out
}

func truthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

func loginFromInputs(in map[string]any) string {
	if v, ok := in["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := in["login"].(string); ok {
		return v
	}
	return ""
}

func readIntDefault(in map[string]any, key string, def int) int {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return def
		}
		return n
	}
	return def
}

func readBool(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	}
	return false
}

func readCSVDefault(in map[string]any, key string, def []string) []string {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case []string:
		return trimEmpty(x)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, fmt.Sprint(item))
		}
		return trimEmpty(out)
	case string:
		return trimEmpty(strings.Split(x, ","))
	}
	return def
}

func trimEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
