package response

import (
	"errors"
	"fmt"
	"net/http"
)

// Errors is a response that may be returned by the API when it cannot
// successfully respond to the user request.
type Errors struct {
	Errors []Error `json:"errors"`
}

// ErrorCode is the code of the error.
type ErrorCode int

// PermissionDeniedErrorCode is returned by the server when the requested action
// is denied. This is mostly due to an invalid or expired session.
const PermissionDeniedErrorCode ErrorCode = 13

// Error contains a detailed error. It implements the error interface.
type Error struct {
	// Error code.
	ErrorCode ErrorCode `json:"error"`
	// Description of the error.
	Description string `json:"description"`
	// Error info.
	Info string `json:"info"`
}

// Error returns the error as a string.
func (e *Error) Error() string {
	return fmt.Sprintf(
		"Error: %d, Description: %s, Info: %s",
		e.ErrorCode,
		e.Description,
		e.Info,
	)
}

// IsPermissionDeniedError returns true if the Livebox API returned a permission
// denied error (the session is expired or not valid).
func IsPermissionDeniedError(err error) bool {
	var respError *Error
	return errors.As(err, &respError) && respError.ErrorCode == PermissionDeniedErrorCode
}

// StatusError is returned when the status code of an HTTP response is not 200.
type StatusError struct {
	Got int
}

// Error returns the status error as a string.
func (s *StatusError) Error() string {
	return fmt.Sprintf("status error: got %d, expected 200", s.Got)
}

// NewStatusError returns a new StatusError with the status code that was received.
func NewStatusError(got int) *StatusError {
	return &StatusError{
		Got: got,
	}
}

// IsStatusError returns true if the error is a StatusError.
func IsStatusError(err error) bool {
	var statusError *StatusError
	return errors.As(err, &statusError)
}

// IsStatusErrorUnauthorized returns true if the error is a StatusError and
// the received status code is 401.
func IsStatusErrorUnauthorized(err error) bool {
	var statusError *StatusError
	return errors.As(err, &statusError) && statusError.Got == http.StatusUnauthorized
}
