package form3api

import (
	"fmt"
	"net/http"
)

type ErrHttp struct {
	StatusCode int
}

func (e ErrHttp) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, http.StatusText(e.StatusCode))
}

func newErrHttp(statusCode int) error {
	return &ErrHttp{StatusCode: statusCode}
}

// ErrNotFound is returned when some arbitrary resource cannot be found.
type ErrNotFound struct{}

func (e ErrNotFound) Error() string {
	return "not found"
}

// ErrTooManyRetries is the error used when client got throttled past the limit.
type ErrTooManyRetries struct{}

func (e ErrTooManyRetries) Error() string {
	return "too many retries"
}

func isSameGenericError(a, b GenericError) bool {
	return (a.ErrorCode == b.ErrorCode || b.ErrorCode == "") &&
		(a.ErrorMessage == b.ErrorMessage || b.ErrorMessage == "")
}

// ErrBadRequest means that the server wasn't expecting request in the current
// form, but most ofthen that some required field is missing.
type ErrBadRequest struct {
	GenericError
}

func (e ErrBadRequest) Error() string {
	if len(e.ErrorCode) > 0 {
		return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorMessage)
	}
	return e.ErrorMessage
}

func (e *ErrBadRequest) Is(target error) bool {
	t, ok := target.(*ErrBadRequest)
	if !ok {
		return false
	}
	return isSameGenericError(e.GenericError, t.GenericError)
}

func newErrBadRequest(e GenericError) error {
	return &ErrBadRequest{GenericError: e}
}

// ErrConflict is returned when the resource has already been created or when
// invalid version has been specified.
type ErrConflict struct {
	GenericError
}

func (e *ErrConflict) Is(target error) bool {
	t, ok := target.(*ErrConflict)
	if !ok {
		return false
	}
	return isSameGenericError(e.GenericError, t.GenericError)
}

func (e ErrConflict) Error() string {
	if len(e.ErrorCode) > 0 {
		return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorMessage)
	}
	return e.ErrorMessage
}

func newErrConflict(e GenericError) error {
	return &ErrConflict{GenericError: e}
}

func isSameForbiddenError(a, b ForbiddenError) bool {
	return (a.Error == b.Error || b.Error == "") &&
		(a.ErrorDescription == b.ErrorDescription || b.ErrorDescription == "")
}

// ErrForbidden is the error returned when the client doesn't have an access to
// the resource.
type ErrForbidden struct {
	ForbiddenError
}

func (e ErrForbidden) Error() string {
	if len(e.ForbiddenError.Error) > 0 {
		return fmt.Sprintf("%s: %s", e.ForbiddenError.Error, e.ErrorDescription)
	}
	return e.ErrorDescription
}

func (e *ErrForbidden) Is(target error) bool {
	t, ok := target.(*ErrForbidden)
	if !ok {
		return false
	}
	return isSameForbiddenError(e.ForbiddenError, t.ForbiddenError)
}

func newErrForbiden(e ForbiddenError) error {
	return &ErrForbidden{ForbiddenError: e}
}
