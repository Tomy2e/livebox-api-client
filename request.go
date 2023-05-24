package livebox

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
)

// Request sends a request to the Livebox API. If the client is not yet
// authenticated, or the session is expired, the client will try to
// authenticate using the admin password given during the creation
// of the client.
func (c *Client) Request(ctx context.Context, req *request.Request, out interface{}) error {
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
func (c *Client) request(ctx context.Context, req *request.Request, out interface{}) error {
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
			// If reauthentication was already attempted, return error now.
			if authAttempted {
				return err
			}

			// Check if the server returned a permission denied error.
			if response.IsPermissionDeniedError(err) {
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
