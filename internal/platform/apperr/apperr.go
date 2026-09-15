// Package apperr defines application-level errors that transports map to
// responses. Domain and application layers never depend on HTTP status codes.
package apperr

import (
	"errors"
	"fmt"
)

// Code categorizes an application error.
type Code string

const (
	CodeBadRequest   Code = "bad_request"
	CodeValidation   Code = "validation_error"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeInternal     Code = "internal_error"
)

// Error is an application error carrying a machine-readable code.
type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// New builds an application error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// BadRequest reports malformed input that is not a domain validation failure.
func BadRequest(message string) *Error { return New(CodeBadRequest, message) }

// Validation reports a failed domain validation rule.
func Validation(message string) *Error { return New(CodeValidation, message) }

// NotFound reports a missing entity.
func NotFound(kind, id string) *Error {
	return New(CodeNotFound, fmt.Sprintf("%s %q not found", kind, id))
}

// Conflict reports a state conflict, such as an invalid lifecycle transition.
func Conflict(message string) *Error { return New(CodeConflict, message) }

// Internal wraps an unexpected infrastructure failure.
func Internal(err error) *Error {
	return &Error{Code: CodeInternal, Message: "internal error", Err: err}
}

// Unauthorized reports a missing or invalid identity.
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }

// Forbidden reports an action the actor is not allowed to perform.
func Forbidden(message string) *Error { return New(CodeForbidden, message) }

// Is reports whether err is an application error with the given code.
func Is(err error, code Code) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// CodeOf returns the application error code for err, or CodeInternal.
func CodeOf(err error) Code {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}
