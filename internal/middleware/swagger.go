package middleware

import (
	"stackyrd/config"
	"stackyrd/pkg/logger"

	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/labstack/echo/v4"
	_ "stackyrd/docs"
)

func init() {
	RegisterMiddleware("swagger", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return nil, nil
	})
}

type SwaggerConfig struct {
	Enabled  bool
	BasePath string
}

var defaultSwaggerConfig = SwaggerConfig{
	Enabled:  true,
	BasePath: "/swagger",
}

func Swagger() echo.HandlerFunc {
	return SwaggerWithConfig(defaultSwaggerConfig)
}

func SwaggerWithConfig(config SwaggerConfig) echo.HandlerFunc {
	if !config.Enabled {
		return func(c echo.Context) error {
			return nil
		}
	}

	return echoSwagger.WrapHandler
}

func RegisterSwaggerRoutes(e *echo.Echo, config SwaggerConfig) {
	if !config.Enabled {
		return
	}

	e.GET(config.BasePath+"/*", SwaggerWithConfig(config))
	e.GET(config.BasePath, func(c echo.Context) error {
		return c.Redirect(301, config.BasePath+"/index.html")
	})
}
