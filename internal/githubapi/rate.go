package githubapi

import (
	"context"
	"sync"
)

// Resources is the concurrent-safe holder for the three GitHub API
// rate buckets (data-model E-008). The zero value is ready to use; call
// [Resources.Refresh] to populate from a live or mocked /rate_limit.
type Resources struct {
	mu      sync.RWMutex
	rest    Quota
	graphql Quota
	search  Quota
}

// NewResources returns a Resources with zero-value Quotas.
func NewResources() *Resources { return &Resources{} }

// Refresh queries /rate_limit and atomically updates every bucket.
// On failure the previous values are retained and the error is
// returned unmodified so callers can decide whether to retry.
func (r *Resources) Refresh(ctx context.Context, c *REST) error {
	rl, err := c.RateLimit(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.rest = rl.Resources.Core
	r.graphql = rl.Resources.GraphQL
	r.search = rl.Resources.Search
	r.mu.Unlock()
	return nil
}

// Snapshot returns a read-only copy of the current quota values.
// Returned Quotas are value types, so the caller cannot mutate the
// holder's internal state through them.
func (r *Resources) Snapshot() ResourcesSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return ResourcesSnapshot{
		REST:    r.rest,
		GraphQL: r.graphql,
		Search:  r.search,
	}
}

// ResourcesSnapshot is a value-type view of the three quotas at a
// point in time, used by callers that only need to read.
type ResourcesSnapshot struct {
	REST    Quota
	GraphQL Quota
	Search  Quota
}
