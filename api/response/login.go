package response

// Login is returned by the server after a request to authenticate to the API.
type Login struct {
	// Status of the login request.
	// It equals 1 if the request failed.
	// It equals 0 if the authentication is successful.
	Status int `json:"status"`
	// Data contains authentication information.
	// It's empty if the request failed.
	Data DataLogin `json:"data"`
}

// DataLogin contains authentication information when the authentication is
// successful.
type DataLogin struct {
	// ContextID is a token that must be sent to authenticate subsequent requests.
	ContextID string `json:"contextID"`
	// The username that was used during login.
	Username string `json:"username"`
	// Groups of the user that is authenticated.
	Groups string `json:"groups"`
}
