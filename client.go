package livebox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
	internalHTTP "github.com/Tomy2e/livebox-api-client/internal/http"
)

// Client is a Livebox API Client. Requests sent using a client will
// be automatically authenticated. Client is thread safe.
type Client interface {
	Request(ctx context.Context, req *request.Request, out interface{}) error
}

// NewClient returns a new Client that will be authenticated using the given
// password.
func NewClient(password string) Client {
	return NewClientWithHTTPClient(password, &http.Client{})
}

// NewClientWithHTTPClient returns a new Client that will be authenticated
// using the given password. The given HTTP client will be used to send
// HTTP requests.
func NewClientWithHTTPClient(password string, c *http.Client) Client {
	httpClient := internalHTTP.NewClient(c)

	return &client{
		password: password,
		client:   httpClient,
		session:  newSession(httpClient),
	}
}

// client implements the Client interface.
type client struct {
	// HTTP client that will be used to send HTTP requests.
	client *internalHTTP.Client
	// Password of the "admin" user.
	password string
	// Session data.
	session *session
}

// Request sends a request to the Livebox API. If the client is not yet
// authenticated, or the session is expired, the client will try to
// authenticate using the admin password given during the creation
// of the client.
func (c *client) Request(ctx context.Context, req *request.Request, out interface{}) error {
	// Try to authenticate if it's the first request.
	if _, err := c.session.Renew(ctx, c.password, renewIfNotInitialized); err != nil {
		return err
	}

	// Send the request, if the session is expired, the client will renew the session.
	return c.request(ctx, req, out)
}

// request sends a request to the Livebox API. Before calling this function,
// the client must have been authenticated successfully in the past.
// This function handles reauthentication when the session expires.
// Reauthentication will only be attempted once.
func (c *client) request(ctx context.Context, req *request.Request, out interface{}) error {
	// Create request payload
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	authAttempted := false

	for {
		// Create HTTP request with request payload
		r, v, err := c.session.NewAuthenticatedRequest(ctx, bytes.NewReader(payload))
		if err != nil {
			return err
		}

		if _, err := c.client.SendRequest(ctx, r, out); err != nil {
			// The 200 status code is always expected, even when the session is expired.
			// Also return error now if reauthentication was already attempted.
			if response.IsStatusError(err) || authAttempted {
				return err
			}

			// Check if the server returned a permission denied error.
			var respError *response.Error
			if errors.As(err, &respError) && respError.ErrorCode == response.PermissionDeniedErrorCode {
				// Try to renew the session if the version of the session that
				// was used is still the current one.
				if _, err := c.session.Renew(ctx, c.password, renewIfVersionIsCurrent(v)); err != nil {
					return err
				}

				// Successful reauthentication. Retry request one more time.
				authAttempted = true
				continue
			}

			return err
		}

		break
	}

	return nil
}
