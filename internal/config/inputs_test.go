package config_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mjun0812/github-metrics/internal/config"
)

func defWithDefault(typ, format, def string) config.InputDef {
	var node yaml.Node
	if def != "" {
		_ = yaml.Unmarshal([]byte(def), &node)
		if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
			node = *node.Content[0]
		}
	}
	return config.InputDef{
		Type:    typ,
		Format:  config.StringList(strings.FieldsFunc(format, func(r rune) bool { return r == ',' })),
		Default: node,
	}
}

func TestNormalizeInput_TypeRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  config.InputDef
		raw  any
		want any
	}{
		{
			name: "string passthrough",
			def:  defWithDefault("string", "", ""),
			raw:  "hello",
			want: "hello",
		},
		{
			name: "empty string falls back to default",
			def:  defWithDefault("string", "", "default-value"),
			raw:  "",
			want: "default-value",
		},
		{
			name: "boolean yes is true",
			def:  defWithDefault("boolean", "", ""),
			raw:  "yes",
			want: true,
		},
		{
			name: "boolean no is false",
			def:  defWithDefault("boolean", "", ""),
			raw:  "no",
			want: false,
		},
		{
			name: "boolean default no when empty",
			def:  defWithDefault("boolean", "", "no"),
			raw:  "",
			want: false,
		},
		{
			name: "boolean numeric one is true",
			def:  defWithDefault("boolean", "", ""),
			raw:  "1",
			want: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := config.NormalizeInput(tc.def, tc.raw)
			if err != nil {
				t.Fatalf("NormalizeInput: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestNormalizeInput_Number(t *testing.T) {
	t.Parallel()

	def := defWithDefault("number", "", "42")
	got, err := config.NormalizeInput(def, "")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if got.(float64) != 42 {
		t.Fatalf("default number = %v, want 42", got)
	}

	got, err = config.NormalizeInput(def, "9000")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if got.(float64) != 9000 {
		t.Fatalf("override number = %v, want 9000", got)
	}

	if _, err := config.NormalizeInput(def, "not-a-number"); err == nil {
		t.Fatalf("expected parse error for non-numeric string")
	}
}

func TestNormalizeInput_ArrayFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		raw    string
		want   []string
	}{
		{name: "comma-separated single line", format: "comma-separated", raw: "a, b,c , d", want: []string{"a", "b", "c", "d"}},
		{name: "space-separated", format: "space-separated", raw: "a b   c", want: []string{"a", "b", "c"}},
		{name: "newline-separated", format: "newline-separated", raw: "a\nb\n\nc", want: []string{"a", "b", "c"}},
		{name: "combined newline+comma", format: "newline-separated,comma-separated", raw: "a,b\nc,d", want: []string{"a", "b", "c", "d"}},
		{name: "dedup keeps first", format: "comma-separated", raw: "a, b, a, c", want: []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			def := defWithDefault("array", tc.format, "")
			got, err := config.NormalizeInput(def, tc.raw)
			if err != nil {
				t.Fatalf("NormalizeInput: %v", err)
			}
			arr, ok := got.([]string)
			if !ok {
				t.Fatalf("expected []string, got %T", got)
			}
			if !reflect.DeepEqual(arr, tc.want) {
				t.Fatalf("got %v, want %v", arr, tc.want)
			}
		})
	}
}

func TestNormalizeInput_TokenWrappingAndOpacity(t *testing.T) {
	t.Parallel()

	def := defWithDefault("token", "", "")
	got, err := config.NormalizeInput(def, "ghp_secret_value")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	tk, ok := got.(config.Token)
	if !ok {
		t.Fatalf("expected Token, got %T", got)
	}
	if tk.String() != "(provided)" {
		t.Fatalf("Token.String = %q, want (provided)", tk.String())
	}
	if tk.Reveal() != "ghp_secret_value" {
		t.Fatalf("Token.Reveal = %q, want ghp_secret_value", tk.Reveal())
	}

	// Empty token must stay empty.
	empty, err := config.NormalizeInput(def, "")
	if err != nil {
		t.Fatalf("NormalizeInput empty: %v", err)
	}
	if empty.(config.Token).Present() {
		t.Fatalf("empty token should not be Present")
	}
}

func TestNormalizeInput_JSON(t *testing.T) {
	t.Parallel()

	def := defWithDefault("json", "", "")
	got, err := config.NormalizeInput(def, `{"a":1,"b":[2,3]}`)
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage, got %T", got)
	}
	if string(raw) != `{"a":1,"b":[2,3]}` {
		t.Fatalf("RawMessage = %q", raw)
	}

	if _, err := config.NormalizeInput(def, "{invalid"); err == nil {
		t.Fatalf("expected error on malformed JSON")
	}
}

func TestNormalizeInput_UnsupportedTypeErrors(t *testing.T) {
	t.Parallel()

	def := config.InputDef{Type: "unobtainium"}
	if _, err := config.NormalizeInput(def, "x"); err == nil {
		t.Fatalf("expected error for unsupported type")
	}
}

func TestInputs_ForAction_PrecedenceAndPlaceholders(t *testing.T) {
	t.Parallel()

	defs := map[string]config.InputDef{
		"user":                 defWithDefault("string", "", "octocat"),
		"plugin_traffic_token": defWithDefault("token", "", ""),
		"repositories":         defWithDefault("number", "", "10"),
		"plugin_languages_sections": defWithDefault(
			"array", "comma-separated", "most-used",
		),
		"plugin_activity": defWithDefault("boolean", "", "no"),
	}
	i := config.NewInputs(defs)

	env := map[string]string{
		"INPUTS":                          `{"user": "actualuser", "plugin_activity": "yes"}`,
		"INPUT_PLUGIN_TRAFFIC_TOKEN":      "ghs_traffic_secret",
		"INPUT_REPOSITORIES":              "42",
		"INPUT_PLUGIN_LANGUAGES_SECTIONS": "most-used, recently-used",
	}

	q, err := i.ForAction(env, nil)
	if err != nil {
		t.Fatalf("ForAction: %v", err)
	}

	if q["user"] != "actualuser" {
		t.Errorf("INPUTS should override env: got %v", q["user"])
	}
	if q["plugin_activity"] != true {
		t.Errorf("INPUTS json boolean: got %v", q["plugin_activity"])
	}
	if q["repositories"].(float64) != 42 {
		t.Errorf("env number: got %v", q["repositories"])
	}
	tk, ok := q["plugin_traffic_token"].(config.Token)
	if !ok || !tk.Present() {
		t.Errorf("token: got %v", q["plugin_traffic_token"])
	}
	if tk.Reveal() != "ghs_traffic_secret" {
		t.Errorf("token reveal = %q", tk.Reveal())
	}
	got := q["plugin_languages_sections"].([]string)
	want := []string{"most-used", "recently-used"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("array: got %v, want %v", got, want)
	}
}

func TestInputs_ForAction_PresetFallback(t *testing.T) {
	t.Parallel()

	defs := map[string]config.InputDef{
		"template": defWithDefault("string", "", "classic"),
		"output":   defWithDefault("string", "", "svg"),
	}
	i := config.NewInputs(defs)

	preset := map[string]any{"template": "repository", "output": "json"}
	env := map[string]string{} // empty: preset wins over default

	q, err := i.ForAction(env, preset)
	if err != nil {
		t.Fatalf("ForAction: %v", err)
	}
	if q["template"] != "repository" {
		t.Errorf("preset should override default: %v", q["template"])
	}
	if q["output"] != "json" {
		t.Errorf("preset should override default: %v", q["output"])
	}
}

func TestInputs_ForData_PlaceholderResolution(t *testing.T) {
	t.Parallel()

	defs := map[string]config.InputDef{
		"plugin_repositories_pinned": defWithDefault("string", "", ""),
	}
	i := config.NewInputs(defs)

	env := map[string]string{
		"INPUT_PLUGIN_REPOSITORIES_PINNED": "https://github.com/.user.login/showcase",
	}
	q, err := i.ForAction(env, nil)
	if err != nil {
		t.Fatalf("ForAction: %v", err)
	}
	// Placeholder must remain intact before data is wired in.
	if q["plugin_repositories_pinned"] != "https://github.com/.user.login/showcase" {
		t.Fatalf("ForAction should not resolve placeholders: %v", q["plugin_repositories_pinned"])
	}

	resolved := i.ForData(q, &config.DataView{UserLogin: "octocat"}, "user")
	want := "https://github.com/octocat/showcase"
	if resolved["plugin_repositories_pinned"] != want {
		t.Fatalf("ForData = %v, want %v", resolved["plugin_repositories_pinned"], want)
	}
}

func TestInputs_ForData_NilDataLeavesPlaceholdersIntact(t *testing.T) {
	t.Parallel()

	defs := map[string]config.InputDef{
		"label": defWithDefault("string", "", ""),
	}
	i := config.NewInputs(defs)
	env := map[string]string{"INPUT_LABEL": "Hi .user.login"}
	q, _ := i.ForAction(env, nil)

	resolved := i.ForData(q, nil, "user")
	if resolved["label"] != "Hi .user.login" {
		t.Fatalf("nil data should not resolve: got %v", resolved["label"])
	}
}

func TestInputs_ForWeb(t *testing.T) {
	t.Parallel()

	defs := map[string]config.InputDef{
		"user":     defWithDefault("string", "", "octocat"),
		"template": defWithDefault("string", "", "classic"),
	}
	i := config.NewInputs(defs)

	q, err := i.ForWeb(map[string][]string{
		"user": {"actualuser"},
	})
	if err != nil {
		t.Fatalf("ForWeb: %v", err)
	}
	if q["user"] != "actualuser" {
		t.Errorf("Web user = %v", q["user"])
	}
	if q["template"] != "classic" {
		t.Errorf("Web template default = %v", q["template"])
	}
}

func TestInputs_ForAction_RejectsInvalidINPUTS(t *testing.T) {
	t.Parallel()

	i := config.NewInputs(map[string]config.InputDef{})
	env := map[string]string{"INPUTS": "{not-json"}
	if _, err := i.ForAction(env, nil); err == nil {
		t.Fatalf("expected error on malformed INPUTS JSON")
	}
}

func TestNormalizeInput_NumberAcceptsNumericTypes(t *testing.T) {
	t.Parallel()

	def := defWithDefault("number", "", "")
	cases := []struct {
		raw  any
		want float64
	}{
		{raw: float64(3.5), want: 3.5},
		{raw: int(7), want: 7},
		{raw: int64(99), want: 99},
	}
	for _, tc := range cases {
		got, err := config.NormalizeInput(def, tc.raw)
		if err != nil {
			t.Fatalf("NormalizeInput(%v): %v", tc.raw, err)
		}
		if got.(float64) != tc.want {
			t.Fatalf("got %v, want %v", got, tc.want)
		}
	}
	if _, err := config.NormalizeInput(def, struct{}{}); err == nil {
		t.Fatalf("expected error for unsupported number type")
	}
}

func TestNormalizeInput_BooleanCoercesNumericAndBool(t *testing.T) {
	t.Parallel()

	def := defWithDefault("boolean", "", "")
	cases := []struct {
		raw  any
		want bool
	}{
		{raw: true, want: true},
		{raw: false, want: false},
		{raw: 7, want: true},
		{raw: 0, want: false},
		{raw: int64(0), want: false},
		{raw: int64(2), want: true},
		{raw: float64(1.5), want: true},
		{raw: struct{}{}, want: false},
	}
	for _, tc := range cases {
		got, err := config.NormalizeInput(def, tc.raw)
		if err != nil {
			t.Fatalf("NormalizeInput(%v): %v", tc.raw, err)
		}
		if got.(bool) != tc.want {
			t.Fatalf("raw=%v: got %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestNormalizeInput_StringFormattersForNonString(t *testing.T) {
	t.Parallel()

	def := defWithDefault("string", "", "")
	cases := []struct {
		raw  any
		want string
	}{
		{raw: true, want: "true"},
		{raw: false, want: "false"},
		{raw: int(5), want: "5"},
		{raw: int64(99), want: "99"},
		{raw: float64(1.5), want: "1.5"},
	}
	for _, tc := range cases {
		got, err := config.NormalizeInput(def, tc.raw)
		if err != nil {
			t.Fatalf("NormalizeInput: %v", err)
		}
		if got.(string) != tc.want {
			t.Fatalf("raw=%v: got %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestNormalizeInput_ArrayAcceptsSlices(t *testing.T) {
	t.Parallel()

	def := defWithDefault("array", "comma-separated", "")
	// pre-split list passes through (after trim+dedup).
	got, err := config.NormalizeInput(def, []any{"a", " b ", "a"})
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if !reflect.DeepEqual(got.([]string), []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}

	// []string also accepted.
	got, err = config.NormalizeInput(def, []string{"x", "y"})
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if !reflect.DeepEqual(got.([]string), []string{"x", "y"}) {
		t.Fatalf("got %v", got)
	}

	// empty string yields empty slice.
	got, err = config.NormalizeInput(def, "")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if len(got.([]string)) != 0 {
		t.Fatalf("empty array got %v", got)
	}

	// no format -> comma-separated default.
	def2 := defWithDefault("array", "", "")
	got, err = config.NormalizeInput(def2, "a,b")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if !reflect.DeepEqual(got.([]string), []string{"a", "b"}) {
		t.Fatalf("default format got %v", got)
	}
}

func TestNormalizeInput_JSON_RawMessagePassthrough(t *testing.T) {
	t.Parallel()

	def := defWithDefault("json", "", "")
	raw := json.RawMessage(`{"k":1}`)
	got, err := config.NormalizeInput(def, raw)
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if string(got.(json.RawMessage)) != `{"k":1}` {
		t.Fatalf("RawMessage passthrough got %s", got)
	}

	// empty string -> "null".
	got, err = config.NormalizeInput(def, "")
	if err != nil {
		t.Fatalf("NormalizeInput: %v", err)
	}
	if string(got.(json.RawMessage)) != "null" {
		t.Fatalf("empty json = %s", got)
	}
}

func TestMetadataLoader_ToHelpersAreIdentity(t *testing.T) {
	t.Parallel()

	// ToAction / ToWeb / ToQuery currently mirror upstream by returning
	// the key unchanged. Lock the contract so a future renamer cannot
	// silently break upstream compatibility.
	m := &config.MetadataLoader{}
	for _, in := range []string{"user", "plugin_activity", "config.timezone"} {
		if m.ToAction(in) != in {
			t.Errorf("ToAction(%q) = %q, want %q", in, m.ToAction(in), in)
		}
		if m.ToWeb(in) != in {
			t.Errorf("ToWeb(%q) = %q, want %q", in, m.ToWeb(in), in)
		}
		if m.ToQuery(in) != in {
			t.Errorf("ToQuery(%q) = %q, want %q", in, m.ToQuery(in), in)
		}
	}
}
