package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var _ = &Hub{} // suppress unused lint; imported by other packages

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client represents a WebSocket client
type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
	Hub  *Hub
}

// Hub manages WebSocket connections
type Hub struct {
	clients     map[*Client]bool
	clientsByID map[string]*Client
	broadcast   chan []byte
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
}

// Message represents a WebSocket message
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Room    string      `json:"room,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		clientsByID: make(map[string]*Client),
		broadcast:   make(chan []byte),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.clientsByID[client.ID] = client
			h.mu.Unlock()
			log.Printf("Client connected: %s", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.clientsByID, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected: %s", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// SendToClient sends a message to a specific client
func (h *Hub) SendToClient(clientID string, message []byte) {
	h.mu.RLock()
	client, ok := h.clientsByID[clientID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case client.Send <- message:
	default:
		h.mu.Lock()
		delete(h.clients, client)
		delete(h.clientsByID, client.ID)
		close(client.Send)
		h.mu.Unlock()
	}
}

// GetConnectedClients returns the number of connected clients
func (h *Hub) GetConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(hub *Hub) echo.HandlerFunc {
	return func(c echo.Context) error {
		conn, err := upgrader.Upgrade(c.Response().Writer, c.Request(), nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return nil
		}

		clientID := c.QueryParam("client_id")
		if clientID == "" {
			clientID = c.RealIP()
		}

		client := &Client{
			ID:   clientID,
			Conn: conn,
			Send: make(chan []byte, 256),
			Hub:  hub,
		}

		hub.register <- client

		go client.writePump()
		go client.readPump()

		return nil
	}
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		_ = c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		c.handleMessage(msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	defer func() { _ = c.Conn.Close() }()

	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}
}

// handleMessage handles incoming messages
func (c *Client) handleMessage(msg Message) {
	switch msg.Type {
	case "ping":
		response := Message{
			Type:    "pong",
			Payload: "pong",
		}
		data, _ := json.Marshal(response)
		c.Send <- data

	case "broadcast":
		data, _ := json.Marshal(msg)
		c.Hub.Broadcast(data)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// BroadcastMessage broadcasts a message to all connected clients
func BroadcastMessage(hub *Hub, messageType string, payload interface{}) {
	msg := Message{
		Type:    messageType,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}
	hub.Broadcast(data)
}

// GetHubStats returns hub statistics
func GetHubStats(hub *Hub) map[string]interface{} {
	return map[string]interface{}{
		"connected_clients": hub.GetConnectedClients(),
	}
}
