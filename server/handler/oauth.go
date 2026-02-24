package handler

import (
	"cito/server/service"
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

type OAuthHandler struct {
	authService service.AuthService
	userService service.UserService
}

func NewOAuthHandler(authService *service.AuthService, userService *service.UserService) *OAuthHandler {
	return &OAuthHandler{authService: *authService, userService: *userService}
}

func (oauthHandler *OAuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("login page")
	url := oauthHandler.authService.GetLoginURL()
	slog.Info("OAuth URL generated", "url", url)
	html := fmt.Sprintf(`<a href="%s">Sign in with GitHub</a>`, url)
	w.Write([]byte(html))
}

func (oauthHandler *OAuthHandler) CallBackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	slog.Info("OAuth callback received", "code", code)
	tok, err := oauthHandler.authService.GetGHToken(context.TODO(), code)
	if err != nil {
		slog.Error("OAuth exchange error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Fetch GitHub user information
	githubUser, err := oauthHandler.authService.FetchGitHubUser(tok.AccessToken)
	if err != nil {
		slog.Error("Failed to fetch GitHub user", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("GitHub user authenticated", "id", githubUser.ID, "login", githubUser.Login, "email", githubUser.Email)

	// Upsert user and get session token
	sessionToken, err := oauthHandler.userService.UpsertUser(*githubUser, tok.AccessToken)
	if err != nil {
		slog.Error("Failed to upsert user", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}
