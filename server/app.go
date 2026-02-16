package main

import (
	"cito/server/middleware"
	"cito/server/service"
	"database/sql"
	"net/http"

	"github.com/gorilla/websocket"
)

// HTTPClient interface for making HTTP requests (enables mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type App struct {
	oauthConfig OAuth2TokenExchanger
	db          *sql.DB
	userService *service.UserService
	httpClient  HTTPClient
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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
