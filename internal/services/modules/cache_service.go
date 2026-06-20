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

	"github.com/gin-gonic/gin"
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
	TTL   int    `json:"ttl_seconds"` // Optional
}

func (s *CacheService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/cache")

	// GET /cache/:key
	sub.GET("/:key", s.GetCachedValue)

	// POST /cache/:key
	sub.POST("/:key", s.SetCachedValue)
}

// GetCachedValue godoc
// @Summary Get cached value by key
// @Description Retrieve a cached value by its key
// @Tags cache
// @Accept json
// @Produce json
// @Param key path string true "Cache key"
// @Success 200 {object} response.Response "Success"
// @Failure 404 {object} response.Response "Key not found or expired"
// @Router /cache/{key} [get]
func (s *CacheService) GetCachedValue(c *gin.Context) {
	key := c.Param("key")
	val, found := s.store.Get(key)
	if !found {
		response.NotFound(c, "Key not found or expired")
		return
	}
	response.Success(c, map[string]string{"key": key, "value": val})
}

// SetCachedValue godoc
// @Summary Set cached value
// @Description Store a value in the cache with optional TTL
// @Tags cache
// @Accept json
// @Produce json
// @Param key path string true "Cache key"
// @Param request body CacheRequest true "Cache request"
// @Success 200 {object} response.Response "Cached successfully"
// @Failure 400 {object} response.Response "Invalid body"
// @Router /cache/{key} [post]
func (s *CacheService) SetCachedValue(c *gin.Context) {
	key := c.Param("key")
	var req CacheRequest
	if err := request.Bind(c, &req); err != nil {
		response.BadRequest(c, "Invalid body")
		return
	}

	ttl := time.Duration(req.TTL) * time.Second
	s.store.Set(key, req.Value, ttl)

	response.Success(c, map[string]string{
		"message": "Cached successfully",
		"key":     key,
		"ttl":     ttl.String(),
	})
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("cache_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewCacheService(config.Services.IsEnabled("cache_service"))
	})
}
