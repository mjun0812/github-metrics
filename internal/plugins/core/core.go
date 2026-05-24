// Package core hosts the special "core" plugin. Unlike data-source
// plugins, core is responsible for translating user-supplied
// configuration into the Computed-side fields of plugins.Data, and it
// also owns the parallel runner that drives every other plugin.
package core

import (
	"context"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name returns the canonical plugin name.
const Name = "core"

// Plugin is the singleton exported by this package so that engine code
// (or test setup) can call core.Plugin.Run directly without going
// through the registry.
var Plugin plugins.Plugin = &corePlugin{}

func init() {
	plugins.Register(Plugin)
}

type corePlugin struct{}

func (p *corePlugin) Name() string                     { return Name }
func (p *corePlugin) Metadata() *config.PluginMetadata { return nil } // wired when sync-assets is consumed
func (p *corePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	d := pc.Data
	if d == nil {
		return nil, nil
	}

	cfg := plugins.ComputedConfig{
		Display:    pickString(pc.Inputs, "config_display", ""),
		Animations: pickBool(pc.Inputs, "config_animations", true),
		Base64:     pickBool(pc.Inputs, "config_base64", true),
		DebugFlags: pickStringList(pc.Inputs, "debug_flags"),
	}
	cfg.Timezone = resolveTimezone(pickString(pc.Inputs, "config_timezone", ""))

	d.Config = cfg

	// Zero-init Computed.Repositories so downstream plugins can blindly
	// increment its counters without nil-checking maps.
	if d.Computed.Repositories.Languages == nil {
		d.Computed.Repositories.Languages = map[string]int{}
	}

	return nil, nil
}

// resolveTimezone parses the user-supplied IANA name and returns the
// Computed-side TimezoneConfig. Invalid names fall back to UTC and
// surface the error via TimezoneConfig.Error.
func resolveTimezone(name string) plugins.TimezoneConfig {
	if name == "" {
		return plugins.TimezoneConfig{Name: "UTC"}
	}
	_, err := time.LoadLocation(name)
	if err != nil {
		return plugins.TimezoneConfig{Name: "UTC", Error: err}
	}
	return plugins.TimezoneConfig{Name: name}
}

// --- map readers ------------------------------------------------------

func pickString(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch s := v.(type) {
	case string:
		if s == "" {
			return fallback
		}
		return s
	default:
		return fallback
	}
}

func pickBool(m map[string]any, key string, fallback bool) bool {
	if m == nil {
		return fallback
	}
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(b) {
		case "yes", "true", "1", "on":
			return true
		case "no", "false", "0", "off":
			return false
		default:
			return fallback
		}
	default:
		return fallback
	}
}

func pickStringList(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch list := v.(type) {
	case []string:
		out := make([]string, 0, len(list))
		out = append(out, list...)
		return out
	case []any:
		out := make([]string, 0, len(list))
		for _, x := range list {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
