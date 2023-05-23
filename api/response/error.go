package response

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode is the code of the error.
type ErrorCode int

// PermissionDeniedErrorCode is returned by the server when the requested action
// is denied. This is mostly due to an invalid or expired session.
const PermissionDeniedErrorCode ErrorCode = 13

// Errors is a response that may be returned by the API when it cannot
// successfully respond to the user request.
//
//nolint:errname // Simplifies the name.
type Errors struct {
	Errors []*Error `json:"errors"`
}

func (e *Errors) Error() string {
	str := strings.Builder{}
	for i, err := range e.Errors {
		if i != 0 {
			str.WriteString("\n")
		}

		str.WriteString(err.Error())
	}

	return str.String()
}

// Error contains a detailed error.
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
	return errorMatches(err, func(err *Error) bool { return err.ErrorCode == PermissionDeniedErrorCode })
}

// IsChannelDoesNotExistError returns true if the Livebox API returned a
// "channel does not exist" error.
func IsChannelDoesNotExistError(err error) bool {
	return errorMatches(err, func(err *Error) bool { return err.Info == "channel does not exist" })
}

// IsFunctionExecutionFailedError returns true if the Livebox API returned a
// "Function execution failed" error.
func IsFunctionExecutionFailedError(err error) bool {
	return errorMatches(err, func(err *Error) bool { return err.Description == "Function execution failed" })
}

func errorMatches(err error, f func(*Error) bool) bool {
	// Is is a multi-error?
	var respErrors *Errors
	if errors.As(err, &respErrors) {
		for _, err := range respErrors.Errors {
			if f(err) {
				return true
			}
		}

		return false
	}

	// Is it a single error?
	var respError *Error
	if errors.As(err, &respError) {
		return f(respError)
	}

	// It's something else.
	return false
}
