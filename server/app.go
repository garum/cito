package main

import (
	"cito/server/middleware"
	"cito/server/model"
	"cito/server/service"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// HTTPClient interface for making HTTP requests (enables mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OAuth2TokenExchanger interface for OAuth2 token exchange (enables mocking)
type OAuth2TokenExchanger interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
}

type App struct {
	oauthConfig  OAuth2TokenExchanger
	db           *sql.DB
	userService  *service.UserService
	httpClient   HTTPClient
	githubAPIURL string
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

// fetchGitHubUser fetches user information from GitHub API
func (app *App) fetchGitHubUser(accessToken string) (*model.GitHubUser, error) {
	apiURL := app.githubAPIURL
	if apiURL == "" {
		apiURL = "https://api.github.com/user"
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	httpClient := app.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	slog.Info("client body gh:", "body", body)
	var githubUser model.GitHubUser
	if err := json.Unmarshal(body, &githubUser); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub user: %w", err)
	}

	if githubUser.Email == "" {
		githubUser.Email, err = app.fetchGitHubUserEmail(accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Email: %w", err)
		}
	}

	return &githubUser, nil
}

func (app *App) fetchGitHubUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	httpClient := app.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	slog.Info("client eamil body gh:", "email", body)
	type Email struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	var emails []Email

	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("failed to parse GitHub emails: %w", err)

	}
	slog.Info("client emails unmarsh body gh:", "emails", emails)

	for _, email := range emails {
		if email.Primary == true {
			return email.Email, nil
		}
	}
	return "", errors.New("No email found")

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

	// Fetch GitHub user information
	githubUser, err := app.fetchGitHubUser(tok.AccessToken)
	if err != nil {
		slog.Error("Failed to fetch GitHub user", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("GitHub user authenticated", "id", githubUser.ID, "login", githubUser.Login, "email", githubUser.Email)

	// Upsert user and get session token
	sessionToken, err := app.userService.UpsertUser(*githubUser, tok.AccessToken)
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

func (app *App) RegisterRoutes(handler *http.ServeMux) {

	checkAuthMiddleware := middleware.MakeAuthMiddleware(app.userService)

	// public
	handler.HandleFunc("/ws", app.handler)
	// auth handlers
	handler.Handle("/login", middleware.LoggingMiddleware(http.HandlerFunc(app.loginHandler)))
	handler.Handle("/oauth2/callback", middleware.LoggingMiddleware(http.HandlerFunc(app.callBackHandler)))

	// secure handlers
	handler.Handle("/", middleware.LoggingMiddleware(checkAuthMiddleware(http.HandlerFunc(app.homeHandler))))
}
