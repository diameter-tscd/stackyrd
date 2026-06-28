package testing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// NewTestEcho creates a new Echo instance for testing
func NewTestEcho() *echo.Echo {
	return echo.New()
}

// NewTestContext creates a new test context with the given method, path, and body
func NewTestContext(method, path string, body interface{}) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()

	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	c := e.NewContext(req, rec)
	return c, rec
}

// NewTestContextWithQuery creates a test context with query parameters
func NewTestContextWithQuery(method, path string, queryParams map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()

	req := httptest.NewRequest(method, path, nil)
	q := req.URL.Query()
	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	c := e.NewContext(req, rec)
	return c, rec
}

// NewTestContextWithParams creates a test context with path parameters
func NewTestContextWithParams(method, path string, params map[string]string, body interface{}) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()

	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	c := e.NewContext(req, rec)

	names := make([]string, 0, len(params))
	values := make([]string, 0, len(params))
	for k, v := range params {
		names = append(names, k)
		values = append(values, v)
	}
	c.SetParamNames(names...)
	c.SetParamValues(values...)

	return c, rec
}

// ParseResponse parses the response body into the given struct
func ParseResponse(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// AssertStatus asserts the response status code
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d", expected, rec.Code)
	}
}

// AssertJSON asserts the response JSON contains the expected fields
func AssertJSON(t *testing.T, rec *httptest.ResponseRecorder, expected map[string]interface{}) {
	t.Helper()
	var actual map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &actual); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("expected key %q not found in response", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("for key %q: expected %v, got %v", key, expectedValue, actualValue)
		}
	}
}
