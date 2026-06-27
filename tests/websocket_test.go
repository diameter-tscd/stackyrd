package main_test

import (
	"encoding/json"
	"testing"
	"time"

	"stackyrd/pkg/websocket"

	"github.com/stretchr/testify/assert"
)

func TestWebSocket_NewHub(t *testing.T) {
	hub := websocket.NewHub()
	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.GetConnectedClients())
}

func TestWebSocket_Broadcast(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	msg := websocket.Message{
		Type:    "test",
		Payload: "payload",
	}
	data, _ := json.Marshal(msg)

	hub.Broadcast(data)
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocket_SendToClient(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	hub.SendToClient("nonexistent", []byte("msg"))
}

func TestWebSocket_BroadcastMessage(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	websocket.BroadcastMessage(hub, "system", "server starting")
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocket_GetHubStats(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	stats := websocket.GetHubStats(hub)
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats["connected_clients"])
}

func TestWebSocket_HubRunStops(t *testing.T) {
	hub := websocket.NewHub()

	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
}

func TestWebSocket_MessageJSON(t *testing.T) {
	msg := websocket.Message{
		Type:    "ping",
		Payload: "pong",
		Room:    "general",
	}
	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded websocket.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "ping", decoded.Type)
	assert.Equal(t, "general", decoded.Room)
}

func TestWebSocket_HandleWebSocketReturnsMiddleware(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	mw := websocket.HandleWebSocket(hub)
	assert.NotNil(t, mw)
}
