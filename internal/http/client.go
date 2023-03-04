package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Tomy2e/livebox-api-client/api/response"
)

// Client is an internal HTTP client with error handling and data parsing.
type Client struct {
	client *http.Client
}

// NewClient returns a new internal HTTP client.
func NewClient(client *http.Client) *Client {
	return &Client{client}
}

// SendRequest sends an HTTP request and unmarshals the response into the out object.
// It will return an error if the status of the response is not 200 or if an
// error is found in the body of the request. The HTTP response is returned,
// its body is already closed.
func (c *Client) SendRequest(ctx context.Context, req *http.Request, out interface{}) (*http.Response, error) {
	res, err := c.client.Do(req)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return res, response.NewStatusError(res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return res, err
	}

	if err := handleRequestError(b); err != nil {
		return res, err
	}

	// No error, we can unmarshal the response body into the "out" parameter.
	if err := json.Unmarshal(b, out); err != nil {
		return res, err
	}

	return res, nil
}

// handleRequestError handles a JSON-encoded error contained in the body
// of an API response. If multiple errors are found, only the first error
// is returned. If there is no error, this function returns nil.
func handleRequestError(body []byte) error {
	// Unmarshal as an Error response (single error).
	var respError response.Error
	if err := json.Unmarshal(body, &respError); err != nil {
		return err
	}

	if respError.ErrorCode != 0 {
		return &respError
	}

	// Unmarshal as an Errors response (multiple errors).
	var respErrors response.Errors
	if err := json.Unmarshal(body, &respErrors); err != nil {
		return err
	}

	if len(respErrors.Errors) > 0 {
		// Only handle first error.
		return &respErrors.Errors[0]
	}

	return nil
}
