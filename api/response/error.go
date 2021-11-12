package response

import "fmt"

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
