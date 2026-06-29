// Package dataprovider is the lazy + memoized profile fetcher
// introduced by #603. It replaces the eagerly-populated
// pc.Data.User / pc.Data.Organization / pc.Data.Computed.RepositoryList
// fields with on-demand accessors plugins call when they actually need
// the data.
//
// Every method is safe for concurrent use: in-flight calls collapse via
// golang.org/x/sync/singleflight, and both first-success and first-error
// outcomes are cached forever so subsequent calls return without
// touching the network. The lifetime of a Provider matches a single
// engine.Compute request; tests construct their own via New or supply
// the plugins.Provider interface directly.
package dataprovider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Cache keys used internally by the singleflight + sync.Map memoizer.
const (
	keyProfile      = "profile"
	keyRepositories = "repositories"
	keyCommits      = "commits"
	keyRepo         = "repo"
)

// Provider concretely implements the plugins.Provider interface. It is
// constructed once per engine.Compute call via New and shared across
// every plugin goroutine.
type Provider struct {
	login string
	// repo is the single repository name for the M7 repository template
	// (Account == AccountRepository). Empty for the classic user /
	// organization templates. When set, Repositories / RepositorySummary
	// synthesize their result from the single Repo() fetch instead of
	// paging the account-wide connection.
	repo   string
	gql    *githubapi.GraphQL
	rest   *githubapi.REST
	logger *slog.Logger

	// skipPrivate, when true, instructs the repository paging fetch
	// (fetchOneRepoPage) to drop nodes with isPrivate == true before
	// they reach the accumulator. Repo-mode (synthesizeRepoResult)
	// bypasses the filter — the user explicitly named the repo.
	skipPrivate bool

	group singleflight.Group
	cache sync.Map // string -> *result
}

// Options carries optional Provider configuration. Use the zero value
// when no overrides are needed.
type Options struct {
	// SkipPrivate, when true, drops isPrivate repositories during the
	// account-wide paging fetch so every consumer plugin sees only the
	// public subset. Has no effect in repository-template mode.
	SkipPrivate bool
}

// result is the cached value behind a cache key. Both Value and Err are
// stored so a permanent failure short-circuits subsequent calls
// (matching the spec: "result cached forever (success OR error)").
type result struct {
	value any
	err   error
}

// New returns a Provider that fetches the profile of login via gql/rest.
// repo is the single repository name for the M7 repository template;
// pass "" for the classic user / organization templates. logger may be
// nil; callers typically pass the engine's logger. opts carries
// optional behavior toggles (e.g. SkipPrivate); pass Options{} when no
// overrides are needed.
func New(login, repo string, gql *githubapi.GraphQL, rest *githubapi.REST, logger *slog.Logger, opts Options) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		login:       login,
		repo:        repo,
		gql:         gql,
		rest:        rest,
		logger:      logger,
		skipPrivate: opts.SkipPrivate,
	}
}

// ErrWrongAccountKind is returned by Provider.User / Provider.Organization
// when the resolved profile is for the other kind. Callers can use
// errors.Is for branch-specific fallback.
var ErrWrongAccountKind = errors.New("dataprovider: profile is the wrong account kind")

// Profile resolves the user-or-organization profile. The discriminated
// union returned by Profile is the canonical entry point: callers that
// already know they need a User / Organization should reach for the
// typed accessors below instead.
func (p *Provider) Profile(ctx context.Context) (*plugins.Profile, error) {
	v, err := p.memoize(ctx, keyProfile, func(ctx context.Context) (any, error) {
		return p.fetchProfile(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.(*plugins.Profile), nil
}

// User returns the User payload when the resolved profile is a user
// account. Returns ErrWrongAccountKind wrapped with the actual kind
// when the login resolves to an organization.
func (p *Provider) User(ctx context.Context) (*plugins.User, error) {
	prof, err := p.Profile(ctx)
	if err != nil {
		return nil, err
	}
	if prof.Kind != plugins.ProfileKindUser || prof.User == nil {
		return nil, fmt.Errorf("%w: want user, got %s", ErrWrongAccountKind, prof.Kind)
	}
	return prof.User, nil
}

// Organization returns the Organization payload when the resolved
// profile is an organization account. Returns ErrWrongAccountKind when
// the login resolves to a user.
func (p *Provider) Organization(ctx context.Context) (*plugins.Organization, error) {
	prof, err := p.Profile(ctx)
	if err != nil {
		return nil, err
	}
	if prof.Kind != plugins.ProfileKindOrganization || prof.Organization == nil {
		return nil, fmt.Errorf("%w: want organization, got %s", ErrWrongAccountKind, prof.Kind)
	}
	return prof.Organization, nil
}

// Repositories returns the per-node accumulator the M4 repository
// paging loop produces. Independent of Profile: callers may invoke it
// without first resolving the profile, and the result is cached
// independently so a Profile failure does not poison the repository
// cache (and vice versa).
func (p *Provider) Repositories(ctx context.Context) ([]plugins.Repository, error) {
	res, err := p.repoResult(ctx)
	if err != nil {
		return nil, err
	}
	return res.repos, nil
}

// RepositorySummary returns the aggregated repository totals (count,
// stargazers, forks, watchers, releases, packages, disk usage,
// deployments, and the top-N license preference) computed across the
// account-wide repository connection. In repository-template mode it
// returns the single-repo totals synthesized from Repo. Shares the
// memoized paging result with Repositories so the connection is walked
// at most once.
func (p *Provider) RepositorySummary(ctx context.Context) (*plugins.ComputedRepositories, error) {
	res, err := p.repoResult(ctx)
	if err != nil {
		return nil, err
	}
	return res.summary, nil
}

// repoResult memoizes the combined repository list + summary under a
// single cache key so Repositories and RepositorySummary collapse onto
// one paging walk. In repository-template mode it synthesizes both from
// the single Repo fetch.
func (p *Provider) repoResult(ctx context.Context) (*repoResult, error) {
	v, err := p.memoize(ctx, keyRepositories, func(ctx context.Context) (any, error) {
		if p.repo != "" {
			return p.synthesizeRepoResult(ctx)
		}
		return p.fetchRepoResult(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.(*repoResult), nil
}

// Repo returns the single repository payload for the M7 repository
// template. Returns (nil, nil) when the Provider is not in repository
// mode (p.repo == "") so callers can distinguish "not repo mode" from a
// fetch error.
func (p *Provider) Repo(ctx context.Context) (*plugins.Repo, error) {
	if p.repo == "" {
		return nil, nil
	}
	v, err := p.memoize(ctx, keyRepo, func(ctx context.Context) (any, error) {
		return p.fetchRepo(ctx)
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*plugins.Repo), nil
}

// CommitCalendar returns the aggregated contribution-calendar payload
// sourced from the indepth GraphQL query. Returns nil with a nil error
// when the upstream returned no calendar (organization accounts, fresh
// users) so callers can branch on absence without distinguishing it
// from a transient failure.
func (p *Provider) CommitCalendar(ctx context.Context) (*plugins.ContributionCalendar, error) {
	v, err := p.memoize(ctx, keyCommits, func(ctx context.Context) (any, error) {
		return p.fetchCommitCalendar(ctx)
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*plugins.ContributionCalendar), nil
}

// memoize collapses concurrent callers for key, then caches the
// outcome (success or error) permanently. Subsequent callers short
// circuit through the cache without re-entering singleflight.
//
// One exception: context.Canceled / context.DeadlineExceeded are NOT
// cached. Such errors describe a property of the in-flight caller's
// ctx (it was canceled or timed out), not of the upstream resource —
// caching them would poison the Provider for the rest of the request
// scope, including callers that arrived later with a fresh ctx. The
// caller that observed the cancellation still gets the error; the next
// caller re-enters the fetch under its own ctx.
func (p *Provider) memoize(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error) {
	if cached, ok := p.cache.Load(key); ok {
		r := cached.(*result)
		return r.value, r.err
	}
	v, err, _ := p.group.Do(key, func() (any, error) {
		// Re-check after winning the singleflight to honour any
		// concurrent writer (singleflight de-dupes in-flight calls
		// only; a parallel caller that finished first has already
		// stored the cached result).
		if cached, ok := p.cache.Load(key); ok {
			r := cached.(*result)
			return r.value, r.err
		}
		val, fetchErr := fn(ctx)
		if fetchErr != nil && (errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded)) {
			// Skip cache.Store so a later caller with a fresh ctx
			// can re-enter the fetch. The current caller still
			// receives fetchErr below.
			return val, fetchErr
		}
		p.cache.Store(key, &result{value: val, err: fetchErr})
		return val, fetchErr
	})
	return v, err
}
