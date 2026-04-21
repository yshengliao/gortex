package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// CORSHandler wraps next with the CORS middleware using the default
// config. Unlike the MiddlewareFunc variant, this runs before the
// router and therefore handles preflight OPTIONS requests even when no
// route is registered for the target path.
func CORSHandler(next http.Handler) http.Handler {
	return CORSHandlerWithConfig(DefaultCORSConfig(), next)
}

// CORSHandlerWithConfig wraps next with CORS using the supplied config.
// Returns next unchanged and panics if the configuration is unsafe
// (wildcard origin + credentials): programmer error that should be
// caught at startup.
func CORSHandlerWithConfig(config *CORSConfig, next http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}
	if len(config.AllowOrigins) == 0 {
		config.AllowOrigins = []string{"*"}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = DefaultCORSConfig().AllowMethods
	}
	if err := config.Validate(); err != nil {
		panic(err)
	}
	allowMethods := strings.Join(config.AllowMethods, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowOrigin := ""
		for _, o := range config.AllowOrigins {
			if o == origin {
				allowOrigin = origin
				break
			}
			if o == "*" && !config.AllowCredentials {
				allowOrigin = "*"
			}
		}

		if r.Method != http.MethodOptions {
			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if len(config.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
			}
			next.ServeHTTP(w, r)
			return
		}

		// Preflight handled here, without forwarding to the router.
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")

		if allowOrigin == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", allowMethods)
		if config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if len(config.AllowHeaders) > 0 {
			allowHeaders := strings.Join(config.AllowHeaders, ", ")
			if config.AllowHeaders[0] == "*" {
				if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					allowHeaders = reqHeaders
				}
			}
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if config.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

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

// ErrCORSWildcardWithCredentials is returned when a CORS config combines
// a wildcard origin with AllowCredentials=true — an unsafe and spec-violating
// combination that browsers will reject.
var ErrCORSWildcardWithCredentials = errors.New("cors: cannot combine AllowOrigins \"*\" with AllowCredentials=true")

// Validate checks the configuration for unsafe combinations.
func (c *CORSConfig) Validate() error {
	if !c.AllowCredentials {
		return nil
	}
	for _, o := range c.AllowOrigins {
		if o == "*" {
			return ErrCORSWildcardWithCredentials
		}
	}
	return nil
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

// CORS returns a middleware that handles CORS with the default config.
// Panics if the resulting configuration is unsafe (e.g. wildcard origin
// with credentials) — this is a programmer error.
func CORS() MiddlewareFunc {
	mw, err := CORSWithConfig(DefaultCORSConfig())
	if err != nil {
		panic(err)
	}
	return mw
}

// CORSWithConfig returns a CORS middleware configured with config. It
// returns an error when the configuration is unsafe — callers should
// surface configuration errors at startup rather than silently ship
// insecure defaults.
func CORSWithConfig(config *CORSConfig) (MiddlewareFunc, error) {
	if config == nil {
		config = DefaultCORSConfig()
	}
	if len(config.AllowOrigins) == 0 {
		config.AllowOrigins = []string{"*"}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = DefaultCORSConfig().AllowMethods
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	allowMethods := strings.Join(config.AllowMethods, ", ")

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			req := c.Request()
			resp := c.Response()

			origin := req.Header.Get("Origin")

			// Determine which origin value to echo. When credentials are
			// enabled we must never respond with "*": echo the concrete
			// matching origin instead.
			allowOrigin := ""
			for _, o := range config.AllowOrigins {
				if o == origin {
					allowOrigin = origin
					break
				}
				if o == "*" && !config.AllowCredentials {
					allowOrigin = "*"
					// Keep scanning in case a concrete match follows.
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

			resp.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			resp.Header().Set("Access-Control-Allow-Methods", allowMethods)

			if config.AllowCredentials {
				resp.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(config.AllowHeaders) > 0 {
				allowHeaders := strings.Join(config.AllowHeaders, ", ")
				if config.AllowHeaders[0] == "*" {
					if requestHeaders := req.Header.Get("Access-Control-Request-Headers"); requestHeaders != "" {
						allowHeaders = requestHeaders
					}
				}
				resp.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			}

			if config.MaxAge > 0 {
				resp.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
			}

			resp.WriteHeader(http.StatusNoContent)
			return nil
		}
	}, nil
}
