package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var _ = &Hub{} // suppress unused lint; imported by other packages

var wsClientSeq atomic.Int64

var upgrader = websocket.Upgrader{
	// Reject cross-origin browsers (CSWSH) while allowing non-browser clients
	// and same-origin pages.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(u.Host, r.Host)
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
	Payload any `json:"payload"`
	Room    string      `json:"room,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		clientsByID: make(map[string]*Client),
		// Buffered so producers never block on a single slow consumer even if
		// Run is briefly paused; sized well above realistic burst.
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, exists := h.clientsByID[client.ID]; exists {
				h.mu.Unlock()
				// Duplicate ID: reject the newcomer so clientsByID never points
				// at a connection that is silently shadowed by another.
				close(client.Send)
				log.Printf("Rejected duplicate client: %s", client.ID)
				continue
			}
			h.clients[client] = true
			h.clientsByID[client.ID] = client
			h.mu.Unlock()
			log.Printf("Client connected: %s", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if h.clientsByID[client.ID] == client {
					delete(h.clientsByID, client.ID)
				}
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected: %s", client.ID)

		case message := <-h.broadcast:
			// Write lock: we mutate the map (close + delete) here, and the
			// unregister path closes client.Send too — RLock would race it.
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// SendToClient sends a message to a specific client
func (h *Hub) SendToClient(clientID string, message []byte) {
	// Hold the write lock across the send: closes of client.Send only happen
	// under this lock, so this can never panic with "send on closed channel".
	h.mu.Lock()
	defer h.mu.Unlock()
	client, ok := h.clientsByID[clientID]
	if !ok {
		return
	}
	select {
	case client.Send <- message:
	default:
		delete(h.clients, client)
		if h.clientsByID[clientID] == client {
			delete(h.clientsByID, clientID)
		}
		close(client.Send)
	}
}

// ConnectedClients returns the number of connected clients
func (h *Hub) ConnectedClients() int {
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
			// Server-assigned unique ID: a bare RealIP would collide for NAT
			// clients and shadow each other's connection.
			clientID = fmt.Sprintf("ws-%d-%d", time.Now().UnixNano(), wsClientSeq.Add(1))
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
		if r := recover(); r != nil {
			log.Printf("panic in readPump: %v", r)
		}
		c.Hub.unregister <- c
		_ = c.Conn.Close()
	}()

	// Bound message size and drop silent dead connections.
	c.Conn.SetReadLimit(1 << 20)
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in writePump: %v", r)
		}
		// Deregister so a wedged readPump can't leave the client registered
		// forever. The hub's membership check makes a second unregister no-op.
		c.Hub.unregister <- c
		_ = c.Conn.Close()
	}()

	for message := range c.Send {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
		data, err := json.Marshal(response)
		if err != nil {
			return
		}
		// Route through the hub so the send is serialized against channel close.
		c.Hub.SendToClient(c.ID, data)

	case "broadcast":
		data, _ := json.Marshal(msg)
		c.Hub.Broadcast(data)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// BroadcastMessage broadcasts a message to all connected clients
func BroadcastMessage(hub *Hub, messageType string, payload any) {
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

// Stats returns hub statistics
func (h *Hub) Stats() map[string]any {
	return map[string]any{
		"connected_clients": h.ConnectedClients(),
	}
}
