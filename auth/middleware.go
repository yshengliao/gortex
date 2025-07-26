package auth

import (
	"strings"

	"github.com/yshengliao/gortex/http/context"
	"github.com/yshengliao/gortex/middleware"
)

const (
	// ClaimsContextKey is the key used to store claims in context
	ClaimsContextKey = "jwt-claims"
)

// Middleware creates a JWT authentication middleware
func Middleware(jwtService *JWTService) middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			// Get token from Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return context.NewHTTPError(401, "missing authorization header")
			}

			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return context.NewHTTPError(401, "invalid authorization header format")
			}

			// Validate token
			claims, err := jwtService.ValidateToken(parts[1])
			if err != nil {
				return context.NewHTTPError(401, "invalid or expired token")
			}

			// Store claims in context
			c.Set(ClaimsContextKey, claims)

			return next(c)
		}
	}
}

// GetClaims retrieves JWT claims from context
func GetClaims(c context.Context) *Claims {
	if claims, ok := c.Get(ClaimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}

// GetUserID retrieves user ID from JWT claims
func GetUserID(c context.Context) string {
	if claims := GetClaims(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetUsername retrieves username from JWT claims
func GetUsername(c context.Context) string {
	if claims := GetClaims(c); claims != nil {
		return claims.Username
	}
	return ""
}

// RequireRole creates a middleware that requires a specific role
func RequireRole(requiredRole string) middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return context.NewHTTPError(401, "unauthorized")
			}

			if claims.Role != requiredRole {
				return context.NewHTTPError(403, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// RequireGameID creates a middleware that requires a game ID in the token
func RequireGameID() middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return context.NewHTTPError(401, "unauthorized")
			}

			if claims.GameID == "" {
				return context.NewHTTPError(403, "game-specific token required")
			}

			return next(c)
		}
	}
}
