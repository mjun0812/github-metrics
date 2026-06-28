package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// InputError signals a user-supplied input value that fails token /
// config validation at startup. Like ConfigError it is never retried;
// callers print the message + exit 1.
type InputError struct {
	Key string
	Msg string
}

func (e *InputError) Error() string { return e.Msg }

// Quota collects the per-API quota a request needs to succeed. Each
// plugin's metadata.yml can declare `quota_required_rest|graphql|search`;
// the validator sums those across the enabled plugin set.
type Quota struct {
	REST    int
	GraphQL int
	Search  int
}

// RateState mirrors `GET /rate_limit`'s salient fields.
type RateState struct {
	REST    RateBucket
	GraphQL RateBucket
	Search  RateBucket
}

// RateBucket is a single API bucket's remaining + reset state.
type RateBucket struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

// ValidationResult carries the verdict of the 3 token-validation
// stages (format / scope / quota). Callers map it to exit codes:
//   - Reject* → exit 1
//   - QuotaSufficient=false → exit 0 (skipped)
//   - everything OK → continue
type ValidationResult struct {
	MissingScopes   []string
	QuotaSufficient bool
	RateState       *RateState
}

// TokenValidator holds the inputs the 3-stage validation needs.
type TokenValidator struct {
	Token          config.Token
	REST           *githubapi.REST
	RequiredScopes []string
	Quota          Quota
	UseMockedData  bool
}

// Validate runs the 3 token-validation stages:
//
//  1. Format reject: github_pat_* + empty-no-mock paths.
//  2. Scope check: HEAD / → X-OAuth-Scopes vs RequiredScopes.
//     Missing scopes are returned as a warning (caller logs them);
//     not an error.
//  3. Quota check: GET /rate_limit → compare against Quota. When
//     remaining < required for any API, QuotaSufficient=false.
//
// MOCKED_TOKEN (M1 convention) short-circuits past stages 2-3.
func (v *TokenValidator) Validate(ctx context.Context) (ValidationResult, error) {
	raw := v.Token.Reveal()

	// Stage 1: format reject.
	switch {
	case strings.HasPrefix(raw, "github_pat_"):
		return ValidationResult{}, &InputError{
			Key: "token",
			Msg: "Error: fine-grained personal access tokens (github_pat_*) are not supported. " +
				"The metrics action requires a classic GitHub PAT (ghp_*) with appropriate scopes. " +
				"See https://github.com/settings/tokens to generate one.",
		}
	case raw == "":
		if !v.UseMockedData {
			return ValidationResult{}, &InputError{
				Key: "token",
				Msg: "token required: set the GITHUB_TOKEN env var " +
					"(or use 'with: token:' in your GitHub Actions workflow). " +
					"For offline demo set use_mocked_data=true (action.yml) " +
					"or --plugin use_mocked_data=true (CLI).",
			}
		}
		// Empty token + mocked data is the offline-demo path.
		return ValidationResult{QuotaSufficient: true}, nil
	case raw == "MOCKED_TOKEN":
		return ValidationResult{QuotaSufficient: true}, nil
	}

	// Stage 2: scope check (best-effort; fail = warning, continue).
	scopes, scopeErr := v.fetchScopes(ctx)
	if scopeErr != nil {
		// Transient network error → propagate as retryable so the
		// outer RetryPolicy can decide. Permanent errors should be
		// surfaced too — caller decides whether to fail.
		return ValidationResult{}, xerrors.NewRetryableError(
			fmt.Errorf("token validation: fetch scopes: %w", scopeErr),
		)
	}
	missing := diffScopes(v.RequiredScopes, scopes)

	// Stage 3: quota check.
	rate, rateErr := v.fetchRateLimit(ctx)
	if rateErr != nil {
		return ValidationResult{}, xerrors.NewRetryableError(
			fmt.Errorf("token validation: fetch rate_limit: %w", rateErr),
		)
	}
	sufficient := rate.REST.Remaining >= v.Quota.REST &&
		rate.GraphQL.Remaining >= v.Quota.GraphQL &&
		rate.Search.Remaining >= v.Quota.Search

	return ValidationResult{
		MissingScopes:   missing,
		QuotaSufficient: sufficient,
		RateState:       rate,
	}, nil
}

// fetchScopes wraps REST.Scopes (M1) so the validator stays mockable
// at this seam. Returns an empty slice for unauthenticated tokens.
func (v *TokenValidator) fetchScopes(ctx context.Context) ([]string, error) {
	if v.REST == nil {
		return nil, fmt.Errorf("nil REST client")
	}
	return v.REST.Scopes(ctx)
}

// fetchRateLimit hits GET /rate_limit and decodes the canonical
// shape into RateState. The REST helper returns the raw body + http
// response so we parse the small JSON locally.
func (v *TokenValidator) fetchRateLimit(ctx context.Context) (*RateState, error) {
	if v.REST == nil {
		return nil, fmt.Errorf("nil REST client")
	}
	body, resp, err := v.REST.Get(ctx, "/rate_limit", nil)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("rate_limit returned status %d", status)
	}
	return decodeRateLimit(body)
}

// decodeRateLimit parses the GitHub /rate_limit JSON shape into the
// RateState struct. Public for token_test.go reuse.
func decodeRateLimit(body []byte) (*RateState, error) {
	var doc struct {
		Resources struct {
			Core    rawBucket `json:"core"`
			Graphql rawBucket `json:"graphql"`
			Search  rawBucket `json:"search"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("rate_limit JSON decode: %w", err)
	}
	return &RateState{
		REST:    doc.Resources.Core.toBucket(),
		GraphQL: doc.Resources.Graphql.toBucket(),
		Search:  doc.Resources.Search.toBucket(),
	}, nil
}

type rawBucket struct {
	Remaining int   `json:"remaining"`
	Limit     int   `json:"limit"`
	Reset     int64 `json:"reset"`
}

func (b rawBucket) toBucket() RateBucket {
	return RateBucket{
		Remaining: b.Remaining,
		Limit:     b.Limit,
		Reset:     time.Unix(b.Reset, 0).UTC(),
	}
}

// diffScopes returns the elements of required that are missing from
// have. Comparison is case-sensitive (GitHub scope tokens are
// lowercase by convention).
func diffScopes(required, have []string) []string {
	if len(required) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(have))
	for _, s := range have {
		set[strings.TrimSpace(s)] = struct{}{}
	}
	var missing []string
	for _, s := range required {
		if _, ok := set[s]; !ok {
			missing = append(missing, s)
		}
	}
	return missing
}
