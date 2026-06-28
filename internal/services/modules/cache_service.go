package modules

import (
	"time"

	"stackyrd/config"
	"stackyrd/pkg/cache"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
)

type CacheService struct {
	enabled bool
	store   *cache.Cache[string]
}

func NewCacheService(enabled bool) *CacheService {
	return &CacheService{
		enabled: enabled,
		store:   cache.New[string](),
	}
}

func (s *CacheService) Name() string        { return "Cache Service" }
func (s *CacheService) WireName() string    { return "cache-service" }
func (s *CacheService) Enabled() bool       { return s.enabled }
func (s *CacheService) Get() interface{}    { return s }
func (s *CacheService) Endpoints() []string { return []string{"/cache"} }

type CacheRequest struct {
	Value string `json:"value"`
	TTL   int    `json:"ttl_seconds"`
}

func (s *CacheService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/cache")

	sub.GET("/:key", s.GetCachedValue)
	sub.POST("/:key", s.SetCachedValue)
}

func (s *CacheService) GetCachedValue(c echo.Context) error {
	key := c.Param("key")
	val, found := s.store.Get(key)
	if !found {
		return response.NotFound(c, "Key not found or expired")
	}
	return response.Success(c, map[string]string{"key": key, "value": val})
}

func (s *CacheService) SetCachedValue(c echo.Context) error {
	key := c.Param("key")
	var req CacheRequest
	if err := request.Bind(c, &req); err != nil {
		return response.BadRequest(c, "Invalid body")
	}

	ttl := time.Duration(req.TTL) * time.Second
	s.store.Set(key, req.Value, ttl)

	return response.Success(c, map[string]string{
		"message": "Cached successfully",
		"key":     key,
		"ttl":     ttl.String(),
	})
}

func init() {
	registry.RegisterService("cache_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewCacheService(config.Services.IsEnabled("cache_service"))
	})
}
