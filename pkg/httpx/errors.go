// Package httpx contains shared HTTP plumbing: typed error categories,
// the error → status code mapper, and small middleware helpers used by
// both apps/api and apps/worker (when worker exposes admin endpoints).
package httpx

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a machine-readable category for typed errors. The HTTP error
// mapper translates these to status codes; clients see Code as a string
// in the response body and can branch on it without parsing messages.
type Code string

const (
	CodeBadRequest   Code = "BAD_REQUEST"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeRateLimited  Code = "RATE_LIMITED"
	CodeInternal     Code = "INTERNAL"
	CodeUnavailable  Code = "UNAVAILABLE"
)

// Error is the typed error returned by service / repo code. Handlers do
// not return raw errors; the error mapper unwraps an *Error and
// produces the corresponding status + JSON body.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New builds a typed error without a wrapped cause.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap attaches a typed code+message to an existing error.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// As is a convenience for errors.As that returns the typed *Error or nil.
func As(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// HTTPStatus maps a Code to the matching HTTP status code.
func (c Code) HTTPStatus() int {
	switch c {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
