package livebox

import (
	"context"
	"log/slog"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/internal/client"
)

// Request sends a request to the Livebox API. If the client is not yet
// authenticated, or the session is expired, the client will try to
// authenticate using the admin password given during the creation
// of the client.
func (c *Client) Request(ctx context.Context, req *request.Request, out any) error {
	err := c.client.Request(ctx, client.ContentTypeWS, req, out)
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to send request to Livebox", slog.Any("error", err))
	} else {
		c.log.InfoContext(ctx, "Sent request to Livebox", slog.Any("request", req))
	}
	return err
}
