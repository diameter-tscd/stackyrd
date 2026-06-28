# WebSocket Guide

Real-time bidirectional communication via WebSocket connections.

## Overview

The WebSocket handler in `pkg/websocket/` provides a hub-based connection manager supporting broadcast, client-specific messaging, and room-based communication.

## Architecture

```mermaid
flowchart TD
    C[Client] -->|WebSocket| H[Hub]
    H --> B1[Broadcast<br/>all clients]
    H --> B2[SendToClient<br/>specific client]
    H --> B3[Run event loop]
    B3 --> R1[register channel]
    B3 --> R2[unregister channel]
    B3 --> R3[broadcast channel]
```

## Basic Usage

### Starting the Hub

```go
import "stackyrd/pkg/websocket"

hub := websocket.NewHub()
go hub.Run() // event loop (blocking)
```

### Registering the WebSocket Endpoint

```go
e := echo.New()
e.GET("/ws", func(c echo.Context) error {
    websocket.HandleWebSocket(hub)(c.Response().Writer, c.Request())
    return nil
})
```

### Broadcasting Messages

```go
// Broadcast raw bytes
hub.Broadcast([]byte(`{"type":"event","data":"hello"}`))

// Broadcast typed message (auto-marshaled)
websocket.BroadcastMessage(hub, "user.created", map[string]interface{}{
    "id":   "usr_123",
    "name": "Alice",
})
```

### Sending to a Specific Client

```go
hub.SendToClient("client-abc-123", []byte(`{"type":"private"}`))
```

## Hub API

```go
type Hub struct {
    clients    map[string]*Client
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte
}

// Start the event loop
func (h *Hub) Run()

// Broadcast raw message to all clients
func (h *Hub) Broadcast(message []byte)

// Send raw message to a specific client by ID
func (h *Hub) SendToClient(clientID string, message []byte)

// Get count of connected clients
func (h *Hub) GetConnectedClients() int
```

## Message Format

```go
type Message struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload,omitempty"`
    Room    string      `json:"room,omitempty"`
}
```

### Using Message Struct

```go
msg := websocket.Message{
    Type:    "order.update",
    Payload: orderData,
    Room:    "room-orders",
}
hub.Broadcast(msg) // auto-marshaled
```

## Client Lifecycle

Each WebSocket connection creates a `Client` with read/write pump goroutines:

```mermaid
flowchart TD
    A[Client connected] --> B[register channel]
    B --> C[Hub adds to clients map]
    C --> D[readPump<br/>read messages]
    C --> E[writePump<br/>send messages]
    C --> F[heartbeat<br/>ping/pong]
    G[Client disconnected] --> H[unregister channel]
    H --> I[Hub removes from clients map]
```

## Monitoring

```go
stats := websocket.GetHubStats(hub)
// Returns: {"connected_clients": 5}
```

## Integration with Services

### Broadcasting from a Service

```go
type BroadcastService struct {
    enabled bool
    logger  *logger.Logger
    hub     *websocket.Hub
}

func (s *BroadcastService) handleEvent(c echo.Context) error {
    var event Event
    if err := c.Bind(&event); err != nil {
        return response.BadRequest(c, err.Error())
    }

    // Broadcast to all connected WebSocket clients
    websocket.BroadcastMessage(s.hub, event.Type, event.Data)
    return response.Success(c, map[string]string{"status": "broadcasted"})
}
```

### WebSocket Service Pattern

```go
// In RegisterRoutes:
func (s *WebSocketService) RegisterRoutes(g *echo.Group) {
    g.GET("/ws", func(c echo.Context) error {
        websocket.HandleWebSocket(s.hub)(c.Response().Writer, c.Request())
        return nil
    })
    g.POST("/ws/broadcast", s.handleBroadcast)
}
```

## Connection Management

- **Client ID**: Auto-generated UUID per connection
- **Read pump**: Goroutine reading messages from the WS connection
- **Write pump**: Goroutine draining buffered messages to the WS connection
- **Cleanup**: Deferred `Close()` on both read and write pumps
- **Concurrency**: Hub channels are unbuffered; `Run()` is the single-event loop

## Best Practices

- Run `hub.Run()` in a dedicated goroutine at startup
- Use `BroadcastMessage()` instead of `Broadcast()` for structured data
- Keep message payloads small (< 1MB recommended)
- Handle client disconnection gracefully (clients register/unregister automatically)
- Do not block in message handlers — use goroutines for heavy processing
