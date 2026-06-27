// Package templates defines the Template contract used by the engine
// to render a request into one of the supported output formats. The
// concrete templates (classic, repository) implement this interface
// from their own packages; the registry below is how they advertise
// themselves to the engine.
package templates

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"sync"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Template is the contract every renderer implements.
type Template interface {
	Name() string
	Metadata() *config.TemplateMetadata
	FS() fs.FS
	Check(q map[string]any, account, format string) error
	Run(ctx context.Context, pc *PartialContext) (string, error)
}

// PartialFunc is the signature each partial (header, activity,
// repositories, ...) implements. Functions are meant to be idempotent.
type PartialFunc func(ctx context.Context, pc *PartialContext) (string, error)

// PartialContext carries the data a partial needs to render its slice
// of the SVG/Markdown output. Helpers (octicon/twemoji/gemoji) land in
// M3; the field is nil-safe for M1 callers.
type PartialContext struct {
	Settings *config.Settings
	Inputs   map[string]any
	Logger   *slog.Logger
	Data     *plugins.Data
	Metadata *config.MetadataLoader
	// Provider is the lazy + memoized dataprovider (#603) the engine
	// constructs per request. Partials read the resolved user /
	// organization / repository via Provider rather than the legacy
	// pc.Data fields. Nil-safe: unit tests that build PartialContext
	// by hand without wiring a Provider fall back to pc.Data fields.
	Provider plugins.Provider
}

// AllowedFormats is the M1 set of output formats. terminal/markdown
// templates are NOT adopted in this spec (constitution principle III).
var AllowedFormats = map[string]struct{}{
	"svg":  {},
	"png":  {},
	"jpeg": {},
	"json": {},
}

// CheckFormat returns an UnsupportedFormatError when format is not in
// the supported set declared by metadata. Templates may invoke this
// from their own Check methods to share the validation.
func CheckFormat(meta *config.TemplateMetadata, format string) error {
	if format == "" {
		return nil
	}
	for _, f := range meta.Formats {
		if f == format {
			return nil
		}
	}
	return xerrors.NewUnsupportedFormatError(format, fmt.Errorf("template supports: %v", meta.Formats))
}

// CheckAccount returns nil when account is in meta.Supports, or an
// InputError otherwise. Templates that need additional shape checks
// (e.g. "repository" template requires q[repo]) layer their own logic
// on top of this helper.
func CheckAccount(meta *config.TemplateMetadata, account string) error {
	if account == "" || len(meta.Supports) == 0 {
		return nil
	}
	for _, s := range meta.Supports {
		if s == account {
			return nil
		}
	}
	return xerrors.NewInputError("account", fmt.Errorf("template supports: %v", meta.Supports))
}

// Registry plumbing -----------------------------------------------------

var (
	registryMu sync.RWMutex
	registry   = map[string]Template{}
)

// Register adds t to the registry. Duplicate names panic, matching the
// plugin registry's contract.
func Register(t Template) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := t.Name()
	if name == "" {
		panic("templates: Register called with empty name")
	}
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("templates: %q registered twice", name))
	}
	registry[name] = t
}

// Get returns the registered template by name.
func Get(name string) (Template, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := registry[name]
	return t, ok
}

// MustGet returns the template or a *NotFoundError so callers can
// branch with errors.As. Used by engine.Compute (T057).
func MustGet(name string) (Template, error) {
	if t, ok := Get(name); ok {
		return t, nil
	}
	return nil, xerrors.NewNotFoundError("template:"+name, nil)
}

// Each iterates registered templates in name-sorted order.
func Each(fn func(name string, t Template) error) error {
	registryMu.RLock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	clone := make(map[string]Template, len(registry))
	for k, v := range registry {
		clone[k] = v
	}
	registryMu.RUnlock()
	sort.Strings(keys)
	for _, k := range keys {
		if err := fn(k, clone[k]); err != nil {
			return err
		}
	}
	return nil
}

// Reset clears the registry. Tests only.
func Reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Template{}
}
