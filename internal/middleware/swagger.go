package middleware

import (
	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "stackyrd/docs"
)

func init() {
	// Register Swagger middleware
	RegisterMiddleware("swagger", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		// Swagger is special - it needs route registration, not just middleware
		// Return nil handler - the route registration happens in server.go
		return nil, nil
	})
}

// SwaggerConfig holds Swagger UI configuration
type SwaggerConfig struct {
	Enabled  bool
	BasePath string
}

// Default Swagger configuration
var defaultSwaggerConfig = SwaggerConfig{
	Enabled:  true,
	BasePath: "/swagger",
}

// Swagger middleware serves Swagger UI documentation
func Swagger() gin.HandlerFunc {
	return SwaggerWithConfig(defaultSwaggerConfig)
}

// SwaggerWithConfig serves Swagger UI with custom configuration
func SwaggerWithConfig(config SwaggerConfig) gin.HandlerFunc {
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return ginSwagger.WrapHandler(swaggerFiles.Handler)
}

// RegisterSwaggerRoutes registers Swagger UI endpoints on the Gin engine
func RegisterSwaggerRoutes(r *gin.Engine, config SwaggerConfig) {
	if !config.Enabled {
		return
	}

	// Register Swagger UI endpoint
	r.GET(config.BasePath+"/*any", SwaggerWithConfig(config))

	// Redirect root swagger path to index
	r.GET(config.BasePath, func(c *gin.Context) {
		c.Redirect(301, config.BasePath+"/index.html")
	})
}
