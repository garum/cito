package middleware

import (
	"cito/server/model"
	"cito/server/service"
	"log/slog"
	"net/http"
)

func MakeAuthMiddleware(userService *service.UserService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for session cookie
			cookie, err := r.Cookie("session_token")
			if err != nil {
				// No session cookie - show login link
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Look up user by session token
			user, err := userService.FindUserBySession(cookie.Value)
			if err != nil || user == nil {
				// Invalid session - show login link
				slog.Warn("Invalid session", "error", err)
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			slog.Info("Auth check success ", "user", user)
			ctx := model.NewContextWithUserValue(r.Context(), user)
			user2, ok := model.GetUserValueFromContext(ctx)
			slog.Info(" got user from context", "user2", user2, "ok", ok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

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
