package middleware

import (
	"errors"
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("jwt", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		secretKey := "your-secret-key"
		if cfg.Auth.Type == "jwt" && cfg.Auth.Secret != "" {
			secretKey = cfg.Auth.Secret
		}
		return JWTRequired(secretKey), nil
	})
}

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTConfig struct {
	SecretKey     string
	TokenLookup   string
	SigningMethod string
}

var defaultJWTConfig = JWTConfig{
	SecretKey:     "your-secret-key",
	TokenLookup:   "header:Authorization",
	SigningMethod: jwt.SigningMethodHS256.Name,
}

func GenerateToken(userID, username, email, role, secretKey string, expiration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func JWTRequired(secretKey string) echo.MiddlewareFunc {
	config := defaultJWTConfig
	config.SecretKey = secretKey
	return JWT(config)
}

func JWT(config JWTConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := extractToken(c, config.TokenLookup)
			if err != nil {
				return response.Unauthorized(c, "Missing or invalid token")
			}

			parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(config.SecretKey), nil
			})

			if err != nil || !parsedToken.Valid {
				return response.Unauthorized(c, "Invalid token")
			}

			if claims, ok := parsedToken.Claims.(*JWTClaims); ok {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("email", claims.Email)
				c.Set("role", claims.Role)
			}

			return next(c)
		}
	}
}

func extractToken(c echo.Context, tokenLookup string) (string, error) {
	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return c.Request().Header.Get("Authorization"), nil
	}

	source := parts[0]
	key := parts[1]

	switch source {
	case "header":
		authHeader := c.Request().Header.Get(key)
		if authHeader == "" {
			return "", errors.New("authorization header not found")
		}
		return strings.TrimPrefix(authHeader, "Bearer "), nil

	case "query":
		return c.QueryParam(key), nil

	case "cookie":
		cookie, err := c.Cookie(key)
		if err != nil {
			return "", err
		}
		return cookie.Value, nil

	default:
		return c.Request().Header.Get("Authorization"), nil
	}
}

func JWTOptional(secretKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := extractToken(c, defaultJWTConfig.TokenLookup)
			if err != nil {
				return next(c)
			}

			parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(secretKey), nil
			})

			if err != nil || !parsedToken.Valid {
				return next(c)
			}

			if claims, ok := parsedToken.Claims.(*JWTClaims); ok {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("email", claims.Email)
				c.Set("role", claims.Role)
			}

			return next(c)
		}
	}
}

func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userRole := c.Get("role")
			if userRole == nil {
				return response.Forbidden(c, "Insufficient permissions")
			}

			roleStr, ok := userRole.(string)
			if !ok {
				return response.Forbidden(c, "Insufficient permissions")
			}

			for _, role := range roles {
				if roleStr == role {
					return next(c)
				}
			}

			return response.Forbidden(c, "Insufficient permissions")
		}
	}
}

func RequireAdmin() echo.MiddlewareFunc {
	return RequireRole("admin")
}

func GetUserID(c echo.Context) string {
	if id := c.Get("user_id"); id != nil {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}

func GetUsername(c echo.Context) string {
	if username := c.Get("username"); username != nil {
		if usernameStr, ok := username.(string); ok {
			return usernameStr
		}
	}
	return ""
}

func GetUserRole(c echo.Context) string {
	if role := c.Get("role"); role != nil {
		if roleStr, ok := role.(string); ok {
			return roleStr
		}
	}
	return ""
}
