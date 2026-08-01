package response

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

type dummyWriter struct {
	header http.Header
}

func (d *dummyWriter) Header() http.Header         { return d.header }
func (d *dummyWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *dummyWriter) WriteHeader(int)             {}

func BenchmarkSuccess(b *testing.B) {
	e := echo.New()
	defer e.Close()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := &dummyWriter{header: make(http.Header)}
	c := e.NewContext(req, w)

	b.ResetTimer()
	for b.Loop() {
		_ = Success(c, map[string]string{"key": "value"})
	}
}

func BenchmarkError(b *testing.B) {
	e := echo.New()
	defer e.Close()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := &dummyWriter{header: make(http.Header)}
	c := e.NewContext(req, w)

	b.ResetTimer()
	for b.Loop() {
		_ = Error(c, http.StatusBadRequest, "BAD_REQUEST", "something went wrong")
	}
}

func BenchmarkSuccessWithMeta(b *testing.B) {
	e := echo.New()
	defer e.Close()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := &dummyWriter{header: make(http.Header)}
	c := e.NewContext(req, w)

	meta := CalculateMeta(1, 20, 150)

	b.ResetTimer()
	for b.Loop() {
		_ = SuccessWithMeta(c, map[string]string{"key": "value"}, meta)
	}
}

func BenchmarkCreated(b *testing.B) {
	e := echo.New()
	defer e.Close()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := &dummyWriter{header: make(http.Header)}
	c := e.NewContext(req, w)

	b.ResetTimer()
	for b.Loop() {
		_ = Created(c, map[string]string{"key": "value"})
	}
}

func BenchmarkGetCorrelationID(b *testing.B) {
	e := echo.New()
	defer e.Close()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := &dummyWriter{header: make(http.Header)}
	c := e.NewContext(req, w)

	b.ResetTimer()
	for b.Loop() {
		_ = getCorrelationID(c)
	}
}
