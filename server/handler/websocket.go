package handler

import (
	"cito/server/messager"
	"cito/server/model"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	upgrader websocket.Upgrader
	hub      *messager.HubManager
}

func NewWebSocketHandler() *WebSocketHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hubManager := messager.NewHubManager()

	go hubManager.Run()

	return &WebSocketHandler{upgrader: upgrader, hub: hubManager}
}

func (webSocketService *WebSocketHandler) Handler(w http.ResponseWriter, r *http.Request) {

	user, ok := model.GetUserValueFromContext(r.Context())
	if !ok {
		slog.Info("redirect to login")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	conn, err := webSocketService.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	slog.Info("user info", "user", user)

	go webSocketService.hub.HandelConnection(user.ID, conn)

	// defer conn.Close()
	// for {
	// 	// read messsage
	// 	_, p, err := conn.ReadMessage()
	// 	if err != nil {
	// 		slog.Error("WebSocket read error", "error", err)
	// 		return
	// 	}
	// 	// echo in console
	// 	slog.Info("WebSocket message received", "message", string(p[:]))
	// }
}
