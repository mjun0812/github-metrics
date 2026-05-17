package action

import (
	"fmt"
	"strings"
)

// ConfigError flags a user-supplied input value that fails validation
// at startup. It is intentionally separate from xerrors.RetryableError
// (this error class is never retried — see contracts/retry-policy.md).
// Callers should print the message verbatim + exit 1.
type ConfigError struct {
	Key   string
	Value string
	Msg   string
}

func (e *ConfigError) Error() string { return e.Msg }

// OutputActionRegistry holds the supported output_action values + the
// migration message map for unsupported values. The registry is
// returned by DefaultRegistry(); callers do not need to allocate
// their own.
type OutputActionRegistry struct {
	Supported            []string
	UnsupportedMigration map[string]string
	supportedList        string // pre-computed " / "-joined for Validate's error message
}

// DefaultRegistry returns the canonical registry for this M6 release.
//
// Supported (6 values): none, commit, pull-request,
// pull-request-merge, pull-request-squash, pull-request-rebase.
//
// Unsupported migration: gist + markdown-* + mixed forms get an
// explicit error message pointing at the adopted alternative.
// Spec FR-015b + contracts/output-actions.md.
func DefaultRegistry() *OutputActionRegistry {
	supported := []string{
		"none",
		"commit",
		"pull-request",
		"pull-request-merge",
		"pull-request-squash",
		"pull-request-rebase",
	}
	supportedList := strings.Join(supported, " / ")

	return &OutputActionRegistry{
		Supported: supported,
		UnsupportedMigration: map[string]string{
			"gist": "output_action='gist' is not supported in this distribution. " +
				"Use 'commit' to commit the rendered file directly to the repository, " +
				"or open an issue if Gist support is critical to your workflow.",
			"markdown commit": "output_action='markdown commit' is not supported " +
				"(markdown template is out of MVP scope). Use 'commit' with " +
				"config_output=svg/png/jpeg/json instead.",
			"markdown pull-request": "output_action='markdown pull-request' is not supported " +
				"(markdown template is out of MVP scope). Use 'pull-request' with " +
				"config_output=svg/png/jpeg/json instead.",
			"markdown gist": "output_action='markdown gist' is not supported " +
				"(both markdown template and Gist are out of MVP scope). " +
				"Use 'commit' or 'pull-request' with config_output=svg/png/jpeg/json instead.",
			"gist pull-request": "output_action='gist pull-request' is not supported " +
				"(Gist is out of MVP scope). Use 'pull-request' to open a PR with the rendered file.",
		},
		// supportedList is captured below in Validate's error message.
		supportedList: supportedList,
	}
}

// Validate returns nil iff value is one of the Supported entries.
// Otherwise it returns a *ConfigError suitable for fail-fast exit 1
// with a migration-friendly message (FR-015b).
func (r *OutputActionRegistry) Validate(value string) error {
	for _, s := range r.Supported {
		if value == s {
			return nil
		}
	}
	if msg, ok := r.UnsupportedMigration[value]; ok {
		return &ConfigError{Key: "output_action", Value: value, Msg: msg}
	}
	return &ConfigError{
		Key:   "output_action",
		Value: value,
		Msg: fmt.Sprintf(
			"output_action=%q is not a recognized value. Supported: %s. "+
				"See https://github.com/mjun0812/github-metrics/blob/main/action.yml for the full input contract.",
			value, r.supportedList,
		),
	}
}
