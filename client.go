// Package livebox provides a client to easily communicate with Livebox 5's API.
//
// This API is usually available at `http://192.168.1.1/ws`. Authentication is
// handled by the library, set the `admin` password and start sending requests.
package livebox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
)

const (
	// All API requests are sent to this endpoint using the POST method.
	apiEndpoint = "http://192.168.1.1/ws"
	// HTTP request Content-Type. HTTP responses will also use this Content-Type.
	apiContentType = "application/x-sah-ws-4-call+json"
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
	// ErrInvalidPassword is returned when the login is not successful
	// because the password is invalid.
	ErrInvalidPassword = errors.New("invalid password")
	// ErrEmptyContextID is returned during login when authentication
	// is successful, but the server did not send a contextID.
	// In practice, you should not expect to see this error.
	ErrEmptyContextID = errors.New("received empty contextID")
	// ErrEmptySessidCookie is returned during login when authentication
	// is successful, but the server did not send a sessionID cookie.
	// In practice, you should not expect to see this error.
	ErrEmptySessidCookie = errors.New("did not receive sessid cookie")
	// ErrUnexpectedStatus is returned when the server does not return the
	// expected 200 status code.
	ErrUnexpectedStatus = errors.New("the server did not return the 200 status code")
)

// Client is a Livebox API Client. Requests sent using a client will
// be automatically authenticated. Client is currently NOT thread safe.
type Client interface {
	Request(ctx context.Context, req *request.Request, out interface{}) error
}

// NewClient returns a new Client that will be authenticated using the given
// password.
func NewClient(password string) Client {
	return &client{
		password: password,
		client:   &http.Client{},
	}
}

// NewClientWithHTTPClient returns a new Client that will be authenticated
// using the given password. The given HTTP client will be used to send
// HTTP requests.
func NewClientWithHTTPClient(password string, c *http.Client) Client {
	return &client{
		password: password,
		client:   c,
	}
}

// client implements the Client interface.
type client struct {
	// HTTP client that will be used to send HTTP requests.
	client *http.Client
	// Password of the "admin" user.
	password string
	// ContextID, used for authenticating requests.
	contextID string
	// Cookie that contains the sessid (with patched name), used for
	// authenticating requests
	sessid *http.Cookie
}

// Request sends a request to the Livebox API. If the client is not yet
// authenticated, or the session is expired, the client will try to
// authenticate using the admin password given during the creation
// of the client.
func (c *client) Request(ctx context.Context, req *request.Request, out interface{}) error {
	// Authenticate if the client is used for the first time.
	if c.contextID == "" || c.sessid == nil {
		if err := c.auth(ctx); err != nil {
			return err
		}
	}

	// Send the request, if the session is expired, the client will create
	// a new session.
	return c.request(ctx, req, out)
}

// newRequest creates a new HTTP Post request with the Content-Type that
// Livebox's API expects. An Authorization Header must be set, the
// authorization parameter cannot be empty.
func newRequest(ctx context.Context, body io.Reader, authorization string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiEndpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", apiContentType)
	req.Header.Set("Authorization", authorization)
	return req, err
}

// auth tries to authenticate to the API. If authentication is successful,
// session data (session ID cookie and context ID) is stored in the client.
// Otherwise, an error is returned.
func (c *client) auth(ctx context.Context) error {
	// Create payload
	payload, err := json.Marshal(request.NewLogin(c.password))
	if err != nil {
		return err
	}

	// Create request and send it
	req, err := newRequest(ctx, bytes.NewReader(payload), authorizationHeaderLogin)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Handle status errors, we expect status code 200 to continue.
	if res.StatusCode != http.StatusOK {
		// An invalid password causes a 401 status code.
		if res.StatusCode == http.StatusUnauthorized {
			return ErrInvalidPassword
		}
		return ErrUnexpectedStatus
	}

	// Read all body.
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// Handle an eventual error (an invalid password does not trigger an error).
	if err := handleRequestError(b); err != nil {
		return err
	}

	// Decode response and verify that it's valid.
	login := response.Login{}
	if err := json.Unmarshal(b, &login); err != nil {
		return err
	}
	if login.Data.ContextID == "" {
		return ErrEmptyContextID
	}

	// Find sessid cookie.
	cookie, ok := findSessidCookie(res)
	if !ok {
		return ErrEmptySessidCookie
	}

	// Save session data.
	c.contextID = login.Data.ContextID
	c.sessid = cookie

	return nil
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

// authorization returns the Authorization Header that contains the contextID.
func (c *client) authorization() string {
	return fmt.Sprintf("X-Sah %s", c.contextID)
}

// cookie returns the Cookie Header that contains the session ID.
func (c *client) cookie() string {
	return fmt.Sprintf("%s=%s", c.sessid.Name, c.sessid.Value)
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
		r, err := newRequest(ctx, bytes.NewReader(payload), c.authorization())
		if err != nil {
			return err
		}

		// Add Cookie for authentication (we add it as a raw value because the
		// cookie name is not HTTP/1.1 compliant).
		r.Header.Add("Cookie", c.cookie())

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
				// Try to reauthenticate.
				if err := c.auth(ctx); err != nil {
					return err
				}

				// Successful reauthentication.
				// Retry request one more time.
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
