package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Settings mirrors the upstream lowlighter/metrics settings.json schema
// (data-model E-001). The Web-only sub-structs (OAuth, Control, Hosted,
// Community, Web) are retained so the key set stays upstream-compatible
// (constitution principle I) even though M1 does not implement the Web
// instance.
type Settings struct {
	Token          string                    `json:"token"`
	Modes          []string                  `json:"modes"`
	Restricted     []string                  `json:"restricted"`
	MaxUsers       int                       `json:"maxusers"`
	Cached         int                       `json:"cached"`
	Ratelimiter    *Ratelimiter              `json:"ratelimiter"`
	Port           int                       `json:"port"`
	Optimize       OptimizeFlag              `json:"optimize"`
	Debug          bool                      `json:"debug"`
	DebugHeadless  bool                      `json:"debug.headless"`
	Mocked         MockFlag                  `json:"mocked"`
	Repositories   int                       `json:"repositories"`
	Padding        []string                  `json:"padding"`
	Outputs        []string                  `json:"outputs"`
	Hosted         Hosted                    `json:"hosted"`
	OAuth          *OAuth                    `json:"oauth"`
	API            APISettings               `json:"api"`
	Control        Control                   `json:"control"`
	Community      CommunitySettings         `json:"community"`
	Templates      TemplatesSettings         `json:"templates"`
	Extras         Extras                    `json:"extras"`
	PluginsDefault bool                      `json:"plugins.default"`
	Plugins        map[string]PluginSettings `json:"plugins"`
	Sandbox        bool                      `json:"sandbox"`
	Web            *WebSettings              `json:"web,omitempty"`
}

// Ratelimiter mirrors the upstream express-rate-limit fields the project consumes.
type Ratelimiter struct {
	Max int `json:"max"`
	// Window keeps the upstream "windowMs" JSON tag (constitution
	// principle I) but uses an idiomatic Go field name without the
	// unit-specific suffix that staticcheck ST1011 flags.
	Window time.Duration `json:"windowMs"`
}

// OptimizeFlag accepts either a bool or a list of optimization passes
// (e.g. ["css", "xml", "svg"]). The custom unmarshaller normalizes both
// shapes into the same struct so callers can branch with one check.
type OptimizeFlag struct {
	Enabled bool
	Passes  []string
}

// UnmarshalJSON parses bool or []string per upstream conventions.
func (f *OptimizeFlag) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var asBool bool
	if err := json.Unmarshal(b, &asBool); err == nil {
		f.Enabled = asBool
		return nil
	}
	var asList []string
	if err := json.Unmarshal(b, &asList); err == nil {
		f.Passes = asList
		f.Enabled = len(asList) > 0
		return nil
	}
	return fmt.Errorf("optimize: unsupported value %s", string(b))
}

// MarshalJSON keeps round-trip safety for the more expressive form.
func (f OptimizeFlag) MarshalJSON() ([]byte, error) {
	if len(f.Passes) > 0 {
		return json.Marshal(f.Passes)
	}
	return json.Marshal(f.Enabled)
}

// MockFlag accepts either a bool or the literal "force" string per upstream.
type MockFlag struct {
	Enabled bool
	Force   bool
}

// UnmarshalJSON parses bool or "force".
func (m *MockFlag) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var asBool bool
	if err := json.Unmarshal(b, &asBool); err == nil {
		m.Enabled = asBool
		return nil
	}
	var asStr string
	if err := json.Unmarshal(b, &asStr); err == nil {
		if asStr == "force" {
			m.Enabled = true
			m.Force = true
			return nil
		}
	}
	return fmt.Errorf("mocked: unsupported value %s", string(b))
}

// MarshalJSON mirrors UnmarshalJSON shape.
func (m MockFlag) MarshalJSON() ([]byte, error) {
	if m.Force {
		return json.Marshal("force")
	}
	return json.Marshal(m.Enabled)
}

// Hosted, OAuth, APISettings, Control, CommunitySettings,
// TemplatesSettings, Extras, PluginSettings, WebSettings retain
// upstream-compatible keys. They are intentionally simple structs so
// adding a new key is a one-line change.

// Hosted records the optional "hosted by" attribution displayed in the
// upstream web instance footer.
type Hosted struct {
	By   string `json:"by"`
	Link string `json:"link"`
}

// OAuth carries upstream OAuth client credentials. Unused by M1 (Web
// is not adopted) but the keys are retained per constitution principle I.
type OAuth struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// APISettings overrides the default GitHub REST and GraphQL endpoints.
type APISettings struct {
	REST    string `json:"rest"`
	GraphQL string `json:"graphql"`
}

// Control holds the upstream control-plane token (Web feature).
type Control struct {
	Token string `json:"token"`
}

// CommunitySettings enumerates the community-template names a Web
// instance is willing to download. Unused by M1.
type CommunitySettings struct {
	Templates []string `json:"templates"`
}

// TemplatesSettings configures which named templates are enabled and
// which one is the engine's default.
type TemplatesSettings struct {
	Default string   `json:"default"`
	Enabled []string `json:"enabled"`
}

// Extras retains the upstream JSON shape (features can be bool or []string).
type Extras struct {
	Default  bool            `json:"default"`
	Features json.RawMessage `json:"features"`
	Logged   []string        `json:"logged"`
}

// PluginSettings carries per-plugin overrides. Extra fields are
// captured verbatim so plugin-specific keys (tokens, API endpoints,
// etc.) round-trip unchanged.
type PluginSettings struct {
	Enabled bool           `json:"enabled"`
	Extra   map[string]any `json:"-"`
}

// UnmarshalJSON splits the canonical "enabled" key from everything else.
func (p *PluginSettings) UnmarshalJSON(b []byte) error {
	raw := map[string]any{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["enabled"]; ok {
		if bv, ok := v.(bool); ok {
			p.Enabled = bv
		}
		delete(raw, "enabled")
	}
	p.Extra = raw
	return nil
}

// WebSettings is reserved for the Web instance (not implemented in M1).
type WebSettings struct {
	Static string `json:"static"`
}

// defaultPort matches the upstream behavior when settings.json is absent.
const defaultPort = 3000

// LoadSettings reads settings.json from path (creating defaults when
// absent), strips // comment keys, applies Sandbox enforcement when
// requested, and returns the resulting Settings.
//
// Absent path returns Settings{Port: defaultPort}.
func LoadSettings(path string) (*Settings, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // explicit user-controlled path
	switch {
	case err == nil:
		// fallthrough into the parse below
	case os.IsNotExist(err):
		return &Settings{Port: defaultPort}, nil
	default:
		return nil, fmt.Errorf("settings: read %s: %w", path, err)
	}

	return parseSettings(raw)
}

// LoadSettingsReader is the io.Reader variant of LoadSettings, mainly
// used by tests.
func LoadSettingsReader(r io.Reader) (*Settings, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("settings: read: %w", err)
	}
	return parseSettings(buf)
}

func parseSettings(raw []byte) (*Settings, error) {
	stripped, err := stripCommentKeys(raw)
	if err != nil {
		return nil, fmt.Errorf("settings: strip comments: %w", err)
	}

	s := &Settings{}
	if err := json.Unmarshal(stripped, s); err != nil {
		return nil, fmt.Errorf("settings: unmarshal: %w", err)
	}
	if s.Port == 0 {
		s.Port = defaultPort
	}
	if s.Sandbox {
		applySandbox(s)
	}
	return s, nil
}

// stripCommentKeys removes every map entry whose key is exactly "//" or
// starts with "//" (e.g. "//2" used by upstream when more than one
// sibling comment is needed). The walk is recursive to handle nested
// objects and arrays of objects. Duplicate "//" keys in the source
// collapse to the last value during json.Unmarshal — that is acceptable
// because we discard all of them anyway.
func stripCommentKeys(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(walkStrip(generic))
}

func walkStrip(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if isCommentKey(k) {
				continue
			}
			out[k] = walkStrip(val)
		}
		return out
	case []any:
		for i, item := range x {
			x[i] = walkStrip(item)
		}
		return x
	default:
		return v
	}
}

func isCommentKey(k string) bool {
	return strings.HasPrefix(k, "//")
}

// applySandbox forces the upstream-defined sandbox overrides:
//
//	optimize=true, cached=0, plugins.default=true,
//	extras.default=true, mocked=true.
//
// We do not touch unrelated fields so a developer can still customize
// e.g. port or token while in sandbox.
func applySandbox(s *Settings) {
	s.Optimize.Enabled = true
	s.Cached = 0
	s.PluginsDefault = true
	s.Extras.Default = true
	s.Mocked.Enabled = true
}

// NoToken reports whether the explicit "NOT_NEEDED" sentinel is set.
// The bare empty string is intentionally treated as "absent" rather than
// "no token" to keep parity with upstream behavior.
func (s *Settings) NoToken() bool {
	return s.Token == "NOT_NEEDED"
}
