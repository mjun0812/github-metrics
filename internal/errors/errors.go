// Package errors defines the typed error model for the github-metrics
// project. Each type carries one structured field describing the source
// of the problem (input name, missing resource, denied reason, ...) so
// that callers can branch with errors.As without parsing strings.
package errors

import (
	"errors"
	"fmt"
)

// InputError reports that an input value failed validation or could not be
// normalized. Field identifies which input (typically the metadata.yml key).
type InputError struct {
	Field string
	Cause error
}

func (e *InputError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("invalid input %q: %v", e.Field, e.Cause)
	}
	return fmt.Sprintf("invalid input %q", e.Field)
}

func (e *InputError) Unwrap() error { return e.Cause }

// NotFoundError reports that a requested resource (login, template, plugin)
// was not present.
type NotFoundError struct {
	Resource string
	Cause    error
}

func (e *NotFoundError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("not found %q: %v", e.Resource, e.Cause)
	}
	return fmt.Sprintf("not found %q", e.Resource)
}

func (e *NotFoundError) Unwrap() error { return e.Cause }

// ForbiddenError reports that the operation is denied (auth scope, allow-list,
// rate exhaustion, ...). Reason carries a short machine-readable code.
type ForbiddenError struct {
	Reason string
	Cause  error
}

func (e *ForbiddenError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("forbidden (%s): %v", e.Reason, e.Cause)
	}
	return fmt.Sprintf("forbidden (%s)", e.Reason)
}

func (e *ForbiddenError) Unwrap() error { return e.Cause }

// UnsupportedFormatError reports that a requested rendering or input
// format is not supported by the active template / plugin.
type UnsupportedFormatError struct {
	Format string
	Cause  error
}

func (e *UnsupportedFormatError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("unsupported format %q: %v", e.Format, e.Cause)
	}
	return fmt.Sprintf("unsupported format %q", e.Format)
}

func (e *UnsupportedFormatError) Unwrap() error { return e.Cause }

// RetryableError wraps a transient failure that callers should retry
// (or that the plugin runner has already recovered from a panic).
type RetryableError struct {
	Cause error
}

func (e *RetryableError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("retryable: %v", e.Cause)
	}
	return "retryable"
}

func (e *RetryableError) Unwrap() error { return e.Cause }

// Convenience constructors keep call sites compact while preserving the
// underlying type information for errors.As.

// NewInputError returns an *InputError describing a malformed or
// unexpected input value.
func NewInputError(field string, cause error) error {
	return &InputError{Field: field, Cause: cause}
}

// NewNotFoundError returns a *NotFoundError keyed by the missing
// resource identifier.
func NewNotFoundError(resource string, cause error) error {
	return &NotFoundError{Resource: resource, Cause: cause}
}

// NewForbiddenError returns a *ForbiddenError tagged with a short
// machine-readable reason code.
func NewForbiddenError(reason string, cause error) error {
	return &ForbiddenError{Reason: reason, Cause: cause}
}

// NewUnsupportedFormatError returns an *UnsupportedFormatError tagged
// with the unsupported format string.
func NewUnsupportedFormatError(format string, cause error) error {
	return &UnsupportedFormatError{Format: format, Cause: cause}
}

// NewRetryableError wraps a transient failure that callers should
// retry. Also used by the plugin runner when a panic is recovered.
func NewRetryableError(cause error) error {
	return &RetryableError{Cause: cause}
}

// Re-export the standard library helpers so callers can rely on a single
// import for typed-error handling.
var (
	Is = errors.Is
	As = errors.As
)
