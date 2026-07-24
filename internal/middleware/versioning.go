package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("versioning", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return VersioningMiddleware(logger), nil
	})
}

// VersionConfig holds version handler mapping
type VersionConfig struct {
	Versions       map[string]VersionHandler
	DefaultVersion string
	SunsetWarnings map[string]SunsetInfo
}

// VersionHandler is a function that registers routes for a version
type VersionHandler func(group *echo.Group)

// SunsetInfo describes deprecation info for a version
type SunsetInfo struct {
	SunsetDate  time.Time
	Deprecation time.Time
	Link        string
}

var versionConfig VersionConfig

// RegisterVersionHandler registers routes for an API version
func RegisterVersionHandler(version string, handler VersionHandler) {
	if versionConfig.Versions == nil {
		versionConfig.Versions = make(map[string]VersionHandler)
		versionConfig.SunsetWarnings = make(map[string]SunsetInfo)
		versionConfig.DefaultVersion = "1"
	}
	versionConfig.Versions[version] = handler
}

// RegisterSunsetWarning registers deprecation info for a version
func RegisterSunsetWarning(version string, info SunsetInfo) {
	if versionConfig.SunsetWarnings == nil {
		versionConfig.SunsetWarnings = make(map[string]SunsetInfo)
	}
	versionConfig.SunsetWarnings[version] = info
}

// SetDefaultVersion sets the default API version
func SetDefaultVersion(v string) {
	versionConfig.DefaultVersion = v
}

// GetVersionConfig returns the version configuration
func GetVersionConfig() VersionConfig {
	return versionConfig
}

// VersionContext contains resolved API version for a request
type VersionContext struct {
	Version      string
	MajorVersion int
	MinorVersion int
	AcceptHeader string
}

// GetVersionFromContext retrieves version info from echo context
func GetVersionFromContext(c echo.Context) *VersionContext {
	if v, ok := c.Get("api_version").(*VersionContext); ok {
		return v
	}
	return nil
}

// VersioningMiddleware reads Accept-Version header and sets version context
func VersioningMiddleware(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			version := c.Request().Header.Get("Accept-Version")
			if version == "" {
				version = c.QueryParam("version")
			}
			if version == "" {
				version = versionConfig.DefaultVersion
			}

			vc := &VersionContext{
				Version:      version,
				AcceptHeader: c.Request().Header.Get("Accept"),
			}

			if maj, err := strconv.Atoi(version); err == nil {
				vc.MajorVersion = maj
			} else {
				parts := strings.SplitN(version, ".", 2)
				if len(parts) > 0 {
					if maj, err := strconv.Atoi(parts[0]); err == nil {
						vc.MajorVersion = maj
					}
				}
				if len(parts) > 1 {
					if min, err := strconv.Atoi(parts[1]); err == nil {
						vc.MinorVersion = min
					}
				}
			}

			c.Set("api_version", vc)
			c.Response().Header().Set("X-API-Version", version)

			if info, ok := versionConfig.SunsetWarnings[version]; ok {
				now := time.Now()
				if !info.Deprecation.IsZero() && now.After(info.Deprecation) {
					c.Response().Header().Set("Warning", fmt.Sprintf("299 - \"Version %s is deprecated. Upgrade by %s\"", version, info.SunsetDate.Format("2006-01-02")))
					c.Response().Header().Set("Deprecation", info.Deprecation.Format(http.TimeFormat))
				}
				if !info.SunsetDate.IsZero() && now.Before(info.SunsetDate) {
					c.Response().Header().Set("Sunset", info.SunsetDate.Format(http.TimeFormat))
					if info.Link != "" {
						c.Response().Header().Set("Link", fmt.Sprintf("<%s>; rel=\"sunset\"", info.Link))
					}
				}
			}

			l.Debug("API version resolved",
				"version", version,
				"major", vc.MajorVersion,
				"path", c.Request().URL.Path,
			)

			return next(c)
		}
	}
}

// VersionMiddleware validates the requested version is supported
func VersionMiddleware(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			vc := GetVersionFromContext(c)
			if vc == nil {
				return next(c)
			}

			if _, supported := versionConfig.Versions[vc.Version]; !supported {
				return response.Error(c,
					http.StatusNotAcceptable,
					"UNSUPPORTED_API_VERSION",
					fmt.Sprintf("API version '%s' is not supported", vc.Version),
					map[string]interface{}{
						"supported_versions": getVersionKeys(versionConfig.Versions),
						"current_version":    vc.Version,
					},
				)
			}

			return next(c)
		}
	}
}

// SetupVersionRoutes configures versioned route groups
func SetupVersionRoutes(e *echo.Echo, basePath string, l *logger.Logger) {
	for version, handler := range versionConfig.Versions {
		v := version
		h := handler
		path := strings.TrimSuffix(basePath, "/")
		versionPath := fmt.Sprintf("%s/v%s", path, v)
		group := e.Group(versionPath)
		h(group)
		l.Info("Registered versioned routes", "version", v, "path", versionPath)
	}
}

func getVersionKeys(m map[string]VersionHandler) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}