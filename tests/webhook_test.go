package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"stackyrd/pkg/webhook"

	"github.com/stretchr/testify/assert"
)

func TestWebhook_NewManager(t *testing.T) {
	cfg := webhook.DefaultWebhookConfig()
	wm := webhook.NewWebhookManager(cfg)
	assert.NotNil(t, wm)
}

func TestWebhook_RegisterAndTrigger(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.DefaultWebhookConfig())

	var called atomic.Int32
	wm.Register("test.event", func(event webhook.WebhookEvent) {
		called.Add(1)
	})

	event := webhook.WebhookEvent{
		Type: "test.event",
		Data: map[string]interface{}{"key": "value"},
	}
	wm.Trigger(event)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), called.Load())
}

func TestWebhook_MultipleHandlers(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.DefaultWebhookConfig())

	var c1, c2 atomic.Int32
	wm.Register("evt", func(event webhook.WebhookEvent) { c1.Add(1) })
	wm.Register("evt", func(event webhook.WebhookEvent) { c2.Add(1) })

	wm.Trigger(webhook.WebhookEvent{Type: "evt"})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), c1.Load())
	assert.Equal(t, int32(1), c2.Load())
}

func TestWebhook_NoHandlerForType(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.DefaultWebhookConfig())
	wm.Register("type_a", func(event webhook.WebhookEvent) {})

	wm.Trigger(webhook.WebhookEvent{Type: "type_b"})
}

func TestWebhook_SignAndVerify(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{
		Secret:  "mysecret",
		Enabled: true,
	})

	payload := []byte(`{"type":"test","data":{"key":"val"}}`)
	sig := wm.SignPayload(payload)

	assert.True(t, webhook.VerifySignature(payload, sig, "mysecret"))
	assert.False(t, webhook.VerifySignature(payload, sig, "wrongsecret"))
	assert.False(t, webhook.VerifySignature([]byte("tampered"), sig, "mysecret"))
}

func TestWebhook_SendDisabled(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{Enabled: false})
	_, err := wm.Send(context.Background(), webhook.WebhookEvent{})
	assert.ErrorContains(t, err, "disabled")
}

func TestWebhook_SendToServer(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wm := webhook.NewWebhookManager(webhook.WebhookConfig{
		URL:        srv.URL + "/hook",
		Enabled:    true,
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	resp, err := wm.Send(context.Background(), webhook.WebhookEvent{
		Type: "test",
		Data: map[string]interface{}{"msg": "hello"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), received.Load())
}

func TestWebhook_SendRetriesOnFailure(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	wm := webhook.NewWebhookManager(webhook.WebhookConfig{
		URL:        srv.URL + "/hook",
		Enabled:    true,
		Timeout:    5 * time.Second,
		MaxRetries: 2,
		RetryDelay: 1 * time.Millisecond,
	})

	_, err := wm.Send(context.Background(), webhook.WebhookEvent{Type: "test"})
	assert.Error(t, err)
	assert.Equal(t, int32(3), attempts.Load()) // initial + 2 retries
}

func TestWebhook_HandleIncoming(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{Enabled: true, Secret: ""})
	var called atomic.Int32
	wm.Register("incoming", func(event webhook.WebhookEvent) { called.Add(1) })

	handler := webhook.NewWebhookHandler(wm)
	rec := httptest.NewRecorder()

	body, _ := json.Marshal(webhook.WebhookEvent{Type: "incoming", Data: map[string]interface{}{}})
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	handler.Handle(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), called.Load())
}

func TestWebhook_HandleIncomingVerify(t *testing.T) {
	secret := "shared-secret"
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{Enabled: true, Secret: secret})

	handler := webhook.NewWebhookHandler(wm)
	rec := httptest.NewRecorder()

	body, _ := json.Marshal(webhook.WebhookEvent{Type: "verified"})
	sig := wm.SignPayload(body)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)

	handler.Handle(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebhook_HandleIncomingBadSignature(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{Enabled: true, Secret: "secret"})

	handler := webhook.NewWebhookHandler(wm)
	rec := httptest.NewRecorder()

	body, _ := json.Marshal(webhook.WebhookEvent{Type: "bad"})
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "badsig")

	handler.Handle(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebhook_HandleIncorrectMethod(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{Enabled: true})

	handler := webhook.NewWebhookHandler(wm)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)

	handler.Handle(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestWebhook_GetStats(t *testing.T) {
	wm := webhook.NewWebhookManager(webhook.WebhookConfig{
		URL:     "http://hook.example.com",
		Enabled: true,
	})
	wm.Register("evt1", func(event webhook.WebhookEvent) {})
	wm.Register("evt2", func(event webhook.WebhookEvent) {})

	stats := wm.GetStats()
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, "http://hook.example.com", stats["url"])
	types := stats["event_types"].([]string)
	assert.Len(t, types, 2)
}
