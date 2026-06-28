package main_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupBenchRouter(numRoutes int) *echo.Echo {
	e := echo.New()
	for i := 0; i < numRoutes; i++ {
		e.GET("/bench/item/:id/"+strconv.Itoa(i), func(c echo.Context) error { return nil })
	}
	return e
}

func BenchmarkRouter_ServeHTTP_Baseline(b *testing.B) {
	r := setupBenchRouter(0)
	req, _ := http.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkRouter_ServeHTTP_SingleRoute(b *testing.B) {
	r := setupBenchRouter(1)
	req, _ := http.NewRequest("GET", "/bench/item/:id/0", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkRouter_ServeHTTP_FiftyRoutes(b *testing.B) {
	r := setupBenchRouter(50)
	req, _ := http.NewRequest("GET", "/bench/item/:id/49", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_Success(b *testing.B) {
	e := echo.New()
	e.GET("/json", func(c echo.Context) error {
		data := map[string]interface{}{"id": 1, "name": "Alice", "email": "alice@example.com"}
		return response.Success(c, data, "ok")
	})

	req, _ := http.NewRequest("GET", "/json", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_WithMetaPagination(b *testing.B) {
	e := echo.New()
	e.GET("/paginated", func(c echo.Context) error {
		meta := response.CalculateMeta(1, 10, 1000)
		return response.SuccessWithMeta(c, []string{"a", "b", "c"}, meta, "ok")
	})

	req, _ := http.NewRequest("GET", "/paginated", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_RequestBind(b *testing.B) {
	e := echo.New()

	type payload struct {
		Name     string `json:"name"     validate:"required"`
		Email    string `json:"email"    validate:"required,email"`
		Phone    string `json:"phone"    validate:"phone"`
		Username string `json:"username" validate:"username"`
		Age      int    `json:"age"      validate:"gte=0,lte=130"`
	}

	e.POST("/bind", func(c echo.Context) error {
		var p payload
		if err := c.Bind(&p); err == nil {
			return response.Success(c, p, "bound")
		} else {
			return response.BadRequest(c, err.Error())
		}
	})

	body := []byte(`{"name":"Alice","email":"alice@example.com","phone":"+1234567890","username":"alice123","age":30}`)
	req, _ := http.NewRequest("POST", "/bind", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_PathParameter(b *testing.B) {
	e := echo.New()
	e.GET("/item/:id", func(c echo.Context) error {
		id := c.Param("id")
		return response.Success(c, map[string]interface{}{"id": id}, "ok")
	})

	req, _ := http.NewRequest("GET", "/item/42", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkMiddleware_RecoveryOverhead(b *testing.B) {
	e := echo.New()
	e.GET("/mid", func(c echo.Context) error { return response.NoContent(c) })

	req, _ := http.NewRequest("GET", "/mid", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkHandler_JSON_SmallPayload(b *testing.B) {
	e := echo.New()
	e.GET("/small", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{"x": 1}, "")
	})
	req, _ := http.NewRequest("GET", "/small", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_LargePayload(b *testing.B) {
	e := echo.New()

	items := make([]map[string]interface{}, 100)
	for i := range items {
		items[i] = map[string]interface{}{
			"id":    i,
			"name":  "User Name Placeholder that has some length",
			"email": "user@example.com",
			"value": float64(i) * 1.23,
		}
	}

	e.GET("/large", func(c echo.Context) error {
		return response.Success(c, items, "ok")
	})
	req, _ := http.NewRequest("GET", "/large", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_ErrorResponse(b *testing.B) {
	e := echo.New()
	e.GET("/err", func(c echo.Context) error {
		return response.NotFound(c, "resource not found")
	})
	req, _ := http.NewRequest("GET", "/err", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkService_Endpoint_Concurrent(b *testing.B) {
	e := echo.New()

	e.GET("/health", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{"status": "ok"}, "healthy")
	})
	e.POST("/users", func(c echo.Context) error {
		data := map[string]interface{}{"id": 1, "name": "Alice", "email": "alice@example.com"}
		return response.Created(c, data, "created")
	})
	e.GET("/users/:id", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{"id": c.Param("id")}, "found")
	})

	bodyJ, _ := json.Marshal(map[string]interface{}{"name": "Alice", "email": "alice@example.com"})
	postReq, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(bodyJ))
	postReq.Header.Set("Content-Type", "application/json")

	getUserReq, _ := http.NewRequest("GET", "/users/1", nil)
	healthReq, _ := http.NewRequest("GET", "/health", nil)

	healthBodyBefore, _ := json.Marshal(map[string]interface{}{"status": "ok", "success": true})

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		frame := 0
		for pb.Next() {
			switch frame % 3 {
			case 0:
				e.ServeHTTP(httptest.NewRecorder(), healthReq)
			case 1:
				e.ServeHTTP(httptest.NewRecorder(), getUserReq)
			default:
				e.ServeHTTP(httptest.NewRecorder(), postReq)
			}
			frame++
		}
	})

	_ = healthBodyBefore
}

func BenchmarkRouter_SubRoute_Depth(b *testing.B) {
	e := echo.New()
	api := e.Group("/api/v1")
	v1 := api.Group("/v1")
	v1.GET("/items/:id", func(c echo.Context) error { return nil })

	req, _ := http.NewRequest("GET", "/api/v1/v1/items/1", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func TestPerformancePackage_RouterReturnsResponse(t *testing.T) {
	e := echo.New()
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}
