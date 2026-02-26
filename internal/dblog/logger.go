package dblog

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"
)

// DB wraps sql.DB with query logging
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// Wrap creates a new logged DB wrapper
func Wrap(db *sql.DB, logger *zap.Logger) *DB {
	return &DB{
		DB:     db,
		logger: logger,
	}
}

// Exec logs and executes a query
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := db.DB.Exec(query, args...)
	duration := time.Since(start)

	db.logger.Debug("SQL EXEC",
		zap.String("query", query),
		zap.Duration("duration", duration),
		zap.Error(err),
	)

	if duration > 100*time.Millisecond {
		db.logger.Warn("SLOW QUERY",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return result, err
}

// Query logs and executes a query
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := db.DB.Query(query, args...)
	duration := time.Since(start)

	db.logger.Debug("SQL QUERY",
		zap.String("query", query),
		zap.Duration("duration", duration),
		zap.Error(err),
	)

	if duration > 100*time.Millisecond {
		db.logger.Warn("SLOW QUERY",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return rows, err
}

// QueryRow logs and executes a query
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := db.DB.QueryRow(query, args...)
	duration := time.Since(start)

	db.logger.Debug("SQL QUERY ROW",
		zap.String("query", query),
		zap.Duration("duration", duration),
	)

	if duration > 100*time.Millisecond {
		db.logger.Warn("SLOW QUERY",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return row
}

// QueryContext logs and executes a query with context
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := db.DB.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	db.logger.Debug("SQL QUERY CONTEXT",
		zap.String("query", query),
		zap.Duration("duration", duration),
		zap.Error(err),
	)

	if duration > 100*time.Millisecond {
		db.logger.Warn("SLOW QUERY",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return rows, err
}
