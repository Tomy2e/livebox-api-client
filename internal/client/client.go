package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
)

const (
	// All API requests are sent to this endpoint using the POST method.
	apiEndpoint = "ws"
	// Value of the Authorization HTTP Header during the login request.
	authorizationHeaderLogin = "X-Sah-Login"
	// Suffix of the name of the cookie that contains the session ID. The
	// Livebox software violates HTTP/1.1 specs by using the "/" character for
	// the session ID cookie name. The cookie is received during login and sent in
	// subsequent requests.
	sessidCookieNameSuffix = "/sessid"
	// When parsing cookies, Go's HTTP library silently ignores cookies that
	// use non-compliant names. We need to replace the non-compliant cookie name
	// with a compliant cookie name.
	patchedSessidCookieNameSuffix = "_sessid"
)

var (
	// ErrInvalidCredentials is returned when the login is not successful
	// because the login or password is invalid.
	ErrInvalidCredentials = errors.New("invalid login or password")
	// ErrEmptyContextID is returned during login when authentication
	// is successful, but the server did not send a contextID.
	// In practice, you should not expect to see this error.
	ErrEmptyContextID = errors.New("received empty contextID")
	// ErrEmptySessidCookie is returned during login when authentication
	// is successful, but the server did not send a sessionID cookie.
	// In practice, you should not expect to see this error.
	ErrEmptySessidCookie = errors.New("did not receive sessid cookie")
	// ErrStatusError is returned if an unexpected status code was received.
	ErrStatusError = errors.New("status error")
)

type ContentType string

const (
	// ContentTypeWS is used for request calls.
	ContentTypeWS ContentType = "application/x-sah-ws-4-call+json"
	// ContentTypeEvent is used for event calls.
	ContentTypeEvent ContentType = "application/x-sah-event-4-call+json"
)

// Client is a low level client to send requests to the Livebox. It handles
// authentication.
type Client struct {
	// HTTP client to use to send requests.
	client *http.Client
	// Address where to send API requests.
	address string
	// Livebox username.
	username string
	// Livebox password.
	password string
	// Session data.
	session session
	// Makes sure there is at most one authentication attempt running in parallel.
	mu sync.Mutex
}

// New returns a new low level client.
func New(client *http.Client, address, username, password string) (*Client, error) {
	if client == nil {
		client = http.DefaultClient
	}

	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse livebox address: %w", err)
	}

	if u.Scheme == "" {
		return nil, errors.New("scheme is missing in livebox address")
	}

	u.Path = apiEndpoint

	return &Client{
		client:   client,
		address:  u.String(),
		username: username,
		password: password,
	}, nil
}

// Request sends a request with the provided contentType. The "in" object will be
// marshalled to json. The response will be unmarshalled into the "out" object.
func (c *Client) Request(ctx context.Context, contentType ContentType, in, out any) error {
	// Authenticate the first request.
	if _, _, v := c.session.GetCredentials(); v == 0 {
		if _, err := c.authenticate(ctx, v); err != nil {
			return err
		}
	}

	// Create request payload
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}

	authAttempted := false

	for {
		// Create HTTP request with request payload
		r, v, err := c.newAuthenticatedRequest(ctx, contentType, bytes.NewReader(payload))
		if err != nil {
			return err
		}

		if _, err := c.doRequest(r, out); err != nil { //nolint:bodyclose // Already closed.
			// If reauthentication was already attempted, return error now.
			if authAttempted {
				return err
			}

			// Check if the server returned a permission denied error.
			if response.IsPermissionDeniedError(err) {
				// Try to renew the session if the version of the session that
				// was used is still the current one.
				if authAttempted, err = c.authenticate(ctx, v); err != nil {
					return err
				}

				continue
			}

			return err
		}

		break
	}

	return nil
}

func (c *Client) newAuthenticatedRequest(ctx context.Context, contentType ContentType, body io.Reader) (*http.Request, uint64, error) {
	authorization, cookie, version := c.session.GetCredentials()

	req, err := newRequest(ctx, contentType, c.address, body, authorization)
	if err != nil {
		return nil, 0, err
	}

	// Add Cookie for authentication (we add it as a raw value because the
	// cookie name is not HTTP/1.1 compliant).
	req.Header.Add("Cookie", cookie)

	return req, version, nil
}

// doRequest sends an HTTP request and unmarshals the response into the out object.
// It will return an error if the status of the response is not 200 or if an
// error is found in the body of the request. The HTTP response is returned,
// its body is already closed.
func (c *Client) doRequest(req *http.Request, out interface{}) (*http.Response, error) {
	res, err := c.client.Do(req)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return res, fmt.Errorf("%w: got %d, expected 200", ErrStatusError, res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return res, err
	}

	// Fix for some event requests that contain a trailing "null" string.
	b = bytes.TrimSuffix(b, []byte("null"))

	if err := handleRequestError(b); err != nil {
		return res, err
	}

	// No error, we can unmarshal the response body into the "out" parameter.
	if err := json.Unmarshal(b, out); err != nil {
		return res, err
	}

	return res, nil
}

func (c *Client) authenticate(ctx context.Context, currentVersion uint64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, _, v := c.session.GetCredentials(); v != currentVersion {
		return false, nil
	}

	// Create payload
	payload, err := json.Marshal(request.NewLogin(c.username, c.password))
	if err != nil {
		return true, err
	}

	// Create request and send it
	req, err := newRequest(ctx, ContentTypeWS, c.address, bytes.NewReader(payload), authorizationHeaderLogin)
	if err != nil {
		return true, err
	}

	login := response.Login{}

	res, err := c.doRequest(req, &login) //nolint:bodyclose // Already closed.
	if err != nil {
		if errors.Is(err, ErrStatusError) && res.StatusCode == http.StatusUnauthorized {
			return true, ErrInvalidCredentials
		}

		return true, err
	}

	if login.Data.ContextID == "" {
		return true, ErrEmptyContextID
	}

	// Find sessid cookie.
	cookie, ok := findSessidCookie(res)
	if !ok {
		return true, ErrEmptySessidCookie
	}

	// Save session data and increment the current version of the session.
	c.session.SetCredentials(login.Data.ContextID, cookie)

	return true, nil
}

// findSessidCookie searches the sessid Cookie in the login response.
// If the cookie is not found, the first return value is nil and the second
// return value is false.
func findSessidCookie(res *http.Response) (*http.Cookie, bool) {
	// Cookie sent by the server contains an invalid character in its name ("/").
	// Go refuses to parse it. We must manually patch the name of the cookie for
	// decoding purposes.
	for i, v := range res.Header["Set-Cookie"] {
		if strings.Contains(v, sessidCookieNameSuffix) {
			res.Header["Set-Cookie"][i] = strings.Replace(v, sessidCookieNameSuffix, patchedSessidCookieNameSuffix, 1)
			break
		}
	}

	// Find the cookie using the patched name.
	for _, c := range res.Cookies() {
		if strings.HasSuffix(c.Name, patchedSessidCookieNameSuffix) {
			// Set the cookie to its original name.
			c.Name = strings.Replace(c.Name, patchedSessidCookieNameSuffix, sessidCookieNameSuffix, 1)
			return c, true
		}
	}

	return nil, false
}

// handleRequestError handles a JSON-encoded error contained in the body
// of an API response. If there is no error, this function returns nil.
func handleRequestError(body []byte) error {
	// Exit early if body doesn't contain the "error" field.
	if !strings.Contains(string(body), `"error"`) {
		return nil
	}

	// Unmarshal as an Error response (single error).
	var respError response.Error
	if err := json.Unmarshal(body, &respError); err != nil {
		return err
	}

	if respError.ErrorCode != 0 || respError.Description != "" || respError.Info != "" {
		return &respError
	}

	// Unmarshal as an Errors response (multiple errors).
	var respErrors response.Errors
	if err := json.Unmarshal(body, &respErrors); err != nil {
		return err
	}

	if len(respErrors.Errors) > 0 {
		return &respErrors
	}

	return nil
}

// newRequest creates a new HTTP Post request with the Content-Type that
// Livebox's API expects. An Authorization Header must be set, the
// authorization parameter cannot be empty.
func newRequest(ctx context.Context, contentType ContentType, url string, body io.Reader, authorization string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", string(contentType))

	if authorization != "X-Sah-Login" {
		req.Header.Set("x-context", strings.Split(authorization, " ")[1])
	} else {
		req.Header.Set("Authorization", authorization)
	}

	return req, err
}
