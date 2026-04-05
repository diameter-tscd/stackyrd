package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"
	"stackyrd/pkg/utils"

	"github.com/gin-gonic/gin"
)

// SimpleStreamGenerator creates automated demo events for streams
type SimpleStreamGenerator struct {
	streamID    string
	broadcaster *utils.EventBroadcaster
	running     bool
	stopChan    chan struct{}
}

func NewSimpleStreamGenerator(streamID string, broadcaster *utils.EventBroadcaster) *SimpleStreamGenerator {
	return &SimpleStreamGenerator{
		streamID:    streamID,
		broadcaster: broadcaster,
		stopChan:    make(chan struct{}),
	}
}

func (sg *SimpleStreamGenerator) Start() {
	if sg.running {
		return
	}
	sg.running = true
	go sg.generateEvents()
}

func (sg *SimpleStreamGenerator) Stop() {
	if !sg.running {
		return
	}
	sg.running = false
	select {
	case sg.stopChan <- struct{}{}:
	default:
		close(sg.stopChan)
	}
}

func (sg *SimpleStreamGenerator) IsRunning() bool {
	return sg.running
}

func (sg *SimpleStreamGenerator) generateEvents() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	events := []struct {
		Type    string
		Message string
		Data    map[string]interface{}
	}{
		{"demo_notification", "Service H notification", map[string]interface{}{"priority": "low"}},
		{"demo_metric", "Metric update", map[string]interface{}{"value": 42}},
		{"demo_alert", "System alert", map[string]interface{}{"level": "info"}},
		{"demo_update", "Data updated", map[string]interface{}{"records": 100}},
	}

	i := 0
	for {
		select {
		case <-sg.stopChan:
			return
		case <-ticker.C:
			event := events[i%len(events)]
			i++

			data := event.Data
			if data == nil {
				data = make(map[string]interface{})
			}
			data["timestamp"] = time.Now().Unix()
			data["service"] = "service_h"
			data["demo_id"] = i

			sg.broadcaster.Broadcast(sg.streamID, event.Type, event.Message, data)
		}
	}
}

// BroadcastService is a demo of using the broadcast utility
type BroadcastService struct {
	enabled     bool
	broadcaster *utils.EventBroadcaster
	streams     map[string]*SimpleStreamGenerator
	logger      *logger.Logger
}

func NewBroadcastService(enabled bool, logger *logger.Logger) *BroadcastService {
	service := &BroadcastService{
		enabled:     enabled,
		broadcaster: utils.NewEventBroadcaster(),
		streams:     make(map[string]*SimpleStreamGenerator),
		logger:      logger,
	}

	if enabled {
		logger.Info("Broadcast Service starting - broadcasting made easy!")
		service.startDemoStreams()
		logger.Info("Broadcast Service ready!")
	}

	return service
}

func (s *BroadcastService) Name() string     { return "Broadcast Service" }
func (s *BroadcastService) WireName() string { return "broadcast-service" }
func (s *BroadcastService) Enabled() bool    { return s.enabled }
func (s *BroadcastService) Get() interface{} { return s }
func (s *BroadcastService) Endpoints() []string {
	return []string{"/events/stream/{stream_id}", "/events/broadcast", "/events/streams"}
}

func (s *BroadcastService) RegisterRoutes(g *gin.RouterGroup) {
	events := g.Group("/events")
	events.GET("/stream/:stream_id", s.streamEvents)
	events.POST("/broadcast", s.broadcastEvent)
	events.GET("/streams", s.getActiveStreams)
	events.POST("/stream/:stream_id/start", s.startStream)
	events.POST("/stream/:stream_id/stop", s.stopStream)
}

// streamEvents handles SSE connections
func (s *BroadcastService) streamEvents(c *gin.Context) {
	streamID := c.Param("stream_id")
	client := s.broadcaster.Subscribe(streamID)
	defer s.broadcaster.Unsubscribe(client.ID)

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Send connection event
	initialEvent := utils.EventData{
		ID:        "connected",
		Type:      "connection",
		Message:   "Connected to stream: " + streamID,
		Data:      map[string]interface{}{"stream_id": streamID, "service": "broadcast_service"},
		Timestamp: time.Now().Unix(),
		StreamID:  streamID,
	}

	s.sendSSEEvent(c, initialEvent)

	// Listen for events
	for {
		select {
		case event := <-client.Channel:
			if err := s.sendSSEEvent(c, event); err != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (s *BroadcastService) broadcastEvent(c *gin.Context) {
	type BroadcastRequest struct {
		StreamID string                 `json:"stream_id,omitempty"`
		Type     string                 `json:"type" validate:"required"`
		Message  string                 `json:"message" validate:"required"`
		Data     map[string]interface{} `json:"data,omitempty"`
	}

	var req BroadcastRequest
	if err := request.Bind(c, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if req.Type == "" || req.Message == "" {
		response.BadRequest(c, "Type and message are required")
		return
	}

	if req.StreamID == "" {
		s.broadcaster.BroadcastToAll(req.Type, req.Message, req.Data)
		response.Success(c, nil, "Event broadcasted to all streams")
	} else {
		s.broadcaster.Broadcast(req.StreamID, req.Type, req.Message, req.Data)
		response.Success(c, nil, fmt.Sprintf("Event broadcasted to stream: %s", req.StreamID))
	}
}

func (s *BroadcastService) getActiveStreams(c *gin.Context) {
	activeStreams := s.broadcaster.GetActiveStreams()
	totalClients := s.broadcaster.GetTotalClients()
	streamCount := s.broadcaster.GetStreamCount()

	streamInfo := make(map[string]interface{})
	for streamID, clientCount := range activeStreams {
		streamInfo[streamID] = map[string]interface{}{
			"clients": clientCount,
			"active":  true,
		}
	}

	result := map[string]interface{}{
		"streams":       streamInfo,
		"total_clients": totalClients,
		"stream_count":  streamCount,
		"service":       "broadcast_service",
	}

	response.Success(c, result, "Active streams retrieved")
}

func (s *BroadcastService) startStream(c *gin.Context) {
	streamID := c.Param("stream_id")

	if generator, exists := s.streams[streamID]; exists {
		generator.Start()
		response.Success(c, nil, fmt.Sprintf("Stream '%s' restarted", streamID))
		return
	}

	generator := NewSimpleStreamGenerator(streamID, s.broadcaster)
	s.streams[streamID] = generator
	generator.Start()

	response.Created(c, nil, fmt.Sprintf("Stream '%s' created and started", streamID))
}

func (s *BroadcastService) stopStream(c *gin.Context) {
	streamID := c.Param("stream_id")

	generator, exists := s.streams[streamID]
	if !exists {
		response.NotFound(c, fmt.Sprintf("Stream '%s' not found", streamID))
		return
	}

	generator.Stop()
	delete(s.streams, streamID)

	response.Success(c, nil, fmt.Sprintf("Stream '%s' stopped and removed", streamID))
}

func (s *BroadcastService) sendSSEEvent(c *gin.Context, event utils.EventData) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
	if err != nil {
		return err
	}

	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (s *BroadcastService) startDemoStreams() {
	streams := []string{"demo-notifications", "demo-metrics", "demo-alerts"}

	for _, streamID := range streams {
		generator := NewSimpleStreamGenerator(streamID, s.broadcaster)
		s.streams[streamID] = generator
		generator.Start()
	}
}

// Auto-registration function
func init() {
	registry.RegisterService("broadcast_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewBroadcastService(config.Services.IsEnabled("broadcast_service"), logger)
	})
}
