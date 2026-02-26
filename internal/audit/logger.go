package audit

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Admin    string
	Action   string
	DeviceID string
	Path     string
	Result   string
	ErrorMsg string
	IP       string
}

// Logger handles audit logging
type Logger struct {
	db      *sql.DB
	logger  *zap.Logger
	logChan chan AuditEntry
}

// NewLogger creates a new audit logger
func NewLogger(db *sql.DB, logger *zap.Logger) *Logger {
	l := &Logger{
		db:      db,
		logger:  logger,
		logChan: make(chan AuditEntry, 1000), // Buffer up to 1000 log entries
	}

	// Start background worker to write logs
	go l.worker()

	return l
}

// worker processes audit log entries from the channel
func (l *Logger) worker() {
	for entry := range l.logChan {
		query := `
			INSERT INTO audit_logs (timestamp, admin, action, device_id, path, result, error, ip)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`

		timestamp := time.Now().Unix()

		_, err := l.db.Exec(query, timestamp, entry.Admin, entry.Action, entry.DeviceID, entry.Path, entry.Result, entry.ErrorMsg, entry.IP)
		if err != nil {
			l.logger.Error("Failed to write audit log",
				zap.Error(err),
				zap.String("admin", entry.Admin),
				zap.String("action", entry.Action),
			)
		}
	}
}

// Log records an audit log entry
func (l *Logger) Log(admin, action, deviceID, path, result, errorMsg, ip string) {
	// Log to application logger immediately (non-blocking)
	l.logger.Info("Audit log",
		zap.String("admin", admin),
		zap.String("action", action),
		zap.String("device_id", deviceID),
		zap.String("path", path),
		zap.String("result", result),
		zap.String("error", errorMsg),
		zap.String("ip", ip),
	)

	// Send to channel for async processing (non-blocking if buffer is full)
	select {
	case l.logChan <- AuditEntry{
		Admin:    admin,
		Action:   action,
		DeviceID: deviceID,
		Path:     path,
		Result:   result,
		ErrorMsg: errorMsg,
		IP:       ip,
	}:
		// Successfully queued
	default:
		// Channel full, log warning but don't block
		l.logger.Warn("Audit log channel full, dropping log entry")
	}
}

// GetLogs retrieves audit logs with optional filters
func (l *Logger) GetLogs(limit int, deviceID string) ([]AuditLog, error) {
	query := `
		SELECT id, timestamp, admin, action, device_id, path, result, error, ip
		FROM audit_logs
	`

	args := []interface{}{}

	if deviceID != "" {
		query += " WHERE device_id = ?"
		args = append(args, deviceID)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var timestamp int64
		var deviceIDVal, pathVal, errorVal sql.NullString

		err := rows.Scan(
			&log.ID,
			&timestamp,
			&log.Admin,
			&log.Action,
			&deviceIDVal,
			&pathVal,
			&log.Result,
			&errorVal,
			&log.IP,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		log.Timestamp = time.Unix(timestamp, 0)
		if deviceIDVal.Valid {
			log.DeviceID = deviceIDVal.String
		}
		if pathVal.Valid {
			log.Path = pathVal.String
		}
		if errorVal.Valid {
			log.Error = errorVal.String
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Admin     string    `json:"admin"`
	Action    string    `json:"action"`
	DeviceID  string    `json:"device_id,omitempty"`
	Path      string    `json:"path,omitempty"`
	Result    string    `json:"result"`
	Error     string    `json:"error,omitempty"`
	IP        string    `json:"ip"`
}
