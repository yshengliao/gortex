package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/compat"
	"github.com/yshengliao/gortex/pkg/router"
)

// WrapForEcho converts a standard HTTP middleware to Echo middleware
func WrapForEcho(m router.Middleware) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		// Wrap the Echo handler to work with our middleware
		gortexHandler := compat.WrapEchoHandler(next)
		
		// Apply the middleware
		wrappedHandler := m(gortexHandler)
		
		// Convert back to Echo handler
		return compat.WrapGortexHandler(wrappedHandler)
	}
}

// WrapEchoMiddleware converts an Echo middleware to standard HTTP middleware
func WrapEchoMiddleware(m echo.MiddlewareFunc) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		// Wrap the Gortex handler to work with Echo middleware
		echoHandler := compat.WrapGortexHandler(next)
		
		// Apply the Echo middleware
		wrappedHandler := m(echoHandler)
		
		// Convert back to Gortex handler
		return compat.WrapEchoHandler(wrappedHandler)
	}
}