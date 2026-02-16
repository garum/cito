package handler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

type WebSocketService struct {
	upgrader websocket.Upgrader
}

func (webSocketService *WebSocketService) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := webSocketService.upgrader.Upgrade(w, r, nil)
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
