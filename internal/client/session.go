package client

import (
	"fmt"
	"net/http"
	"sync"
)

type session struct {
	// mu guards the following fields.
	mu sync.RWMutex
	// ContextID used for creating authenticated requests.
	contextID string
	// Cookie that contains the sessid (with patched name), used for creating
	// authenticated requests.
	sessid *http.Cookie
	// Current version of the session. It is incremented each time the session
	// is successfully renewed.
	version uint64
}

// GetCredentials returns the current credentials and their version.
func (s *session) GetCredentials() (authorization, cookie string, version uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.version == 0 {
		return "", "", 0
	}

	return fmt.Sprintf("X-Sah %s", s.contextID), fmt.Sprintf("%s=%s", s.sessid.Name, s.sessid.Value), s.version
}

// SetCredentials sets the current credentials and bumps the version.
func (s *session) SetCredentials(contextID string, sessid *http.Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.contextID = contextID
	s.sessid = sessid
	s.version++
}
