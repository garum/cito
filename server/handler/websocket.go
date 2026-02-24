package handler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	upgrader websocket.Upgrader
}

func NewWebSocketHandler() *WebSocketHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return &WebSocketHandler{upgrader: upgrader}
}

func (webSocketService *WebSocketHandler) Handler(w http.ResponseWriter, r *http.Request) {
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
