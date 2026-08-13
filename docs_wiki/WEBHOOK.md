# Webhooks

## Send

```go
wm := webhook.NewWebhookManager(webhook.WebhookConfig{
    URL: "https://example.com/webhook",
    Secret: "shared-secret",
    MaxRetries: 3,
    Timeout: 30 * time.Second,
})

event := webhook.WebhookEvent{Type: "user.created", Data: map[string]any{"id": 1}}
resp, err := wm.Send(ctx, event)
```

`Send` signs with HMAC-SHA256 when `Secret` is set.

## Receive

```go
wm.Register("user.created", func(e webhook.WebhookEvent) {
    // handle event
})

handler := webhook.NewWebhookHandler(wm)
e.POST("/api/v1/webhook", func(c echo.Context) error {
    handler.Handle(c.Response().Writer, c.Request())
    return nil
})
```

## Verify

```go
webhook.VerifySignature(payload, signature, secret)
```
