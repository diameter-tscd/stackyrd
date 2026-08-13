# WebSockets

Hub-based real-time communication.

```go
hub := websocket.NewHub()
go hub.Run()

e.GET("/ws", func(c echo.Context) error {
    websocket.HandleWebSocket(hub)(c.Response().Writer, c.Request())
    return nil
})
```

## Broadcast

```go
websocket.BroadcastMessage(hub, "event", map[string]any{"msg": "hi"})
```

## Direct

```go
hub.SendToClient(clientID, data)
hub.ConnectedClients()  // count
```

Clients auto-register on connect and unregister on close.
