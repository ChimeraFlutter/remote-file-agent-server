package middleware

import (
	"database/sql"
	"net/http"
	"runtime"
	"time"

	"go.uber.org/zap"
)

// DebugMiddleware logs detailed request information for debugging
func DebugMiddleware(logger *zap.Logger, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Log request start
			stats := db.Stats()
			goroutines := runtime.NumGoroutine()

			logger.Info("REQUEST START",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote", r.RemoteAddr),
				zap.Int("db_open_conns", stats.OpenConnections),
				zap.Int("db_in_use", stats.InUse),
				zap.Int("db_idle", stats.Idle),
				zap.Int("goroutines", goroutines),
			)

			// Call next handler
			next.ServeHTTP(w, r)

			// Log request end
			duration := time.Since(start)
			statsEnd := db.Stats()
			goroutinesEnd := runtime.NumGoroutine()

			logger.Info("REQUEST END",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", duration),
				zap.Int("db_open_conns", statsEnd.OpenConnections),
				zap.Int("db_in_use", statsEnd.InUse),
				zap.Int("db_idle", statsEnd.Idle),
				zap.Int("goroutines", goroutinesEnd),
				zap.Int("goroutine_delta", goroutinesEnd-goroutines),
			)
		})
	}
}
