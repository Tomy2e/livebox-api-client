package request

// NewLogin returns a new authentication request object. the username is
// hard-coded to "admin".
func NewLogin(password string) *Request {
	return New(
		"sah.Device.Information",
		"createContext",
		map[string]interface{}{
			"applicationName": "webui",
			"username":        "admin",
			"password":        password,
		},
	)
}
