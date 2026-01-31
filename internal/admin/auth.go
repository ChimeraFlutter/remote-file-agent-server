package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Auth handles admin authentication
type Auth struct {
	adminPassword string
	sessions      *SessionManager
}

// NewAuth creates a new Auth instance
func NewAuth(adminPassword string, sessionTimeout time.Duration) *Auth {
	return &Auth{
		adminPassword: adminPassword,
		sessions:      NewSessionManager(sessionTimeout),
	}
}

// ValidatePassword checks if the provided password is correct
func (a *Auth) ValidatePassword(password string) bool {
	return password == a.adminPassword
}

// CreateSession creates a new session for an admin
func (a *Auth) CreateSession(ip string) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &Session{
		SessionID: sessionID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(a.sessions.timeout),
		IP:        ip,
	}

	a.sessions.Add(session)
	return sessionID, nil
}

// ValidateSession checks if a session is valid
func (a *Auth) ValidateSession(sessionID string) bool {
	return a.sessions.Validate(sessionID)
}

// DeleteSession removes a session
func (a *Auth) DeleteSession(sessionID string) {
	a.sessions.Delete(sessionID)
}

// GetSession retrieves a session
func (a *Auth) GetSession(sessionID string) (*Session, bool) {
	return a.sessions.Get(sessionID)
}

// generateSessionID generates a random session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
