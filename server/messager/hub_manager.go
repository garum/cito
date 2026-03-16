package messager

import (
	"cito/server/model"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type HubManager struct {
	clients  map[int]*websocket.Conn
	messages chan model.Message
	mu       sync.Mutex
}

func NewHubManager() *HubManager {
	return &HubManager{clients: make(map[int]*websocket.Conn)}
}

func (h *HubManager) Register(clientId int, con *websocket.Conn) {
	h.mu.Lock()
	h.clients[clientId] = con
	h.mu.Unlock()
}

func (h *HubManager) Unregister(clientId int) {
	h.mu.Lock()
	delete(h.clients, clientId)
	h.mu.Unlock()

}

// Process all the messages receive in messages channel
func (h *HubManager) Run() {

	for message := range h.messages {
		h.mu.Lock()
		conn, ok := h.clients[message.ToUserId]
		h.mu.Unlock()
		if !ok {
			slog.Error("No websocket connection found:", "ToUserId", message.ToUserId)
			continue
		}
		// Write the message to the websocket
		byteMessage, err := json.Marshal(message)
		if err != nil {
			slog.Error("Marshal message :", "err", err)
			continue
		}
		err = conn.WriteMessage(websocket.TextMessage, byteMessage)

		if err != nil {
			slog.Error("Marshal message :", "err", err)
			continue
		}
	}
}

func (h *HubManager) HandelConnection(clientId int, conn *websocket.Conn) {
	h.Register(clientId, conn)
	defer h.Unregister(clientId)
	defer conn.Close()

	for {
		// read messsage
		_, recvBytes, err := conn.ReadMessage()
		if err != nil {
			slog.Error("WebSocket read error", "error", err)
			return
		}

		var message model.Message
		err = json.Unmarshal(recvBytes, &message)
		if err != nil {
			slog.Error("Unmarshal message", "error", err)
		}

		slog.Info("WebSocket message received", "message", message)
		message.FromUserID = clientId
		message.Time = time.Now()
		// send messages
		h.messages <- message
		// echo in console
	}
}

func (h *HubManager) AddMessage(message model.Message) {
	h.messages <- message
}
