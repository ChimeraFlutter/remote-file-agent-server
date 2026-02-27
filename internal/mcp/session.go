package mcp

import (
	"sync"
	"time"

	"github.com/remote-file-manager/server/internal/devices"
)

// Session represents an MCP session
type Session struct {
	SessionID      string
	SelectedDevice *devices.Device
	CreatedAt      time.Time
	LastActivity   time.Time
	ResponseChan   chan []byte
	mu             sync.RWMutex
}

// UpdateActivity updates the last activity time
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// GetSelectedDevice returns the selected device (thread-safe)
func (s *Session) GetSelectedDevice() *devices.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SelectedDevice
}

// SetSelectedDevice sets the selected device (thread-safe)
func (s *Session) SetSelectedDevice(device *devices.Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedDevice = device
}

// SessionManager manages MCP sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// GetOrCreate gets an existing session or creates a new one
func (sm *SessionManager) GetOrCreate(sessionID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.UpdateActivity()
		return session
	}

	session := &Session{
		SessionID:    sessionID,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ResponseChan: make(chan []byte, 100),
	}

	sm.sessions[sessionID] = session
	return session
}

// Get retrieves a session by ID
func (sm *SessionManager) Get(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if exists {
		session.UpdateActivity()
	}
	return session, exists
}

// Delete removes a session
func (sm *SessionManager) Delete(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		close(session.ResponseChan)
		delete(sm.sessions, sessionID)
	}
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for sessionID, session := range sm.sessions {
			if now.Sub(session.LastActivity) > sm.timeout {
				close(session.ResponseChan)
				delete(sm.sessions, sessionID)
			}
		}
		sm.mu.Unlock()
	}
}
