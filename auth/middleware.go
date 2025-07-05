package auth

import (
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	// ClaimsContextKey is the key used to store claims in echo context
	ClaimsContextKey = "jwt-claims"
)

// Middleware creates a JWT authentication middleware
func Middleware(jwtService *JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get token from Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(401, "missing authorization header")
			}

			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return echo.NewHTTPError(401, "invalid authorization header format")
			}

			// Validate token
			claims, err := jwtService.ValidateToken(parts[1])
			if err != nil {
				return echo.NewHTTPError(401, "invalid or expired token")
			}

			// Store claims in context
			c.Set(ClaimsContextKey, claims)

			return next(c)
		}
	}
}

// GetClaims retrieves JWT claims from echo context
func GetClaims(c echo.Context) *Claims {
	if claims, ok := c.Get(ClaimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}

// GetUserID retrieves user ID from JWT claims
func GetUserID(c echo.Context) string {
	if claims := GetClaims(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetUsername retrieves username from JWT claims
func GetUsername(c echo.Context) string {
	if claims := GetClaims(c); claims != nil {
		return claims.Username
	}
	return ""
}

// RequireRole creates a middleware that requires a specific role
func RequireRole(requiredRole string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return echo.NewHTTPError(401, "unauthorized")
			}

			if claims.Role != requiredRole {
				return echo.NewHTTPError(403, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// RequireGameID creates a middleware that requires a game ID in the token
func RequireGameID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return echo.NewHTTPError(401, "unauthorized")
			}

			if claims.GameID == "" {
				return echo.NewHTTPError(403, "game-specific token required")
			}

			return next(c)
		}
	}
}