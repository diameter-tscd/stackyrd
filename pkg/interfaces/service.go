package interfaces

import (
	"github.com/labstack/echo/v4"
)

// Service defines the interface that all services must implement
type Service interface {
	Name() string
	Enabled() bool
	RegisterRoutes(g *echo.Group)
	Get() any
}
