package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Marshal serializes the populated Data structure into the upstream-
// compatible JSON shape declared in
// specs/002-output-classic-json/contracts/json-output.md.
//
// The wire format mirrors upstream lowlighter/metrics output keys
// (account, user, config, computed, plugins, errors) while normalizing
// Go-only details (time.Time → RFC 3339, Token → "(provided)" mask,
// maps with non-string keys → [{key,value}] arrays, sets → sorted
// slices). Cycles in Data.Plugins[*] are replaced with the literal
// string "[Circular]" so the call never panics on pathological inputs.
func Marshal(data *plugins.Data) ([]byte, error) {
	out := buildEnvelope(data)
	return json.Marshal(out)
}

// buildEnvelope assembles the top-level JSON shape from Data. Each
// branch normalizes its value via cycleDetector.normalize so the
// resulting map can be marshalled by encoding/json without further
// preprocessing.
func buildEnvelope(data *plugins.Data) map[string]any {
	envelope := map[string]any{
		"account": "",
		"user":    nil,
		"config":  map[string]any{},
		"computed": map[string]any{
			"commits": 0,
			"repositories": map[string]any{
				"count":        0,
				"stargazers":   0,
				"forks":        0,
				"releases":     0,
				"watchers":     0,
				"issues":       0,
				"pullRequests": 0,
				"languages":    map[string]any{},
			},
		},
		"plugins": map[string]any{},
		"errors":  []map[string]string{},
	}
	if data == nil {
		return envelope
	}

	cd := newCycleDetector()
	envelope["account"] = string(data.Account)
	envelope["user"] = userToMap(data.User)
	if r := data.RepoRef(); r != nil {
		// M7: emit `data.repo` only when the repository template
		// populated it. Classic-template runs omit the field entirely
		// (golden classic shape stays unchanged).
		envelope["repo"] = repoToMap(r)
	}
	envelope["config"] = configToMap(data.Config)
	envelope["computed"] = computedToMap(data.Computed)

	plugins := map[string]any{}
	for name, raw := range data.Plugins {
		plugins[name] = cd.normalize(raw)
	}
	envelope["plugins"] = plugins

	errs := make([]map[string]string, 0, len(data.Errors)+1)
	for _, e := range data.Errors {
		if e == nil {
			continue
		}
		errs = append(errs, map[string]string{"error": e.Error()})
	}
	if data.Config.Timezone.Error != nil {
		errs = append(errs, map[string]string{"error": data.Config.Timezone.Error.Error()})
	}
	envelope["errors"] = errs
	return envelope
}

func userToMap(u *plugins.User) any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"login":     u.Login,
		"name":      u.Name,
		"avatarUrl": u.AvatarURL,
	}
}

// repoToMap renders the M7 `data.repo` envelope field. Snake_case
// keys match the upstream `template.mjs:14-17` convention so JSON
// consumers porting from lowlighter/metrics see the same shape.
func repoToMap(r *plugins.Repo) map[string]any {
	if r == nil {
		return nil
	}
	out := map[string]any{
		"owner":                      r.Owner,
		"owner_avatar":               r.OwnerAvatar,
		"name":                       r.Name,
		"name_with_owner":            r.Owner + "/" + r.Name,
		"description":                r.Description,
		"stargazers":                 r.Stargazers,
		"forks":                      r.Forks,
		"contributors":               r.Contributors,
		"is_archived":                r.IsArchived,
		"default_branch":             r.DefaultBranch,
		"license_name":               r.LicenseName,
		"sponsorships_as_maintainer": r.SponsorshipsAsMaintainer,
		"activity": map[string]any{
			"recent_commits":     r.Activity.RecentCommits,
			"open_issues":        r.Activity.OpenIssues,
			"open_pull_requests": r.Activity.OpenPullRequests,
		},
	}
	if r.PrimaryLanguage != "" {
		out["primary_language"] = map[string]any{
			"name":  r.PrimaryLanguage,
			"color": r.PrimaryLanguageColor,
		}
	}
	return out
}

func configToMap(c plugins.ComputedConfig) map[string]any {
	tz := map[string]any{
		"name":  c.Timezone.Name,
		"error": errString(c.Timezone.Error),
	}
	return map[string]any{
		"timezone":   tz,
		"animations": c.Animations,
		"display":    c.Display,
		"base64":     c.Base64,
		"debugFlags": c.DebugFlags,
	}
}

func computedToMap(c plugins.Computed) map[string]any {
	r := c.Repositories
	return map[string]any{
		"commits": c.Commits,
		"repositories": map[string]any{
			"count":        r.Count,
			"stargazers":   r.Stargazers,
			"forks":        r.Forks,
			"releases":     r.Releases,
			"watchers":     r.Watchers,
			"issues":       r.Issues,
			"pullRequests": r.PullRequests,
			"languages":    r.Languages,
		},
	}
}

func errString(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

// cycleDetector walks an arbitrary value tree, breaking pointer/map/
// slice cycles by replacing the second visit of a node with the
// literal string "[Circular]". Visited keys are reflect.Value.Pointer
// values; unaddressable composite values short-circuit conservatively
// to the same sentinel.
type cycleDetector struct {
	seen map[uintptr]struct{}
}

func newCycleDetector() *cycleDetector {
	return &cycleDetector{seen: map[uintptr]struct{}{}}
}

// circular is the sentinel value emitted when a cycle is detected.
const circular = "[Circular]"

func (c *cycleDetector) normalize(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64:
		return x
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	case error:
		return map[string]string{"error": x.Error()}
	case config.Token:
		return x.String()
	}
	rv := reflect.ValueOf(v)
	return c.normalizeReflect(rv)
}

func (c *cycleDetector) normalizeReflect(rv reflect.Value) any {
	if !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		ptr := rv.Pointer()
		if _, seen := c.seen[ptr]; seen {
			return circular
		}
		c.seen[ptr] = struct{}{}
		defer delete(c.seen, ptr)
		return c.normalizeReflect(rv.Elem())
	case reflect.Map:
		return c.normalizeMap(rv)
	case reflect.Slice, reflect.Array:
		return c.normalizeSlice(rv)
	case reflect.Struct:
		return c.normalizeStruct(rv)
	case reflect.String:
		return rv.String()
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint()
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	default:
		return fmt.Sprintf("%v", rv.Interface())
	}
}

func (c *cycleDetector) normalizeMap(rv reflect.Value) any {
	if rv.IsNil() {
		return nil
	}
	ptr := rv.Pointer()
	if _, seen := c.seen[ptr]; seen {
		return circular
	}
	c.seen[ptr] = struct{}{}
	defer delete(c.seen, ptr)

	// String-keyed maps stay as JSON objects. Other key types collapse
	// to []{key,value} arrays sorted by key for determinism.
	if rv.Type().Key().Kind() == reflect.String {
		out := map[string]any{}
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = c.normalize(iter.Value().Interface())
		}
		return out
	}

	type kv struct {
		Key   any `json:"key"`
		Value any `json:"value"`
	}
	pairs := make([]kv, 0, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		pairs = append(pairs, kv{
			Key:   c.normalize(iter.Key().Interface()),
			Value: c.normalize(iter.Value().Interface()),
		})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return fmt.Sprintf("%v", pairs[i].Key) < fmt.Sprintf("%v", pairs[j].Key)
	})
	return pairs
}

func (c *cycleDetector) normalizeSlice(rv reflect.Value) any {
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return nil
	}
	if rv.Kind() == reflect.Slice {
		ptr := rv.Pointer()
		if _, seen := c.seen[ptr]; seen {
			return circular
		}
		c.seen[ptr] = struct{}{}
		defer delete(c.seen, ptr)
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = c.normalize(rv.Index(i).Interface())
	}
	return out
}

func (c *cycleDetector) normalizeStruct(rv reflect.Value) any {
	t := rv.Type()
	switch v := rv.Interface().(type) {
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	case config.Token:
		return v.String()
	}
	out := map[string]any{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := jsonName(f)
		if name == "-" {
			continue
		}
		val := rv.Field(i)
		out[name] = c.normalize(val.Interface())
	}
	return out
}

// jsonName resolves the JSON-side name for a struct field. We honor
// the `json:"name"` tag when present, otherwise default to a
// lowerCamelCase rendering of the Go field name.
func jsonName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" {
		// strip ",omitempty" etc.
		for i := 0; i < len(tag); i++ {
			if tag[i] == ',' {
				return tag[:i]
			}
		}
		return tag
	}
	return lowerFirst(f.Name)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] - 'A' + 'a'
	}
	return string(r)
}
