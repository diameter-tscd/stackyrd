package middleware

import (
	"strings"
	"time"

	"github.com/samber/oops"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"
)

func init() {
	RegisterMiddleware("jwt", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		if cfg.Auth.Type != "jwt" || cfg.Auth.Secret == "" {
			return nil, oops.In("jwt-middleware").Tags("auth").Code("jwt_missing_secret").With("auth_type", cfg.Auth.Type).Public("JWT middleware requires a valid secret").Errorf("jwt middleware requires auth.type=jwt and a non-empty auth.secret")
		}
		return JWTRequired(cfg.Auth.Secret), nil
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

// jwtIssuer and jwtAudience bound tokens to this service so a token minted
// here is rejected elsewhere even with the same signing secret.
const (
	jwtIssuer   = "stackyrd"
	jwtAudience = "stackyrd-api"
)

func GenerateToken(userID, username, email, role, secretKey string, expiration time.Duration) (string, error) {
	if len(secretKey) < 16 {
		return "", oops.In("token-generation").Tags("jwt").Code("secret_too_short").Public("Secret key must be at least 16 bytes").With("secret_len", len(secretKey)).Errorf("secret key must be at least 16 bytes")
	}
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func JWTRequired(secretKey string) echo.MiddlewareFunc {
	return JWT(JWTConfig{SecretKey: secretKey, TokenLookup: "header:Authorization", SigningMethod: jwt.SigningMethodHS256.Name}, false)
}

func JWTOptional(secretKey string) echo.MiddlewareFunc {
	return JWT(JWTConfig{SecretKey: secretKey, TokenLookup: "header:Authorization", SigningMethod: jwt.SigningMethodHS256.Name}, true)
}

func JWT(config JWTConfig, optional bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				if optional {
					return next(c)
				}
				return response.Unauthorized(c, "Missing or invalid token")
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")

parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, oops.In("jwt-middleware").Tags("jwt", "auth").Code("unexpected_signing_method").With("method", token.Method.Alg()).Errorf("unexpected signing method: %s", token.Method.Alg())
			}
			return []byte(config.SecretKey), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
				jwt.WithIssuer(jwtIssuer), jwt.WithAudience(jwtAudience),
				jwt.WithIssuedAt(), jwt.WithLeeway(30*time.Second))
			if err != nil || !parsedToken.Valid {
				if optional {
					return next(c)
				}
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
	if id, ok := c.Get("user_id").(string); ok {
		return id
	}
	return ""
}

func GetUsername(c echo.Context) string {
	if username, ok := c.Get("username").(string); ok {
		return username
	}
	return ""
}

func GetUserRole(c echo.Context) string {
	if role, ok := c.Get("role").(string); ok {
		return role
	}
	return ""
}
