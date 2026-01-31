package audit

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Logger handles audit logging
type Logger struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewLogger creates a new audit logger
func NewLogger(db *sql.DB, logger *zap.Logger) *Logger {
	return &Logger{
		db:     db,
		logger: logger,
	}
}

// Log records an audit log entry
func (l *Logger) Log(admin, action, deviceID, path, result, errorMsg, ip string) {
	query := `
		INSERT INTO audit_logs (timestamp, admin, action, device_id, path, result, error, ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	timestamp := time.Now().Unix()

	_, err := l.db.Exec(query, timestamp, admin, action, deviceID, path, result, errorMsg, ip)
	if err != nil {
		l.logger.Error("Failed to write audit log",
			zap.Error(err),
			zap.String("admin", admin),
			zap.String("action", action),
		)
	}

	// Also log to application logger
	l.logger.Info("Audit log",
		zap.String("admin", admin),
		zap.String("action", action),
		zap.String("device_id", deviceID),
		zap.String("path", path),
		zap.String("result", result),
		zap.String("error", errorMsg),
		zap.String("ip", ip),
	)
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
