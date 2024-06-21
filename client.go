package livebox

import (
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Tomy2e/livebox-api-client/internal/client"
)

const (
	DefaultAddress  = "http://192.168.1.1"
	DefaultUsername = "admin"
)

// ErrInvalidCredentials is returned when the login is not successful because
// the login or password is invalid.
var ErrInvalidCredentials = client.ErrInvalidCredentials

// Client is a Livebox API Client. Requests sent using a client will be automatically
// authenticated using the specified password. Client is thread safe.
type Client struct {
	client *client.Client
	log    *slog.Logger

	// Events keep-alive.
	mu           sync.Mutex
	eventsCtr    uint64
	eventsStopCh chan<- struct{}
}

// NewClient returns a new Client that will be authenticated using the given password.
func NewClient(password string, opts ...Opt) (*Client, error) {
	co := newClientOpts(opts)

	c, err := client.New(co.httpClient, co.address, co.username, password)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: c,
		log:    co.log,
	}, nil
}

// clientOpts contain client custom options.
type clientOpts struct {
	address    string
	username   string
	httpClient *http.Client
	log        *slog.Logger
}

// newClientOpts returns a clientOpts object with the custom options.
// If an option was not specified, the default value for this option is used.
func newClientOpts(opts []Opt) *clientOpts {
	co := &clientOpts{
		httpClient: http.DefaultClient,
		address:    DefaultAddress,
		username:   DefaultUsername,
	}

	for _, f := range opts {
		f(co)
	}

	if co.log == nil {
		co.log = slog.New(slog.NewTextHandler(io.Discard, nil))
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

// WithLogger attaches a logger to the client. Logging is disabled if unset.
func WithLogger(log *slog.Logger) Opt {
	return func(c *clientOpts) {
		c.log = log
	}
}

// WithUsername sets the username that will be used to authenticate. Defaults
// to "admin" if not specified.
func WithUsername(username string) Opt {
	return func(c *clientOpts) {
		c.username = username
	}
}
