package admin

import (
	"net/http"
)

const (
	SessionCookieName = "rfm_session"
)

// AuthMiddleware creates an authentication middleware
func AuthMiddleware(auth *Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Validate session
			if !auth.ValidateSession(cookie.Value) {
				http.Error(w, `{"error":"session_expired"}`, http.StatusUnauthorized)
				return
			}

			// Session is valid, continue
			next.ServeHTTP(w, r)
		})
	}
}
