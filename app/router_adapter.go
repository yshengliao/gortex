package app

import (
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/compat"
	"github.com/yshengliao/gortex/pkg/router"
)

// RouterAdapter allows switching between Echo and custom router
type RouterAdapter struct {
	mode        RuntimeMode
	echo        *echo.Echo
	gortex      router.Router
	echoAdapter *compat.EchoAdapter
}

// NewRouterAdapter creates a new router adapter
func NewRouterAdapter(mode RuntimeMode, e *echo.Echo) *RouterAdapter {
	adapter := &RouterAdapter{
		mode:   mode,
		echo:   e,
		gortex: router.NewRouter(),
	}
	
	if mode != ModeEcho {
		adapter.echoAdapter = compat.NewEchoAdapter(adapter.gortex)
		adapter.echoAdapter.SetRuntimeMode(mode)
	}
	
	return adapter
}

// RegisterEchoRoute registers a route that works with both routers
func (ra *RouterAdapter) RegisterEchoRoute(method, path string, handler echo.HandlerFunc) {
	if ra == nil {
		// If adapter is nil, we can't register routes
		return
	}
	
	switch ra.mode {
	case ModeEcho:
		// Use Echo directly
		switch method {
		case "GET":
			ra.echo.GET(path, handler)
		case "POST":
			ra.echo.POST(path, handler)
		case "PUT":
			ra.echo.PUT(path, handler)
		case "DELETE":
			ra.echo.DELETE(path, handler)
		case "PATCH":
			ra.echo.PATCH(path, handler)
		case "HEAD":
			ra.echo.HEAD(path, handler)
		case "OPTIONS":
			ra.echo.OPTIONS(path, handler)
		}
		
	case ModeGortex:
		// Convert Echo handler to Gortex
		gortexHandler := compat.WrapEchoHandler(handler)
		switch method {
		case "GET":
			ra.gortex.GET(path, gortexHandler)
		case "POST":
			ra.gortex.POST(path, gortexHandler)
		case "PUT":
			ra.gortex.PUT(path, gortexHandler)
		case "DELETE":
			ra.gortex.DELETE(path, gortexHandler)
		case "PATCH":
			ra.gortex.PATCH(path, gortexHandler)
		case "HEAD":
			ra.gortex.HEAD(path, gortexHandler)
		case "OPTIONS":
			ra.gortex.OPTIONS(path, gortexHandler)
		}
		
	case ModeDual:
		// Register on both for A/B testing
		ra.RegisterEchoRoute(method, path, handler)
		// Also register on Gortex
		gortexHandler := compat.WrapEchoHandler(handler)
		switch method {
		case "GET":
			ra.gortex.GET(path, gortexHandler)
		case "POST":
			ra.gortex.POST(path, gortexHandler)
		case "PUT":
			ra.gortex.PUT(path, gortexHandler)
		case "DELETE":
			ra.gortex.DELETE(path, gortexHandler)
		case "PATCH":
			ra.gortex.PATCH(path, gortexHandler)
		case "HEAD":
			ra.gortex.HEAD(path, gortexHandler)
		case "OPTIONS":
			ra.gortex.OPTIONS(path, gortexHandler)
		}
	}
}

// GetRouter returns the active router based on mode
func (ra *RouterAdapter) GetRouter() interface{} {
	switch ra.mode {
	case ModeEcho:
		return ra.echo
	case ModeGortex:
		return ra.gortex
	case ModeDual:
		// In dual mode, return Echo by default
		return ra.echo
	}
	return ra.echo
}

// GetGortexRouter returns the Gortex router (for testing)
func (ra *RouterAdapter) GetGortexRouter() router.Router {
	return ra.gortex
}