package modules

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"stackyrd/pkg/logger"
)

// BenchmarkGetProducts measures the performance of ProductsService.getProducts.
func BenchmarkGetProducts(b *testing.B) {
	// Create a ProductsService with a no-op logger.
	loggerInstance := &logger.Logger{}
	svc := NewProductsService(true, loggerInstance)

	// Prepare a dummy echo context.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	ctx := e.NewContext(req, w)

	b.ResetTimer()
	for b.Loop() {
		_ = svc.getProducts(ctx)
	}
}

// dummyResponseWriter implements http.ResponseWriter with no-op methods.
type dummyResponseWriter struct{}

func (d *dummyResponseWriter) Header() http.Header       { return make(http.Header) }
func (d *dummyResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (d *dummyResponseWriter) WriteHeader(int)           {}
