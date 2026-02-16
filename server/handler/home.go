package handler

import (
	"cito/server/model"
	"fmt"
	"log/slog"
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Home page accessed")
	// User is logged in - show greeting
	user, ok := model.GetUserValueFromContext(r.Context())
	slog.Info("Ger Values ", "user", user, "ok", ok)

	if !ok {
		slog.Info("redirect to login")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	html := fmt.Sprintf(`<h1>Hello, %s!</h1><p>You are logged in.</p>`, user.Username)
	w.Write([]byte(html))
}
