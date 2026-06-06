package infrastructure

import (
	"github.com/gin-gonic/gin"
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
	Handlers []gin.HandlerFunc
	Handler  func(*gin.RouterGroup)
}

type RouteRegistrar interface {
	RouteHandlers() []RouteHandler
}
