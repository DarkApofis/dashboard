// Package errs is the typed error vocabulary shared by every domain package.
//
// It deliberately knows nothing about HTTP or SQL: a domain error carries a
// Kind (what class of failure it is), a stable machine-readable Code and a
// human message. The transport layer is the single place that turns a Kind into
// a status code, and the infrastructure layer is the single place that turns a
// driver error into one of these.
package errs

import (
	"errors"
	"fmt"
)

// Kind classifies a failure so that callers can react without string matching.
type Kind string

const (
	// KindValidation is input that cannot even be interpreted: a malformed body,
	// a non-numeric id, an unknown status in a query string. HTTP 400.
	KindValidation Kind = "validation"
	// KindUnprocessable is input that is well formed but breaks a business rule:
	// a non-positive amount, an expiration in the past, a business_id that does
	// not exist. HTTP 422. Keeping it apart from KindValidation lets a client
	// tell "I sent garbage" from "I sent something you will not accept".
	KindUnprocessable Kind = "unprocessable"
	KindNotFound      Kind = "not_found"
	KindConflict      Kind = "conflict"
	KindUnauthorized  Kind = "unauthorized"
	KindInternal      Kind = "internal"
)

// Error is a domain error. Values are normally declared as package-level
// sentinels and enriched per call site with Wrap/WithMessage/WithDetail, which
// always return a copy so the sentinel itself is never mutated.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details map[string]string

	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is matches on Code, so a wrapped or re-messaged copy of a sentinel still
// satisfies errors.Is(err, Sentinel).
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && other.Code == e.Code
}

// Wrap attaches the underlying cause. The cause is for logs only; it never
// reaches the API response.
func (e *Error) Wrap(cause error) *Error {
	copied := e.clone()
	copied.cause = cause
	return copied
}

// WithMessage replaces the human message, keeping Kind and Code.
func (e *Error) WithMessage(format string, args ...any) *Error {
	copied := e.clone()
	copied.Message = fmt.Sprintf(format, args...)
	return copied
}

// WithDetail adds a field-level explanation, typically the offending input.
func (e *Error) WithDetail(field, message string) *Error {
	copied := e.clone()
	if copied.Details == nil {
		copied.Details = make(map[string]string, 1)
	}
	copied.Details[field] = message
	return copied
}

func (e *Error) clone() *Error {
	copied := *e
	if e.Details != nil {
		copied.Details = make(map[string]string, len(e.Details))
		for k, v := range e.Details {
			copied.Details[k] = v
		}
	}
	return &copied
}

// New builds a domain error of an arbitrary kind.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Validation reports input that cannot be interpreted (HTTP 400).
func Validation(code, message string) *Error { return New(KindValidation, code, message) }

// Unprocessable reports well-formed input that breaks a business rule (HTTP 422).
func Unprocessable(code, message string) *Error { return New(KindUnprocessable, code, message) }

// NotFound reports a missing resource (HTTP 404).
func NotFound(code, message string) *Error { return New(KindNotFound, code, message) }

// Conflict reports a request that clashes with the current state (HTTP 409).
func Conflict(code, message string) *Error { return New(KindConflict, code, message) }

// Unauthorized reports a failed authentication check (HTTP 401).
func Unauthorized(code, message string) *Error { return New(KindUnauthorized, code, message) }

// Internal reports an unexpected failure (HTTP 500).
func Internal(code, message string) *Error { return New(KindInternal, code, message) }

// From extracts the domain error from an error chain, if there is one.
func From(err error) (*Error, bool) {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr, true
	}
	return nil, false
}
