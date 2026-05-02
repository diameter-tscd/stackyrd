package middleware

import (
	"errors"
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	// Register JWT middleware
	RegisterMiddleware("jwt", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		// Use config for JWT secret, fallback to default
		secretKey := "your-secret-key" // default
		if cfg.Auth.Type == "jwt" && cfg.Auth.Secret != "" {
			secretKey = cfg.Auth.Secret
		}
		return JWTRequired(secretKey), nil
	})
}

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey     string
	TokenLookup   string // "header:Authorization", "query:token", "cookie:token"
	SigningMethod string
}

// Default JWT configuration
var defaultJWTConfig = JWTConfig{
	SecretKey:     "your-secret-key",
	TokenLookup:   "header:Authorization",
	SigningMethod: jwt.SigningMethodHS256.Name,
}

// GenerateToken creates a new JWT token
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

// JWTRequired middleware validates JWT tokens
func JWTRequired(secretKey string) gin.HandlerFunc {
	config := defaultJWTConfig
	config.SecretKey = secretKey
	return JWT(config)
}

// JWT middleware with custom configuration
func JWT(config JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractToken(c, config.TokenLookup)
		if err != nil {
			response.Unauthorized(c, "Missing or invalid token")
			c.Abort()
			return
		}

		parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.SecretKey), nil
		})

		if err != nil || !parsedToken.Valid {
			response.Unauthorized(c, "Invalid token")
			c.Abort()
			return
		}

		if claims, ok := parsedToken.Claims.(*JWTClaims); ok {
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
		}

		c.Next()
	}
}

// extractToken extracts token from header, query, or cookie
func extractToken(c *gin.Context, tokenLookup string) (string, error) {
	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return c.GetHeader("Authorization"), nil
	}

	source := parts[0]
	key := parts[1]

	switch source {
	case "header":
		authHeader := c.GetHeader(key)
		if authHeader == "" {
			return "", errors.New("authorization header not found")
		}
		// Remove "Bearer " prefix
		return strings.TrimPrefix(authHeader, "Bearer "), nil

	case "query":
		return c.Query(key), nil

	case "cookie":
		cookie, err := c.Cookie(key)
		if err != nil {
			return "", err
		}
		return cookie, nil

	default:
		return c.GetHeader("Authorization"), nil
	}
}

// JWTOptional middleware validates JWT tokens if present, but doesn't require them
func JWTOptional(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractToken(c, defaultJWTConfig.TokenLookup)
		if err != nil {
			c.Next()
			return
		}

		parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})

		if err != nil || !parsedToken.Valid {
			c.Next()
			return
		}

		if claims, ok := parsedToken.Claims.(*JWTClaims); ok {
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
		}

		c.Next()
	}
}

// RequireRole middleware checks if user has required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		response.Forbidden(c, "Insufficient permissions")
		c.Abort()
	}
}

// RequireAdmin middleware checks if user has admin role
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// GetUserID retrieves user ID from context
func GetUserID(c *gin.Context) string {
	if id, exists := c.Get("user_id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}

// GetUsername retrieves username from context
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		if usernameStr, ok := username.(string); ok {
			return usernameStr
		}
	}
	return ""
}

// GetUserRole retrieves user role from context
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("role"); exists {
		if roleStr, ok := role.(string); ok {
			return roleStr
		}
	}
	return ""
}