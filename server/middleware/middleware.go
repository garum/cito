package middleware

import (
	"log/slog"
	"net/http"
)

// Middleware function signature
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BEFORE the handler runs
		slog.Info("Request received", "method", r.Method, "path", r.URL.Path)

		// Call the next handler in the chain
		next.ServeHTTP(w, r)

		// AFTER the handler runs
		slog.Info("Response sent", "method", r.Method, "path", r.URL.Path)
	})
}
