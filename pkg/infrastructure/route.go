package infrastructure

import (
	"github.com/labstack/echo/v4"
)

type RouterMode int8

const (
	RouterDefault RouterMode = iota
	RouterCustom
	RouterNone
)

type RouteHandler struct {
	Path     string
	Mode     RouterMode
	Handlers []echo.MiddlewareFunc
	Handler  func(*echo.Group)
}

type RouteRegistrar interface {
	RouteHandlers() []RouteHandler
}
