package main

import (
	"cito/server/handler"
	"cito/server/middleware"
	"cito/server/service"
	"database/sql"
	"net/http"
)

// HTTPClient interface for making HTTP requests (enables mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type App struct {
	userService      *service.UserService
	authService      *service.AuthService
	oauthHandler     *handler.OAuthHandler
	webSocketHandler *handler.WebSocketHandler
}

func NewApp(oauthConfig service.OAuth2TokenExchanger, db *sql.DB) *App {
	userService := service.NewUserService(db)
	authService := service.NewAuthService(oauthConfig, &http.Client{})
	oauthHandler := handler.NewOAuthHandler(authService, userService)
	webSocketHandler := handler.NewWebSocketHandler()
	return &App{
		userService:      userService,
		authService:      authService,
		oauthHandler:     oauthHandler,
		webSocketHandler: webSocketHandler,
	}
}

func (app *App) RegisterRoutes(mux *http.ServeMux) {

	checkAuthMiddleware := middleware.MakeAuthMiddleware(app.userService)

	// public
	mux.HandleFunc("/ws", app.webSocketHandler.Handler)
	// auth handlers
	mux.Handle("/login", middleware.LoggingMiddleware(http.HandlerFunc(app.oauthHandler.LoginHandler)))
	mux.Handle("/oauth2/callback", middleware.LoggingMiddleware(http.HandlerFunc(app.oauthHandler.CallBackHandler)))

	// secure handlers
	mux.Handle("/", middleware.LoggingMiddleware(checkAuthMiddleware(http.HandlerFunc(handler.HomeHandler))))
}
