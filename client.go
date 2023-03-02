package livebox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
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
	return &client{
		password: password,
		client:   c,
		session:  newSession(c),
	}
}

// client implements the Client interface.
type client struct {
	// HTTP client that will be used to send HTTP requests.
	client *http.Client
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

		// Send HTTP request
		resp, err := c.client.Do(r)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Verify HTTP Status. The 200 status code is always expected,
		// even when the session is expired.
		if resp.StatusCode != http.StatusOK {
			return ErrUnexpectedStatus
		}

		// Read all body.
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Handle eventual errors contained in the body.
		// Session renewal is handled here.
		if err := handleRequestError(b); err != nil {
			// If reauthentication was already attempted, return error now.
			if authAttempted {
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

		// No error, we can unmarshal the response body into the "out" parameter.
		if err := json.Unmarshal(b, out); err != nil {
			return err
		}

		break
	}

	return nil
}

// handleRequestError handles a JSON-encoded error contained in the body
// of an API response. If multiple errors are found, only the first error
// is returned. If there is no error, this function returns nil.
func handleRequestError(body []byte) error {
	// Unmarshal as an Error response.
	var respError response.Errors
	if err := json.Unmarshal(body, &respError); err != nil {
		return err
	}

	if len(respError.Errors) > 0 {
		// Only handle first error.
		return &respError.Errors[0]
	}

	return nil
}
