package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	URL        string
	Secret     string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	Headers    map[string]string
	Enabled    bool
}

// DefaultWebhookConfig returns default webhook configuration
func DefaultWebhookConfig() WebhookConfig {
	return WebhookConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		Headers:    make(map[string]string),
		Enabled:    true,
	}
}

// WebhookEvent represents a webhook event
type WebhookEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature,omitempty"`
}

// WebhookResponse represents a webhook response
type WebhookResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Duration   time.Duration
}

// WebhookManager manages webhooks
type WebhookManager struct {
	config    WebhookConfig
	client    *http.Client
	mu        sync.RWMutex
	handlers  map[string][]func(event WebhookEvent)
	semaphore chan struct{} // bounded concurrency for Trigger
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(config WebhookConfig) *WebhookManager {
	return &WebhookManager{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		handlers:  make(map[string][]func(event WebhookEvent)),
		semaphore: make(chan struct{}, 100), // max 100 concurrent handler goroutines
	}
}

// Register registers a webhook handler for an event type
func (wm *WebhookManager) Register(eventType string, handler func(event WebhookEvent)) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.handlers[eventType] = append(wm.handlers[eventType], handler)
}

// Trigger triggers webhook handlers for an event
func (wm *WebhookManager) Trigger(event WebhookEvent) {
	wm.mu.RLock()
	handlers := wm.handlers[event.Type]
	wm.mu.RUnlock()

	for _, handler := range handlers {
		wm.semaphore <- struct{}{}
		go func(fn func(WebhookEvent)) {
			defer func() { <-wm.semaphore }()
			fn(event)
		}(handler)
	}
}

// Send sends a webhook event to a URL
func (wm *WebhookManager) Send(ctx context.Context, event WebhookEvent) (*WebhookResponse, error) {
	if !wm.config.Enabled {
		return nil, fmt.Errorf("webhook is disabled")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	// Sign the payload
	if wm.config.Secret != "" {
		signature := wm.SignPayload(payload)
		event.Signature = signature
		payload, _ = json.Marshal(event)
	}

	var lastErr error
	for attempt := 0; attempt <= wm.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wm.config.RetryDelay):
			}
		}

		resp, err := wm.doRequest(ctx, payload)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil, lastErr
}

// doRequest performs the HTTP request
func (wm *WebhookManager) doRequest(ctx context.Context, payload []byte) (*WebhookResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wm.config.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "stackyrd-Webhook/1.0")

	for key, value := range wm.config.Headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	resp, err := wm.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &WebhookResponse{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Headers:    headers,
		Duration:   duration,
	}, nil
}

// SignPayload signs a payload with HMAC-SHA256
func (wm *WebhookManager) SignPayload(payload []byte) string {
	h := hmac.New(sha256.New, []byte(wm.config.Secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies a webhook signature
func VerifySignature(payload []byte, signature, secret string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// WebhookHandler handles incoming webhook requests
type WebhookHandler struct {
	manager *WebhookManager
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(manager *WebhookManager) *WebhookHandler {
	return &WebhookHandler{
		manager: manager,
	}
}

// Handle handles an incoming webhook request
func (wh *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is set
	if wh.manager.config.Secret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if !VerifySignature(body, signature, wh.manager.config.Secret) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Trigger handlers
	wh.manager.Trigger(event)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// GetStats returns webhook statistics
func (wm *WebhookManager) GetStats() map[string]interface{} {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	eventTypes := make([]string, 0, len(wm.handlers))
	for eventType := range wm.handlers {
		eventTypes = append(eventTypes, eventType)
	}

	return map[string]interface{}{
		"enabled":     wm.config.Enabled,
		"event_types": eventTypes,
		"url":         wm.config.URL,
	}
}

// InfrastructureComponent interface implementation

func (wm *WebhookManager) Name() string {
	return "Webhook"
}

func (wm *WebhookManager) Close() error {
	return nil
}

func (wm *WebhookManager) GetStatus() map[string]interface{} {
	return wm.GetStats()
}

// RouteHandlers returns HTTP routes for receiving webhooks
func (wm *WebhookManager) RouteHandlers() []infrastructure.RouteHandler {
	path := "/api/v1/webhook"
	return []infrastructure.RouteHandler{
		{
			Path: path,
			Mode: infrastructure.RouterCustom,
			Handlers: []echo.MiddlewareFunc{
				func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						wh := NewWebhookHandler(wm)
						wh.Handle(c.Response().Writer, c.Request())
						return nil
					}
				},
			},
		},
	}
}

var _ infrastructure.InfrastructureComponent = (*WebhookManager)(nil)
var _ infrastructure.RouteRegistrar = (*WebhookManager)(nil)

func init() {
	infrastructure.RegisterComponent("webhook", func(cfg *config.Config, l *logger.Logger) (infrastructure.InfrastructureComponent, error) {
		if !cfg.Webhook.Enabled {
			return nil, nil
		}
		wm := NewWebhookManager(WebhookConfig{
			URL:        cfg.Webhook.URL,
			Secret:     cfg.Webhook.Secret,
			Timeout:    time.Duration(cfg.Webhook.Timeout) * time.Second,
			MaxRetries: cfg.Webhook.MaxRetries,
			RetryDelay: 1 * time.Second,
			Headers:    cfg.Webhook.Headers,
			Enabled:    true,
		})
		l.Info("Webhook manager initialized", "url", cfg.Webhook.URL)
		return wm, nil
	})
}
