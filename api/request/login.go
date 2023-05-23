package request

// NewLogin returns a new authentication request object.
func NewLogin(username, password string) *Request {
	return New(
		"sah.Device.Information",
		"createContext",
		Parameters{
			"applicationName": "webui",
			"username":        username,
			"password":        password,
		},
	)
}
