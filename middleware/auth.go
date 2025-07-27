package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yshengliao/gortex/pkg/auth"
	"github.com/yshengliao/gortex/pkg/errors"
)

// AuthConfig contains configuration for the auth middleware
type AuthConfig struct {
	// JWTService is the JWT service for token validation
	JWTService *auth.JWTService
	// SkipPaths is a list of paths to skip authentication
	SkipPaths []string
	// ClaimsContextKey is the key used to store claims in context
	ClaimsContextKey string
}

// DefaultAuthConfig returns the default configuration
func DefaultAuthConfig(jwtService *auth.JWTService) *AuthConfig {
	return &AuthConfig{
		JWTService:       jwtService,
		SkipPaths:        []string{},
		ClaimsContextKey: "jwt-claims",
	}
}

// JWTAuth returns a middleware that validates JWT tokens
func JWTAuth(jwtService *auth.JWTService) MiddlewareFunc {
	return JWTAuthWithConfig(DefaultAuthConfig(jwtService))
}

// JWTAuthWithConfig returns a middleware with custom configuration
func JWTAuthWithConfig(config *AuthConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		panic("auth middleware: config is required")
	}
	if config.JWTService == nil {
		panic("auth middleware: JWTService is required")
	}
	if config.ClaimsContextKey == "" {
		config.ClaimsContextKey = "jwt-claims"
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			req, ok := c.Request().(*http.Request)
			if !ok {
				return next(c)
			}

			// Skip if path is in skip list
			for _, skip := range config.SkipPaths {
				if req.URL.Path == skip || strings.HasPrefix(req.URL.Path, skip) {
					return next(c)
				}
			}

			// Get token from Authorization header
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "missing authorization header",
					},
				}
			}

			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "invalid authorization header format",
					},
				}
			}

			// Validate token
			claims, err := config.JWTService.ValidateToken(parts[1])
			if err != nil {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "invalid or expired token",
						Details: map[string]interface{}{
							"error": err.Error(),
						},
					},
				}
			}

			// Store claims in router context
			c.Set(config.ClaimsContextKey, claims)

			// Also store in request context for standard Go code
			ctx := context.WithValue(req.Context(), config.ClaimsContextKey, claims)
			newReq := req.WithContext(ctx)
			
			// Update the request in context if possible
			if setter, ok := c.(interface{ SetRequest(*http.Request) }); ok {
				setter.SetRequest(newReq)
			}

			return next(c)
		}
	}
}

// RequireRole returns a middleware that requires a specific role
func RequireRole(requiredRole string, claimsKey ...string) MiddlewareFunc {
	key := "jwt-claims"
	if len(claimsKey) > 0 && claimsKey[0] != "" {
		key = claimsKey[0]
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			claimsVal := c.Get(key)
			if claimsVal == nil {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "unauthorized",
					},
				}
			}

			claims, ok := claimsVal.(*auth.Claims)
			if !ok {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "invalid claims type",
					},
				}
			}

			if claims.Role != requiredRole {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeForbidden),
						Message: "insufficient permissions",
						Details: map[string]interface{}{
							"required_role": requiredRole,
							"user_role":     claims.Role,
						},
					},
				}
			}

			return next(c)
		}
	}
}

// RequireGameID returns a middleware that requires a game ID in the token
func RequireGameID(claimsKey ...string) MiddlewareFunc {
	key := "jwt-claims"
	if len(claimsKey) > 0 && claimsKey[0] != "" {
		key = claimsKey[0]
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			claimsVal := c.Get(key)
			if claimsVal == nil {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "unauthorized",
					},
				}
			}

			claims, ok := claimsVal.(*auth.Claims)
			if !ok {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "invalid claims type",
					},
				}
			}

			if claims.GameID == "" {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeForbidden),
						Message: "game-specific token required",
					},
				}
			}

			return next(c)
		}
	}
}

// GetClaims retrieves JWT claims from context
func GetClaims(c Context, claimsKey ...string) *auth.Claims {
	key := "jwt-claims"
	if len(claimsKey) > 0 && claimsKey[0] != "" {
		key = claimsKey[0]
	}

	if claims, ok := c.Get(key).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// GetClaimsFromContext retrieves JWT claims from standard context
func GetClaimsFromContext(ctx Context, claimsKey ...string) *auth.Claims {
	key := "jwt-claims"
	if len(claimsKey) > 0 && claimsKey[0] != "" {
		key = claimsKey[0]
	}

	if claims, ok := ctx.Value(key).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// GetUserID retrieves user ID from JWT claims
func GetUserID(c Context, claimsKey ...string) string {
	if claims := GetClaims(c, claimsKey...); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetUsername retrieves username from JWT claims
func GetUsername(c Context, claimsKey ...string) string {
	if claims := GetClaims(c, claimsKey...); claims != nil {
		return claims.Username
	}
	return ""
}

// SessionConfig contains configuration for session-based authentication
type SessionConfig struct {
	// SessionStore handles session storage and retrieval
	SessionStore SessionStore
	// SessionKey is the cookie/header name for the session ID
	SessionKey string
	// SkipPaths is a list of paths to skip authentication
	SkipPaths []string
}

// SessionStore interface for session storage
type SessionStore interface {
	// Get retrieves session data by session ID
	Get(sessionID string) (map[string]interface{}, error)
	// Validate checks if a session is valid
	Validate(sessionID string) (bool, error)
}

// SessionAuth returns a middleware that validates session-based authentication
func SessionAuth(store SessionStore) MiddlewareFunc {
	return SessionAuthWithConfig(&SessionConfig{
		SessionStore: store,
		SessionKey:   "session_id",
		SkipPaths:    []string{},
	})
}

// SessionAuthWithConfig returns a session middleware with custom configuration
func SessionAuthWithConfig(config *SessionConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		panic("session auth middleware: config is required")
	}
	if config.SessionStore == nil {
		panic("session auth middleware: SessionStore is required")
	}
	if config.SessionKey == "" {
		config.SessionKey = "session_id"
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			req, ok := c.Request().(*http.Request)
			if !ok {
				return next(c)
			}

			// Skip if path is in skip list
			for _, skip := range config.SkipPaths {
				if req.URL.Path == skip || strings.HasPrefix(req.URL.Path, skip) {
					return next(c)
				}
			}

			// Get session ID from cookie or header
			sessionID := ""
			if cookie, err := req.Cookie(config.SessionKey); err == nil {
				sessionID = cookie.Value
			}
			if sessionID == "" {
				sessionID = req.Header.Get(config.SessionKey)
			}

			if sessionID == "" {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "missing session",
					},
				}
			}

			// Validate session
			valid, err := config.SessionStore.Validate(sessionID)
			if err != nil {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeInternalServerError),
						Message: "session validation failed",
						Details: map[string]interface{}{
							"error": err.Error(),
						},
					},
				}
			}

			if !valid {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeUnauthorized),
						Message: "invalid or expired session",
					},
				}
			}

			// Get session data
			sessionData, err := config.SessionStore.Get(sessionID)
			if err != nil {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    int(errors.CodeInternalServerError),
						Message: "failed to retrieve session data",
						Details: map[string]interface{}{
							"error": err.Error(),
						},
					},
				}
			}

			// Store session data in context
			c.Set("session", sessionData)
			c.Set("session_id", sessionID)

			// Also store in request context
			ctx := context.WithValue(req.Context(), "session", sessionData)
			ctx = context.WithValue(ctx, "session_id", sessionID)
			newReq := req.WithContext(ctx)
			
			// Update the request in context if possible
			if setter, ok := c.(interface{ SetRequest(*http.Request) }); ok {
				setter.SetRequest(newReq)
			}

			return next(c)
		}
	}
}