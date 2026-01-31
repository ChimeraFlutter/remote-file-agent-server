package objects

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// TokenManager manages download tokens
type TokenManager struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewTokenManager creates a new token manager
func NewTokenManager(db *sql.DB, logger *zap.Logger) *TokenManager {
	return &TokenManager{
		db:     db,
		logger: logger,
	}
}

// GenerateToken generates a new download token for an object
func (tm *TokenManager) GenerateToken(objectID string, expiresIn time.Duration) (*DownloadToken, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	// Create token record
	token := &DownloadToken{
		Token:     tokenStr,
		ObjectID:  objectID,
		ExpiresAt: time.Now().Add(expiresIn),
		CreatedAt: time.Now(),
	}

	// Insert into database
	query := `
		INSERT INTO download_tokens (token, object_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := tm.db.Exec(query, token.Token, token.ObjectID, token.ExpiresAt, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert token: %w", err)
	}

	tm.logger.Debug("Download token generated",
		zap.String("object_id", objectID),
		zap.Time("expires_at", token.ExpiresAt))

	return token, nil
}

// ValidateToken validates a download token
func (tm *TokenManager) ValidateToken(tokenStr string) (*DownloadToken, error) {
	// Query token
	query := `
		SELECT token, object_id, expires_at, created_at
		FROM download_tokens
		WHERE token = ?
	`

	token := &DownloadToken{}

	err := tm.db.QueryRow(query, tokenStr).Scan(
		&token.Token, &token.ObjectID, &token.ExpiresAt, &token.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid token")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query token: %w", err)
	}

	// Check if token has expired
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	tm.logger.Debug("Download token validated",
		zap.String("token", tokenStr),
		zap.String("object_id", token.ObjectID))

	return token, nil
}

// CleanupExpiredTokens removes expired tokens from the database
func (tm *TokenManager) CleanupExpiredTokens() error {
	query := `DELETE FROM download_tokens WHERE expires_at < ?`
	result, err := tm.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired tokens: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows > 0 {
		tm.logger.Info("Cleaned up expired tokens", zap.Int64("count", rows))
	}

	return nil
}

// RevokeToken revokes a download token
func (tm *TokenManager) RevokeToken(tokenStr string) error {
	query := `DELETE FROM download_tokens WHERE token = ?`
	result, err := tm.db.Exec(query, tokenStr)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("token not found")
	}

	tm.logger.Debug("Download token revoked", zap.String("token", tokenStr))

	return nil
}

// GetTokensForObject retrieves all tokens for an object
func (tm *TokenManager) GetTokensForObject(objectID string) ([]*DownloadToken, error) {
	query := `
		SELECT token, object_id, expires_at, created_at, used_at
		FROM download_tokens
		WHERE object_id = ?
		ORDER BY created_at DESC
	`

	rows, err := tm.db.Query(query, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*DownloadToken
	for rows.Next() {
		token := &DownloadToken{}
		var usedAt sql.NullTime

		err := rows.Scan(&token.Token, &token.ObjectID, &token.ExpiresAt, &token.CreatedAt, &usedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan token: %w", err)
		}

		if usedAt.Valid {
			token.UsedAt = &usedAt.Time
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}
