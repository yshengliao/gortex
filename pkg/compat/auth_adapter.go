package compat

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/auth"
)

// GetClaimsFromEcho retrieves JWT claims from Echo context (backward compatibility)
func GetClaimsFromEcho(c echo.Context) *auth.Claims {
	// First try the standard location
	if claims, ok := c.Get("jwt-claims").(*auth.Claims); ok {
		return claims
	}
	// Try the old Echo auth middleware location
	if claims, ok := c.Get(auth.ClaimsContextKey).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// SetClaimsInEcho sets JWT claims in Echo context (backward compatibility)
func SetClaimsInEcho(c echo.Context, claims *auth.Claims) {
	c.Set("jwt-claims", claims)
	c.Set(auth.ClaimsContextKey, claims) // Also set in old location for compatibility
}

// GetUserIDFromEcho retrieves user ID from JWT claims in Echo context
func GetUserIDFromEcho(c echo.Context) string {
	if claims := GetClaimsFromEcho(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetUsernameFromEcho retrieves username from JWT claims in Echo context
func GetUsernameFromEcho(c echo.Context) string {
	if claims := GetClaimsFromEcho(c); claims != nil {
		return claims.Username
	}
	return ""
}

// AuthErrorResponse creates an auth error response for Echo handlers
func AuthErrorResponse(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    http.StatusText(statusCode),
			"message": message,
		},
	})
}