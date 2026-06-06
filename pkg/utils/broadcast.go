package utils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// EventData represents the structure of event data sent through streams
type EventData struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	StreamID  string                 `json:"stream_id,omitempty"`
}

// StreamClient represents a connected client for a specific stream
type StreamClient struct {
	ID              string
	StreamID        string
	Channel         chan EventData
	droppedMessages atomic.Int64 // number of messages dropped because channel was full
	lastSeen        atomic.Int64 // unix timestamp updated on subscribe / successful broadcast
}

// EventBroadcaster manages multiple event streams and their clients
type EventBroadcaster struct {
	streams   map[string][]*StreamClient // streamID -> clients
	clients   map[string]*StreamClient   // clientID -> client
	mu        sync.RWMutex
	nextID    int
	clientTTL time.Duration
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
	eb := &EventBroadcaster{
		streams:   make(map[string][]*StreamClient),
		clients:   make(map[string]*StreamClient),
		nextID:    1,
		clientTTL: 24 * time.Hour, // Clients automatically removed after 24 hours
	}

	// Start cleanup routine
	go eb.cleanupRoutine()

	return eb
}

// touchLastSeen updates the client's last-seen timestamp.
func (eb *EventBroadcaster) touchLastSeen(clientID string) {
	eb.mu.RLock()
	client, exists := eb.clients[clientID]
	eb.mu.RUnlock()
	if exists {
		client.lastSeen.Store(time.Now().Unix())
	}
}

// ExpireStaleClients removes clients whose TTL has expired.  Can be called
// synchronously by tests or the background ticker.
func (eb *EventBroadcaster) ExpireStaleClients() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.expireStaleClientsLocked()
}

// unsubscribeNoLock removes a client without acquiring the outer lock.
// Must be called with eb.mu already held.
func (eb *EventBroadcaster) unsubscribeNoLock(clientID string) {
	client, exists := eb.clients[clientID]
	if !exists {
		return
	}

	// Remove from streams
	if clients, ok := eb.streams[client.StreamID]; ok {
		for i, c := range clients {
			if c.ID == clientID {
				eb.streams[client.StreamID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}

	// Remove from clients map
	delete(eb.clients, clientID)

	// Close channel safely
	select {
	case <-client.Channel:
	default:
		close(client.Channel)
	}
}

func (eb *EventBroadcaster) expireStaleClientsLocked() {
	now := time.Now().Unix()
	for clientID, client := range eb.clients {
		if now-client.lastSeen.Load() > int64(eb.clientTTL.Seconds()) {
			eb.unsubscribeNoLock(clientID)
		}
	}
}

// cleanupRoutine checks client TTLs every 30 minutes and garbage-collects
// stale clients that have not been heard from.
func (eb *EventBroadcaster) cleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		eb.ExpireStaleClients()
	}
}

// Subscribe creates a new client and subscribes to a stream
func (eb *EventBroadcaster) Subscribe(streamID string) *StreamClient {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	clientID := fmt.Sprintf("client_%d", eb.nextID)
	eb.nextID++

	now := time.Now().Unix()
	client := &StreamClient{
		ID:       clientID,
		StreamID: streamID,
		Channel:  make(chan EventData, 100), // Buffer up to 100 messages
	}
	client.lastSeen.Store(now)

	eb.clients[clientID] = client
	eb.streams[streamID] = append(eb.streams[streamID], client)

	return client
}

// Unsubscribe removes a client from all streams
func (eb *EventBroadcaster) Unsubscribe(clientID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	client, exists := eb.clients[clientID]
	if !exists {
		return
	}

	// Remove from streams
	if clients, ok := eb.streams[client.StreamID]; ok {
		for i, c := range clients {
			if c.ID == clientID {
				eb.streams[client.StreamID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}

	// Remove from clients map
	delete(eb.clients, clientID)

	// Close channel safely
	select {
	case <-client.Channel:
	default:
		close(client.Channel)
	}
}

// Broadcast sends an event to all clients subscribed to a stream
func (eb *EventBroadcaster) Broadcast(streamID string, eventType string, message string, data map[string]interface{}) {
	eb.mu.RLock()
	clients := eb.streams[streamID]
	eb.mu.RUnlock()

	event := EventData{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
		StreamID:  streamID,
	}

	var toUnsubscribe []string

	for _, client := range clients {
		select {
		case client.Channel <- event:
			// Update last-seen on successful delivery so TTL cleanup keeps
			// active clients.
			client.lastSeen.Store(time.Now().Unix())
		default:
			// Channel full — count and queue for unsubscription to prevent
			// unbounded goroutine/memory growth.
			client.droppedMessages.Add(1)
			if client.droppedMessages.Load() > 100 {
				toUnsubscribe = append(toUnsubscribe, client.ID)
			}
		}
	}

	if len(toUnsubscribe) > 0 {
		eb.mu.Lock()
		for _, id := range toUnsubscribe {
			eb.unsubscribeNoLock(id)
		}
		eb.mu.Unlock()
	}
}

// BroadcastToAll sends an event to all clients across all streams
func (eb *EventBroadcaster) BroadcastToAll(eventType string, message string, data map[string]interface{}) {
	eb.mu.RLock()
	// Snapshot all clients across all streams while holding the lock
	type streamClient struct {
		client *StreamClient
	}
	var snapshot []streamClient
	for _, streamClients := range eb.streams {
		for _, client := range streamClients {
			snapshot = append(snapshot, streamClient{client: client})
		}
	}
	eb.mu.RUnlock()

	event := EventData{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	var toUnsubscribe []string

	for _, sc := range snapshot {
		select {
		case sc.client.Channel <- event:
			sc.client.lastSeen.Store(time.Now().Unix())
		default:
			// Channel full — count and queue for unsubscription
			sc.client.droppedMessages.Add(1)
			if sc.client.droppedMessages.Load() > 100 {
				toUnsubscribe = append(toUnsubscribe, sc.client.ID)
			}
		}
	}

	if len(toUnsubscribe) > 0 {
		eb.mu.Lock()
		for _, id := range toUnsubscribe {
			eb.unsubscribeNoLock(id)
		}
		eb.mu.Unlock()
	}
}

// GetActiveStreams returns list of active streams and their client counts
func (eb *EventBroadcaster) GetActiveStreams() map[string]int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	result := make(map[string]int)
	for streamID, clients := range eb.streams {
		result[streamID] = len(clients)
	}
	return result
}

// GetStreamClients returns clients for a specific stream
func (eb *EventBroadcaster) GetStreamClients(streamID string) []*StreamClient {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	clients := eb.streams[streamID]
	result := make([]*StreamClient, len(clients))
	copy(result, clients)
	return result
}

// GetTotalClients returns the total number of connected clients across all streams
func (eb *EventBroadcaster) GetTotalClients() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	total := 0
	for _, clients := range eb.streams {
		total += len(clients)
	}
	return total
}

// GetStreamCount returns the number of active streams
func (eb *EventBroadcaster) GetStreamCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.streams)
}

// IsStreamActive checks if a stream has any connected clients
func (eb *EventBroadcaster) IsStreamActive(streamID string) bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	clients, exists := eb.streams[streamID]
	return exists && len(clients) > 0
}
