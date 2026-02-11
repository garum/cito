package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

type App struct {
	oauthConfig *oauth2.Config
	db          *sql.DB
	userService *UserService
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (app *App) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()
	for {
		// read messsage
		_, p, err := conn.ReadMessage()
		if err != nil {
			slog.Error("WebSocket read error", "error", err)
			return
		}
		// echo in console
		slog.Info("WebSocket message received", "message", string(p[:]))
	}
}

func (app *App) loginHandler(w http.ResponseWriter, r *http.Request) {

	url := app.oauthConfig.AuthCodeURL("state")
	slog.Info("OAuth URL generated", "url", url)
	html := fmt.Sprintf(`<a href="%s">Sign in with GitHub</a>`, url)
	w.Write([]byte(html))
}

func (app *App) homeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Home page accessed")

	// Check for session cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		// No session cookie - show login link
		http.Redirect(w, r, "/login", http.StatusMovedPermanently)
		return
	}

	// Look up user by session token
	user, err := app.userService.FindUserBySession(cookie.Value)
	if err != nil {
		// Invalid session - show login link
		slog.Warn("Invalid session", "error", err)
		http.Redirect(w, r, "/login", http.StatusMovedPermanently)
		return
	}

	// User is logged in - show greeting
	html := fmt.Sprintf(`<h1>Hello, %s!</h1><p>You are logged in.</p>`, user.Username)
	w.Write([]byte(html))
}

func (app *App) callBackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	slog.Info("OAuth callback received", "code", code)
	tok, err := app.oauthConfig.Exchange(context.TODO(), code)
	if err != nil {
		slog.Error("OAuth exchange error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// get user information from GitHub
	cli := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bearToken := "Bearer " + tok.AccessToken
	req.Header.Add("Authorization", bearToken)

	resp, err := cli.Do(req)
	if err != nil {
		slog.Error("Failed to fetch user info", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse GitHub user response
	var githubUser GitHubUser
	if err := json.Unmarshal(body, &githubUser); err != nil {
		slog.Error("Failed to parse GitHub user", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("GitHub user authenticated", "id", githubUser.ID, "login", githubUser.Login, "email", githubUser.Email)

	// Upsert user and get session token
	sessionToken, err := app.userService.UpsertUser(githubUser, tok.AccessToken)
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

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
