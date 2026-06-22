// Package languages — indepth.go implements the "languages.indepth"
// sub-mode of the M4 languages plugin. It walks
// base.Computed.RepositoryList, shallow clones each repo with go-git,
// runs go-enry against every blob in HEAD's tree, and aggregates byte
// counts per language across all repos.
//
// Like recent.go, the runtime is unconditionally compiled — only the
// fixture-heavy tests sit behind //go:build heavy.
package languages

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/sync/errgroup"

	enry "github.com/go-enry/go-enry/v2"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// IndepthName is the canonical plugin slug for the indepth sub-mode.
const IndepthName = "languages.indepth"

// IndepthPlugin is the singleton registered with the global plugin
// registry.
var IndepthPlugin plugins.Plugin = &indepthPlugin{}

func init() {
	plugins.Register(IndepthPlugin)
}

type indepthPlugin struct{}

func (p *indepthPlugin) Name() string                     { return IndepthName }
func (p *indepthPlugin) Metadata() *config.PluginMetadata { return nil }

// IndepthResult is the JSON payload published under
// data.Plugins["languages.indepth"]. Field set mirrors data-model E-012.
type IndepthResult struct {
	Skipped       bool                     `json:"skipped,omitempty"`
	SkippedReason string                   `json:"-"`
	Repositories  map[string]LanguageBytes `json:"repositories"`
	Total         LanguageBytes            `json:"total"`
	Analyzed      []string                 `json:"analyzed"`
	Errors        []string                 `json:"errors,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *IndepthResult) IsSkipped() bool { return r != nil && r.Skipped }

// LanguageBytes carries per-language byte and line totals.
type LanguageBytes struct {
	Bytes map[string]int64 `json:"bytes"`
	Lines map[string]int64 `json:"lines,omitempty"`
}

// IndepthCloner abstracts the shallow-clone step so tests can substitute
// a local-filesystem fixture without going through a real git server.
// Production code never sees this interface; only the heavy test file
// injects a custom implementation via IndepthClonerKey.
type IndepthCloner interface {
	Clone(ctx context.Context, dst, url string) (string, error)
}

// gitCloner is the production cloner backed by go-git PlainCloneContext.
type gitCloner struct{}

func (gitCloner) Clone(ctx context.Context, dst, url string) (string, error) {
	_, err := gogit.PlainCloneContext(ctx, dst, false, &gogit.CloneOptions{
		URL:               url,
		Depth:             1,
		SingleBranch:      true,
		ShallowSubmodules: true,
		Progress:          io.Discard,
	})
	if err != nil {
		return "", err
	}
	return dst, nil
}

// IndepthClonerKey is the inputs map key tests use to inject a custom
// IndepthCloner. The key is intentionally exported-only-for-tests; the
// dotted-leading-underscore shape keeps it from colliding with the
// user-facing `plugin_languages_*` input namespace.
const IndepthClonerKey = "_test_languages_indepth_cloner"

type indepthInputs struct {
	timeoutRepo  time.Duration
	timeoutTotal time.Duration
	concurrency  int
}

func (p *indepthPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	// Gate: like languages.recent, the indepth sub-mode is enabled only
	// when the parent languages plugin AND the dedicated indepth input
	// are both truthy. This keeps the plugin silent (no Data.Errors
	// pollution) when never requested.
	if !pluginutil.Truthy(pc.Inputs["plugin_languages"]) {
		return &IndepthResult{
			Skipped:       true,
			SkippedReason: "plugin_languages not enabled",
			Repositories:  map[string]LanguageBytes{},
			Total:         LanguageBytes{Bytes: map[string]int64{}, Lines: map[string]int64{}},
			Analyzed:      []string{},
		}, nil
	}
	if !pluginutil.Truthy(pc.Inputs["plugin_languages_indepth"]) {
		return &IndepthResult{
			Skipped:       true,
			SkippedReason: "plugin_languages_indepth not enabled",
			Repositories:  map[string]LanguageBytes{},
			Total:         LanguageBytes{Bytes: map[string]int64{}, Lines: map[string]int64{}},
			Analyzed:      []string{},
		}, nil
	}
	if !pluginutil.ExtrasEnabled(pc.Inputs, "extras.metrics.cpu.overuse") ||
		!pluginutil.ExtrasEnabled(pc.Inputs, "extras.metrics.run.git") ||
		!pluginutil.ExtrasEnabled(pc.Inputs, "extras.metrics.run.linguist") {
		return &IndepthResult{
			Skipped:       true,
			SkippedReason: "indepth extras not satisfied",
			Repositories:  map[string]LanguageBytes{},
			Total:         LanguageBytes{Bytes: map[string]int64{}, Lines: map[string]int64{}},
			Analyzed:      []string{},
		}, nil
	}
	in := parseIndepthInputs(pc.Inputs)
	repos := resolveRepositoryList(ctx, pc)
	if len(repos) == 0 {
		return &IndepthResult{
			Repositories: map[string]LanguageBytes{},
			Total:        LanguageBytes{Bytes: map[string]int64{}, Lines: map[string]int64{}},
			Analyzed:     []string{},
		}, nil
	}

	cln := pickIndepthCloner(pc.Inputs)

	overall, overallCancel := context.WithTimeout(ctx, in.timeoutTotal)
	defer overallCancel()

	var (
		mu       sync.Mutex
		bytesSum = map[string]int64{}
		linesSum = map[string]int64{}
		perRepo  = map[string]LanguageBytes{}
		errs     []string
		analyzed []string
	)

	g, gctx := errgroup.WithContext(overall)
	g.SetLimit(in.concurrency)

	for _, repo := range repos {
		repo := repo
		g.Go(func() error {
			// Honor the overall deadline.
			select {
			case <-gctx.Done():
				return nil
			default:
			}

			tmp, err := os.MkdirTemp("", "metrics-indepth-")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: mkdir: %v", repo.NameWithOwner, err))
				mu.Unlock()
				return nil
			}
			defer func() { _ = os.RemoveAll(tmp) }()

			repoCtx, cancel := context.WithTimeout(gctx, in.timeoutRepo)
			defer cancel()

			cloneURL := repoCloneURL(repo)
			cloneDir, err := cln.Clone(repoCtx, tmp, cloneURL)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: clone: %v", repo.NameWithOwner, err))
				mu.Unlock()
				return nil
			}

			langs, err := analyzeRepository(repoCtx, cloneDir)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: analyze: %v", repo.NameWithOwner, err))
				mu.Unlock()
				return nil
			}

			mu.Lock()
			perRepo[repo.NameWithOwner] = langs
			for lang, n := range langs.Bytes {
				bytesSum[lang] += n
			}
			for lang, n := range langs.Lines {
				linesSum[lang] += n
			}
			analyzed = append(analyzed, repo.NameWithOwner)
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	sort.Strings(analyzed)
	if perRepo == nil {
		perRepo = map[string]LanguageBytes{}
	}
	return &IndepthResult{
		Repositories: perRepo,
		Total:        LanguageBytes{Bytes: bytesSum, Lines: linesSum},
		Analyzed:     analyzed,
		Errors:       errs,
	}, nil
}

// analyzeRepository walks HEAD's tree of the given clone directory and
// returns language byte and line totals computed via go-enry over each blob.
func analyzeRepository(ctx context.Context, dir string) (LanguageBytes, error) {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return LanguageBytes{}, fmt.Errorf("open repo: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return LanguageBytes{}, fmt.Errorf("head: %w", err)
	}
	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return LanguageBytes{}, fmt.Errorf("commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return LanguageBytes{}, fmt.Errorf("tree: %w", err)
	}

	langs := LanguageBytes{Bytes: map[string]int64{}, Lines: map[string]int64{}}
	err = tree.Files().ForEach(func(f *object.File) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Skip enormous files; go-enry filename mode handles most
		// signals cheaply, but reading multi-MB blobs into memory is
		// wasted work.
		if f.Size > 1<<20 {
			lang, _ := enry.GetLanguageByExtension(f.Name)
			if lang != "" {
				langs.Bytes[lang] += f.Size
			}
			return nil
		}
		content, readErr := f.Contents()
		if readErr != nil {
			// Unreadable blob (corrupt object, etc.) → skip this file
			// but keep walking. The other blobs in the tree still
			// contribute to the per-language totals.
			return nil //nolint:nilerr // best-effort: skip unreadable blobs, keep walking.
		}
		lang := enry.GetLanguage(filepath.Base(f.Name), []byte(content))
		if lang == "" {
			return nil
		}
		langs.Bytes[lang] += int64(len(content))
		langs.Lines[lang] += countLines(content)
		return nil
	})
	if err != nil {
		return LanguageBytes{}, err
	}
	return langs, nil
}

func countLines(content string) int64 {
	if content == "" {
		return 0
	}
	n := int64(bytes.Count([]byte(content), []byte{'\n'}))
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

// repoCloneURL returns the HTTPS clone URL the indepth path uses for
// PlainCloneContext. Falls back to the upstream `URL` when present.
func repoCloneURL(r plugins.Repository) string {
	if r.URL != "" {
		// GitHub: `https://github.com/owner/name` → `.git`.
		if strings.HasSuffix(r.URL, ".git") {
			return r.URL
		}
		return r.URL + ".git"
	}
	return "https://github.com/" + r.NameWithOwner + ".git"
}

func parseIndepthInputs(in map[string]any) indepthInputs {
	out := indepthInputs{
		timeoutRepo:  7*time.Minute + 30*time.Second,
		timeoutTotal: 15 * time.Minute,
		concurrency:  4,
	}
	if v, ok := in["plugin_languages_analysis_timeout_repositories"]; ok {
		if d, ok := parseDurationLoose(v); ok {
			out.timeoutRepo = d
		}
	}
	if v, ok := in["plugin_languages_analysis_timeout"]; ok {
		if d, ok := parseDurationLoose(v); ok {
			out.timeoutTotal = d
		}
	}
	return out
}

// parseDurationLoose accepts numeric (seconds), strings like "30s",
// "5min", "7.5min" — the upstream input shape used in metadata.yml.
func parseDurationLoose(v any) (time.Duration, bool) {
	switch x := v.(type) {
	case time.Duration:
		return x, true
	case int:
		return time.Duration(x) * time.Second, true
	case int64:
		return time.Duration(x) * time.Second, true
	case float64:
		return time.Duration(x * float64(time.Second)), true
	case string:
		s := strings.TrimSpace(x)
		// Translate upstream's "min" suffix to Go's "m".
		s = strings.ReplaceAll(s, "min", "m")
		if d, err := time.ParseDuration(s); err == nil {
			return d, true
		}
	}
	return 0, false
}

// pickIndepthCloner returns the test-injected cloner if present,
// otherwise the production gitCloner.
func pickIndepthCloner(in map[string]any) IndepthCloner {
	if v, ok := in[IndepthClonerKey]; ok {
		if c, ok := v.(IndepthCloner); ok {
			return c
		}
	}
	return gitCloner{}
}
