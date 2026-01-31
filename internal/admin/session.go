package admin

import (
	"sync"
	"time"
)

// Session represents an admin session
type Session struct {
	SessionID string
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
}

// SessionManager manages admin sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionManager creates a new SessionManager
func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// Add adds a new session
func (sm *SessionManager) Add(session *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[session.SessionID] = session
}

// Get retrieves a session
func (sm *SessionManager) Get(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.sessions[sessionID]
	return session, ok
}

// Validate checks if a session is valid
func (sm *SessionManager) Validate(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		return false
	}

	return true
}

// Delete removes a session
func (sm *SessionManager) Delete(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for sessionID, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, sessionID)
			}
		}
		sm.mu.Unlock()
	}
}
