// Package people owns the M4 "people" plugin. User mode fetches
// followers / following lists via GraphQL. Repository mode fetches
// contributors / stargazers / watchers through REST repository
// endpoints so the repository template can reuse the same people
// partial with real avatar data.
package people

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
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

func (p *peoplePlugin) Requires() []plugins.DataKey {
	// people reads from pc.Data fields populated by base; it does not
	// call Provider directly.
	return []plugins.DataKey{}
}

// Result is the JSON payload published under data.Plugins["people"].
//
// Counts carries the true total per type as reported by the API
// (GraphQL UserConnection.totalCount for user-mode followers/following).
// It is distinct from len(Types[t]), which is bounded by the fetched
// page (plugin_people_limit). The header label must read the total
// (#470), while the avatar list still renders the limited slice. Repo
// mode is REST-based with no totalCount concept, so its entries are
// omitted here and the partial falls back to len(list).
type Result struct {
	Skipped       bool                `json:"skipped,omitempty"`
	SkippedReason string              `json:"-"`
	Mode          string              `json:"mode,omitempty"`
	Size          int                 `json:"size,omitempty"`
	Types         map[string][]Person `json:"types"`
	Counts        map[string]int      `json:"counts,omitempty"`
}

// Avatar size bounds mirror assets/plugins/people/metadata.yml
// (plugin_people_size: default 28, min 8, max 64).
const (
	defaultPeopleSize = 28
	minPeopleSize     = 8
	maxPeopleSize     = 64
)

// readPeopleSize resolves plugin_people_size, applying the metadata
// default (28) and clamping to [min 8, max 64].
func readPeopleSize(in map[string]any) int {
	size := pluginutil.ReadIntDefault(in, "plugin_people_size", defaultPeopleSize)
	if size < minPeopleSize {
		size = minPeopleSize
	}
	if size > maxPeopleSize {
		size = maxPeopleSize
	}
	return size
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

// Run dispatches per requested type. User-mode followers/following hit
// GraphQL. Repository-mode contributors/stargazers/watchers hit REST.
// Other supported types return empty lists. Unknown types are logged
// and skipped.
func (p *peoplePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if !pluginutil.TruthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled", Types: map[string][]Person{}}, nil
	}
	repo := pc.Data.RepoRef()
	defaultTypes := []string{"followers", "following"}
	if repo != nil {
		defaultTypes = []string{"stargazers", "watchers"}
	}
	types := readCSVDefault(pc.Inputs, "plugin_people_types", defaultTypes)
	limit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_people_limit", 24)
	size := readPeopleSize(pc.Inputs)
	shuffle := pluginutil.ReadBool(pc.Inputs, "plugin_people_shuffle")

	out := make(map[string][]Person, len(types))
	counts := make(map[string]int)
	needFollowers := false
	needFollowing := false
	repoTypes := []string{}
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
		if repo != nil && isRepositoryPeopleType(t) {
			repoTypes = append(repoTypes, t)
		}
		out[t] = []Person{}
	}

	if needFollowers || needFollowing {
		if pc.GraphQL == nil {
			return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable", Types: map[string][]Person{}}, nil
		}
		login := pluginutil.LoginFromInputs(pc.Inputs)
		if login == "" {
			return &Result{Skipped: true, SkippedReason: "no login", Types: map[string][]Person{}}, nil
		}
		resp, err := pc.GraphQL.UserFollowers(ctx, login, limit, size)
		if err != nil {
			return nil, xerrors.NewRetryableError(fmt.Errorf("people: %w", err))
		}
		if resp != nil && resp.User != nil {
			if needFollowers && resp.User.Followers != nil {
				out["followers"] = followersToPeople(resp.User.Followers.Nodes)
				counts["followers"] = resp.User.Followers.TotalCount
			}
			if needFollowing && resp.User.Following != nil {
				out["following"] = followingToPeople(resp.User.Following.Nodes)
				counts["following"] = resp.User.Following.TotalCount
			}
		}
	}

	if len(repoTypes) > 0 {
		if pc.REST == nil {
			return &Result{Skipped: true, SkippedReason: "REST client unavailable", Types: map[string][]Person{}}, nil
		}
		counts["contributors"] = repo.Contributors
		counts["stargazers"] = repo.Stargazers
		counts["watchers"] = repo.Watchers
		for _, typ := range repoTypes {
			people, err := fetchRepositoryPeople(ctx, pc.REST, repo.Owner, repo.Name, typ, limit)
			if err != nil {
				return nil, xerrors.NewRetryableError(err)
			}
			out[typ] = people
		}
	}

	if shuffle {
		seed := pluginutil.ReadIntDefault(pc.Inputs, "plugin_people_shuffle_seed", 1)
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

	return &Result{Mode: plugins.AggregationMode(pc.Data), Size: size, Types: out, Counts: counts}, nil
}

func isRepositoryPeopleType(t string) bool {
	return t == "contributors" || t == "stargazers" || t == "watchers"
}

func fetchRepositoryPeople(ctx context.Context, rest *githubapi.REST, owner, repo, typ string, limit int) ([]Person, error) {
	endpoint, ok := map[string]string{
		"contributors": "contributors",
		"stargazers":   "stargazers",
		"watchers":     "subscribers",
	}[typ]
	if !ok {
		return nil, nil
	}
	perPage := limit
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}
	path := fmt.Sprintf(
		"/repos/%s/%s/%s?per_page=%d",
		url.PathEscape(owner),
		url.PathEscape(repo),
		endpoint,
		perPage,
	)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("people: repository %s: %w", typ, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("people: repository %s: nil response", typ)
	}
	if resp.StatusCode == http.StatusNotFound {
		return []Person{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("people: repository %s: status %d: %s", typ, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var nodes []struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("people: repository %s: decode: %w", typ, err)
	}
	out := make([]Person, 0, len(nodes))
	for _, n := range nodes {
		if n.Login == "" {
			continue
		}
		out = append(out, Person{Login: n.Login, Name: n.Name, AvatarURL: n.AvatarURL})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
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

func readCSVDefault(in map[string]any, key string, def []string) []string {
	if _, ok := in[key]; !ok {
		return def
	}
	return pluginutil.ReadCSV(in, key)
}
