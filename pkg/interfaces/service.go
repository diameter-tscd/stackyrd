package interfaces

import (
	"github.com/labstack/echo/v4"
)

// Service defines the interface that all services must implement
type Service interface {
	Name() string
	WireName() string
	Enabled() bool
	Endpoints() []string
	RegisterRoutes(g *echo.Group)
	Get() interface{}
}
