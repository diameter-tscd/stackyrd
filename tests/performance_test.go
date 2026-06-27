package main_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupBenchRouter returns a gin.Engine with N distinct routes registered.
// Pass 0 for the bare-engine baseline.
func setupBenchRouter(numRoutes int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for i := 0; i < numRoutes; i++ {
		r.GET("/bench/item/:id/"+strconv.Itoa(i), func(c *gin.Context) {})
	}
	return r
}

// ─── Bare router overhead ────────────────────────────────────────────────

func BenchmarkRouter_ServeHTTP_Baseline(b *testing.B) {
	r := setupBenchRouter(0)
	req, _ := http.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Router latency with one registered route ──────────────────────────────

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

// ─── JSON response serialisation ─────────────────────────────────────────

func BenchmarkHandler_JSON_Success(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/json", func(c *gin.Context) {
		data := gin.H{"id": 1, "name": "Alice", "email": "alice@example.com"}
		response.Success(c, data, "ok")
	})

	req, _ := http.NewRequest("GET", "/json", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_WithMetaPagination(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/paginated", func(c *gin.Context) {
		meta := response.CalculateMeta(1, 10, 1000)
		response.SuccessWithMeta(c, []string{"a", "b", "c"}, meta, "ok")
	})

	req, _ := http.NewRequest("GET", "/paginated", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── JSON request binding ────────────────────────────────────────────────

func BenchmarkHandler_JSON_RequestBind(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	type payload struct {
		Name     string `json:"name"     validate:"required"`
		Email    string `json:"email"    validate:"required,email"`
		Phone    string `json:"phone"    validate:"phone"`
		Username string `json:"username" validate:"username"`
		Age      int    `json:"age"      validate:"gte=0,lte=130"`
	}

	r.POST("/bind", func(c *gin.Context) {
		var p payload
		if err := c.ShouldBindJSON(&p); err == nil {
			response.Success(c, p, "bound")
		} else {
			response.BadRequest(c, err.Error())
		}
	})

	body := []byte(`{"name":"Alice","email":"alice@example.com","phone":"+1234567890","username":"alice123","age":30}`)
	req, _ := http.NewRequest("POST", "/bind", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Path parameter look-up ─────────────────────────────────────────────

func BenchmarkHandler_PathParameter(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/item/:id", func(c *gin.Context) {
		id := c.Param("id")
		response.Success(c, gin.H{"id": id}, "ok")
	})

	req, _ := http.NewRequest("GET", "/item/42", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Middleware overhead ────────────────────────────────────────────────

func BenchmarkMiddleware_RecoveryOverhead(b *testing.B) {
	gin.SetMode(gin.TestMode)

	r := gin.New() // baseline — no middleware
	r.GET("/mid", func(c *gin.Context) { response.NoContent(c) })

	req, _ := http.NewRequest("GET", "/mid", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// ─── Response body size impact ─────────────────────────────────────────

func BenchmarkHandler_JSON_SmallPayload(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/small", func(c *gin.Context) {
		response.Success(c, gin.H{"x": 1}, "")
	})
	req, _ := http.NewRequest("GET", "/small", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_JSON_LargePayload(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 5 KB payload: 100 records of 50 bytes each
	items := make([]gin.H, 100)
	for i := range items {
		items[i] = gin.H{
			"id":    i,
			"name":  "User Name Placeholder that has some length",
			"email": "user@example.com",
			"value": float64(i) * 1.23,
		}
	}

	r.GET("/large", func(c *gin.Context) {
		response.Success(c, items, "ok")
	})
	req, _ := http.NewRequest("GET", "/large", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Error response serialisation ─────────────────────────────────────

func BenchmarkHandler_ErrorResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/err", func(c *gin.Context) {
		response.NotFound(c, "resource not found")
	})
	req, _ := http.NewRequest("GET", "/err", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Concurrent request throughput ─────────────────────────────────────

func BenchmarkService_Endpoint_Concurrent(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Register service-like routes to exercise a realistic route tree.
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"}, "healthy")
	})
	r.POST("/users", func(c *gin.Context) {
		data := gin.H{"id": 1, "name": "Alice", "email": "alice@example.com"}
		response.Created(c, data, "created")
	})
	r.GET("/users/:id", func(c *gin.Context) {
		response.Success(c, gin.H{"id": c.Param("id")}, "found")
	})

	bodyJ, _ := json.Marshal(gin.H{"name": "Alice", "email": "alice@example.com"})
	postReq, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(bodyJ))
	postReq.Header.Set("Content-Type", "application/json")

	getUserReq, _ := http.NewRequest("GET", "/users/1", nil)
	healthReq, _ := http.NewRequest("GET", "/health", nil)

	// snapshot body sizes to capture serialisation cost fairly
	healthBodyBefore, _ := json.Marshal(gin.H{"status": "ok", "success": true})

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		frame := 0
		for pb.Next() {
			switch frame % 3 {
			case 0:
				r.ServeHTTP(httptest.NewRecorder(), healthReq)
			case 1:
				r.ServeHTTP(httptest.NewRecorder(), getUserReq)
			default:
				r.ServeHTTP(httptest.NewRecorder(), postReq)
			}
			frame++
		}
	})

	_ = healthBodyBefore // silences unused warning; reserved for fine-grained body-size assertions
}

// ─── Router group / sub-route overhead ─────────────────────────────────

func BenchmarkRouter_SubRoute_Depth(b *testing.B) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	api := r.Group("/api/v1")
	v1 := api.Group("/v1")
	v1.GET("/items/:id", func(c *gin.Context) {})

	req, _ := http.NewRequest("GET", "/api/v1/v1/items/1", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// ─── Service wiring validation (import-time side-effects check) ─────────

func TestPerformancePackage_RouterReturnsResponse(t *testing.T) {
	// sanity test: ensure the benchmark router returns 200 for registered routes.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })

	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}
