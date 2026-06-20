# Webhook Guide

Webhook management with HMAC-SHA256 signing/verification, retry logic, and concurrent local handlers.

## Overview

The `pkg/webhook/` package supports both sending outgoing webhooks and receiving incoming webhooks with signature verification.

## Configuration

```go
cfg := webhook.DefaultWebhookConfig()
cfg.URL = "https://hooks.example.com/events"
cfg.Secret = "shared-secret-key"
cfg.Timeout = 30 * time.Second
cfg.MaxRetries = 3
cfg.RetryDelay = 1 * time.Second
cfg.Headers = map[string]string{
    "X-Source": "stackyrd",
}
```

## Sending Webhooks

### Basic Send

```go
mgr := webhook.NewWebhookManager(cfg)

event := webhook.WebhookEvent{
    ID:        "evt_123",
    Type:      "order.created",
    Timestamp: time.Now(),
    Data: map[string]interface{}{
        "order_id": "ORD-456",
        "amount":   99.95,
    },
}

resp, err := mgr.Send(context.Background(), event)
// resp.StatusCode, resp.Body, resp.Duration
```

The `Send` method:
1. Serializes the event to JSON
2. Signs the payload with HMAC-SHA256
3. POSTs to the configured URL with retries
4. Returns the response details

## Receiving Webhooks

### Register Local Handlers

```go
mgr := webhook.NewWebhookManager(cfg)

mgr.Register("order.created", func(event webhook.WebhookEvent) {
    log.Printf("Order created: %v", event.Data)
})

mgr.Register("order.updated", func(event webhook.WebhookEvent) {
    log.Printf("Order updated: %v", event.Data)
})
```

### HTTP Endpoint Handler

```go
handler := webhook.NewWebhookHandler(mgr)

http.HandleFunc("/webhook", handler.Handle)
```

The handler:
1. Reads the request body
2. Verifies the HMAC-SHA256 signature
3. Unmarshals the WebhookEvent
4. Dispatches to registered handlers for the event type
5. Returns 200 OK

### Signature Verification

```go
valid := webhook.VerifySignature(
    payload,
    signature,
    secret,
)
```

## WebhookEvent Structure

```go
type WebhookEvent struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
    Signature string                 `json:"signature,omitempty"`
}
```

## Full Example: Outgoing + Incoming

```go
// Outgoing
mgr := webhook.NewWebhookManager(webhook.WebhookConfig{
    URL:    "https://partner.example.com/webhook",
    Secret: "shared-secret",
})

event := webhook.WebhookEvent{
    Type: "user.created",
    Data: map[string]interface{}{"user_id": "usr_123"},
}
mgr.Send(ctx, event)

// Incoming (same manager, different endpoint)
handler := webhook.NewWebhookHandler(mgr)
mgr.Register("user.updated", func(e webhook.WebhookEvent) {
    log.Printf("User updated: %v", e.Data)
})

r.POST("/webhook", func(c *gin.Context) {
    handler.Handle(c.Writer, c.Request)
})
```

## Monitoring

```go
stats := mgr.GetStats()
// Returns: url, enabled, timeout, max_retries
```

Metrics via `pkg/metrics`:

```go
m := metrics.GetMetrics()
m.RecordWebhookEvent("order.created", "success", time.Since(start))
```

## Best Practices

- Always set a `Secret` for signature verification
- Use unique event IDs for idempotency
- Set `MaxRetries` to 3 for transient failures
- Register local handlers for critical events (don't rely solely on external delivery)
- Keep webhook payloads small and focused
- Monitor webhook delivery metrics via Prometheus
- Use a shared secret (not API keys) for HMAC signing
