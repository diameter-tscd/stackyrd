package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"stackyrd/pkg/resilience"

	"github.com/gin-gonic/gin"
)

// HTTPClient is an HTTP client with retry capabilities
type HTTPClient struct {
	client      *http.Client
	retryConfig resilience.RetryConfig
}

// HTTPClientOption configures the HTTPClient
type HTTPClientOption func(*HTTPClient)

// WithHTTPClient sets a custom http.Client
func WithHTTPClient(client *http.Client) HTTPClientOption {
	return func(c *HTTPClient) {
		c.client = client
	}
}

// WithRetryConfig sets a custom retry configuration
func WithRetryConfig(config resilience.RetryConfig) HTTPClientOption {
	return func(c *HTTPClient) {
		c.retryConfig = config
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) HTTPClientOption {
	return func(c *HTTPClient) {
		c.client.Timeout = timeout
	}
}

// NewHTTPClient creates a new HTTP client with retry support.
// Example:
//
//	client := NewHTTPClient()
//	client := NewHTTPClient(WithTimeout(60*time.Second))
//	client := NewHTTPClient(WithRetryConfig(customConfig))
func NewHTTPClient(opts ...HTTPClientOption) *HTTPClient {
	client := &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryConfig: resilience.DefaultRetryConfig(),
	}

	// Add default retry logic for HTTP requests
	client.retryConfig.RetryIf = defaultHTTPRetryIf
	client.retryConfig.OnRetry = func(attempt int, err error) {
		// Log retry attempts if needed
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// defaultHTTPRetryIf determines if an error should trigger a retry for HTTP requests
func defaultHTTPRetryIf(err error) bool {
	if err == nil {
		return false
	}

	// Retry on retryable errors
	if resilience.IsRetryable(err) {
		return true
	}

	// Retry on network-related errors
	// Add more sophisticated error checking here as needed
	return true
}

// Do executes an HTTP request with retry support.
// Example:
//
//	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
//	resp, err := client.Do(ctx, req)
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var lastErr error

	err := resilience.RetryWithContext(ctx, func() error {
		// Clone request as it may be modified by Do
		reqClone := req.Clone(ctx)

		var err error
		resp, err = c.client.Do(reqClone)
		if err != nil {
			lastErr = err
			return resilience.NewRetryableError(err)
		}

		// Retry on server errors (5xx)
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			// Close body to prevent leak
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			return resilience.NewRetryableError(lastErr)
		}

		return nil
	}, c.retryConfig)

	if err != nil {
		return nil, err
	}

	return resp, lastErr
}

// Get performs an HTTP GET request with retry.
// Example:
//
//	resp, err := client.Get(ctx, "https://api.example.com/users", nil)
func (c *HTTPClient) Get(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	setHeaders(req, headers)
	return c.Do(ctx, req)
}

// Post performs an HTTP POST request with retry.
// Example:
//
//	body := strings.NewReader("key=value")
//	resp, err := client.Post(ctx, "https://api.example.com/data", body, nil)
func (c *HTTPClient) Post(ctx context.Context, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	setHeaders(req, headers)
	return c.Do(ctx, req)
}

// PostJSON performs an HTTP POST request with JSON body and retry.
// Example:
//
//	data := map[string]interface{}{"name": "John", "age": 30}
//	resp, err := client.PostJSON(ctx, "https://api.example.com/users", data, nil)
func (c *HTTPClient) PostJSON(ctx context.Context, url string, data interface{}, headers map[string]string) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	setHeaders(req, headers)
	return c.Do(ctx, req)
}

// Put performs an HTTP PUT request with retry.
// Example:
//
//	body := strings.NewReader(`{"status": "active"}`)
//	resp, err := client.Put(ctx, "https://api.example.com/users/123", body, nil)
func (c *HTTPClient) Put(ctx context.Context, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}

	setHeaders(req, headers)
	return c.Do(ctx, req)
}

// Delete performs an HTTP DELETE request with retry.
// Example:
//
//	resp, err := client.Delete(ctx, "https://api.example.com/users/123", nil)
func (c *HTTPClient) Delete(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}

	setHeaders(req, headers)
	return c.Do(ctx, req)
}

// GetWithGin performs a GET request using gin.Context for context propagation.
// Example:
//
//	resp, err := client.GetWithGin(c, "https://api.example.com/data", nil)
func (c *HTTPClient) GetWithGin(ginCtx *gin.Context, url string, headers map[string]string) (*http.Response, error) {
	ctx := ginCtx.Request.Context()
	return c.Get(ctx, url, headers)
}

// PostJSONWithGin performs a POST JSON request using gin.Context for context propagation.
// Example:
//
//	data := map[string]string{"action": "restart"}
//	resp, err := client.PostJSONWithGin(c, "https://api.example.com/action", data, nil)
func (c *HTTPClient) PostJSONWithGin(ginCtx *gin.Context, url string, data interface{}, headers map[string]string) (*http.Response, error) {
	ctx := ginCtx.Request.Context()
	return c.PostJSON(ctx, url, data, headers)
}

// RespondWithHTTPResponse sends an HTTP response from an external request through gin.
// Example:
//
//	resp, _ := client.Get(ctx, "https://api.example.com/data", nil)
//	RespondWithHTTPResponse(c, resp)
func RespondWithHTTPResponse(ginCtx *gin.Context, resp *http.Response) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response body"})
		return
	}

	ginCtx.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// setHeaders sets headers on an HTTP request
func setHeaders(req *http.Request, headers map[string]string) {
	if headers == nil {
		return
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}
}
