package livebox

import (
	"net/http"

	internalHTTP "github.com/Tomy2e/livebox-api-client/internal/http"
)

const DefaultAddress = "http://192.168.1.1"

// Client is a Livebox API Client. Requests sent using a client will
// be automatically authenticated using the specified password.
// Client is thread safe.
type Client struct {
	// HTTP client that will be used to send HTTP requests.
	client *internalHTTP.Client
	// Password of the "admin" user.
	password string
	// Session data.
	session *session
}

// NewClient returns a new Client that will be authenticated using the given
// password.
func NewClient(password string, opts ...Opt) (*Client, error) {
	var (
		err error
		co  = newClientOpts(opts)
		c   = &Client{
			password: password,
			client:   internalHTTP.NewClient(co.httpClient),
		}
	)

	c.session, err = newSession(c.client, co.address)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// clientOpts contain client custom options.
type clientOpts struct {
	address    string
	httpClient *http.Client
}

// newClientOpts returns a clientOpts object with the custom options.
// If an option was not specified, the default value for this option is used.
func newClientOpts(opts []Opt) *clientOpts {
	co := &clientOpts{
		httpClient: http.DefaultClient,
		address:    DefaultAddress,
	}

	for _, f := range opts {
		f(co)
	}

	return co
}

// Opt is a Livebox client option.
type Opt func(c *clientOpts)

// WithHTTPClient allows using a custom http client. If not used, the Go default
// HTTP client is used.
func WithHTTPClient(httpClient *http.Client) Opt {
	return func(c *clientOpts) {
		c.httpClient = httpClient
	}
}

// WithAddress allows using a custom Livebox address. If not used, the Livebox
// address is set to http://192.168.1.1.
func WithAddress(address string) Opt {
	return func(c *clientOpts) {
		c.address = address
	}
}
