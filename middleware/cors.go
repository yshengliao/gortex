package middleware

import (
	"net/http"
	"strconv"
	"strings"

)

// CORSConfig contains configuration for the CORS middleware
type CORSConfig struct {
	// AllowOrigins is a list of origins that are allowed
	AllowOrigins []string
	// AllowMethods is a list of methods that are allowed
	AllowMethods []string
	// AllowHeaders is a list of headers that are allowed
	AllowHeaders []string
	// ExposeHeaders is a list of headers that are exposed to the client
	ExposeHeaders []string
	// AllowCredentials indicates whether the request can include credentials
	AllowCredentials bool
	// MaxAge indicates how long the results of a preflight request can be cached
	MaxAge int
}

// DefaultCORSConfig returns the default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
		},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           0,
	}
}

// CORS returns a middleware that handles CORS
func CORS() MiddlewareFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig returns a middleware with custom configuration
func CORSWithConfig(config *CORSConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		config = DefaultCORSConfig()
	}
	if len(config.AllowOrigins) == 0 {
		config.AllowOrigins = []string{"*"}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = DefaultCORSConfig().AllowMethods
	}

	// Prepare allow methods header value
	allowMethods := strings.Join(config.AllowMethods, ", ")

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			req := c.Request()

			resp := c.Response()

			origin := req.Header.Get("Origin")

			// Check if origin is allowed
			allowOrigin := ""
			for _, o := range config.AllowOrigins {
				if o == "*" || o == origin {
					allowOrigin = o
					break
				}
			}

			// Simple request
			if req.Method != http.MethodOptions {
				if allowOrigin != "" {
					resp.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				}
				if config.AllowCredentials {
					resp.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if len(config.ExposeHeaders) > 0 {
					resp.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
				}
				return next(c)
			}

			// Preflight request
			resp.Header().Add("Vary", "Origin")
			resp.Header().Add("Vary", "Access-Control-Request-Method")
			resp.Header().Add("Vary", "Access-Control-Request-Headers")

			if allowOrigin == "" {
				resp.WriteHeader(http.StatusNoContent)
			return nil
			}

			// Handle preflight request
			resp.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			resp.Header().Set("Access-Control-Allow-Methods", allowMethods)

			if config.AllowCredentials {
				resp.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle allow headers
			if len(config.AllowHeaders) > 0 {
				allowHeaders := strings.Join(config.AllowHeaders, ", ")
				if config.AllowHeaders[0] == "*" {
					if requestHeaders := req.Header.Get("Access-Control-Request-Headers"); requestHeaders != "" {
						allowHeaders = requestHeaders
					}
				}
				resp.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			}

			// Set max age
			if config.MaxAge > 0 {
				resp.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
			}

			resp.WriteHeader(http.StatusNoContent)
			return nil
		}
	}
}

