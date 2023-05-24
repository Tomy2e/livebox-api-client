package livebox

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
	internalHTTP "github.com/Tomy2e/livebox-api-client/internal/http"
)

const (
	// All API requests are sent to this endpoint using the POST method.
	apiEndpoint = "ws"
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
)

var (
	// errSessionNotInitialized is returned when trying to use a session that
	// has never been renewed.
	errSessionNotInitialized = errors.New("the session has never been renewed")
)

// session is thread-safe store for session data. Only the exported methods
// should be called to guarantee thread safety.
type session struct {
	// HTTP client used to send authentication requests.
	client *internalHTTP.Client
	// Address where to send API requests.
	address string
	// ContextID used for creating authenticated requests.
	contextID string
	// Cookie that contains the sessid (with patched name), used for
	// creating authenticated requests.
	sessid *http.Cookie
	// Current version of the session. It is incremented each time the session
	// is successfully renewed.
	version uint64
	// Lock to prevent concurrent access.
	lock sync.RWMutex
}

// renewCondition defines a callback function to control the renewal of the
// session. As the callback function is called in a thread-safe context,
// the session fields can be accessed/updated.
type renewCondition func(*session) bool

// renewIfNotInitialized allows to renew the session only if the session has
// not been initialized yet.
var renewIfNotInitialized renewCondition = func(s *session) bool {
	return s.version == 0
}

// renewIfVersionIsCurrent allows to renew the session only if the provided
// version is the current version of the session.
func renewIfVersionIsCurrent(version uint64) renewCondition {
	return func(s *session) bool {
		return version == s.version
	}
}

// newSession returns a new session with the provided HTTP client and livebox address.
func newSession(c *internalHTTP.Client, liveboxAddress string) (*session, error) {
	u, err := url.Parse(liveboxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse livebox address: %w", err)
	}

	u.Path = apiEndpoint

	return &session{
		client:  c,
		address: u.String(),
	}, nil
}

// Renew tries to renew the current session by authenticating to the Livebox API.
// If authentication is successful, session data (session ID cookie and context ID)
// is stored in the client and true is returned. If the session was not renewed
// because the renew condition was false, it returns false. Otherwise, an error
// is returned.
func (s *session) Renew(ctx context.Context, password string, rc renewCondition) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !rc(s) {
		return false, nil
	}

	// Create payload
	payload, err := json.Marshal(request.NewLogin(password))
	if err != nil {
		return false, err
	}

	// Create request and send it
	req, err := newRequest(ctx, s.address, bytes.NewReader(payload), authorizationHeaderLogin)
	if err != nil {
		return false, err
	}

	login := response.Login{}

	res, err := s.client.SendRequest(ctx, req, &login)
	if err != nil {
		if response.IsStatusErrorUnauthorized(err) {
			return false, ErrInvalidPassword
		}

		return false, err
	}

	if login.Data.ContextID == "" {
		return false, ErrEmptyContextID
	}

	// Find sessid cookie.
	cookie, ok := findSessidCookie(res)
	if !ok {
		return false, ErrEmptySessidCookie
	}

	// Save session data and increment the current version of the session.
	s.contextID = login.Data.ContextID
	s.sessid = cookie
	s.version++

	return true, nil
}

// authorization returns the Authorization Header that contains the contextID.
func (s *session) authorization() string {
	return fmt.Sprintf("X-Sah %s", s.contextID)
}

// cookie returns the Cookie Header that contains the session ID.
func (s *session) cookie() string {
	return fmt.Sprintf("%s=%s", s.sessid.Name, s.sessid.Value)
}

// NewAuthenticatedRequest returns an authenticated request to the Livebox API.
// It also returns the current version of the session that was used to create
// the request. The session must have been renewed successfully at least once
// before calling this method.
func (s *session) NewAuthenticatedRequest(ctx context.Context, body io.Reader) (*http.Request, uint64, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.version == 0 {
		return nil, 0, errSessionNotInitialized
	}

	req, err := newRequest(ctx, s.address, body, s.authorization())
	if err != nil {
		return nil, 0, err
	}

	// Add Cookie for authentication (we add it as a raw value because the
	// cookie name is not HTTP/1.1 compliant).
	req.Header.Add("Cookie", s.cookie())

	return req, s.version, nil
}

// newRequest creates a new HTTP Post request with the Content-Type that
// Livebox's API expects. An Authorization Header must be set, the
// authorization parameter cannot be empty.
func newRequest(ctx context.Context, url string, body io.Reader, authorization string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", apiContentType)
	req.Header.Set("Authorization", authorization)

	return req, err
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
