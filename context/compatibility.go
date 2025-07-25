package context

import (
	"github.com/labstack/echo/v4"
)

// EchoContextToGortex converts an Echo context to a Gortex context
func EchoContextToGortex(c echo.Context) Context {
	return &EchoContextAdapter{echoCtx: c}
}

// GortexContextToEcho converts a Gortex context to an Echo context
func GortexContextToEcho(c Context) echo.Context {
	// If it's already an Echo context adapter, return the underlying Echo context
	if adapter, ok := c.(*EchoContextAdapter); ok {
		return adapter.echoCtx
	}
	
	// Otherwise, create an adapter
	return &GortexContextAdapter{ctx: c}
}

// WrapEchoHandler wraps an Echo handler to use Gortex context
func WrapEchoHandler(h echo.HandlerFunc) HandlerFunc {
	return func(c Context) error {
		echoCtx := GortexContextToEcho(c)
		return h(echoCtx)
	}
}

// WrapGortexHandler wraps a Gortex handler to use Echo context
func WrapGortexHandler(h HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		gortexCtx := EchoContextToGortex(c)
		return h(gortexCtx)
	}
}

// IsGortexContext checks if the given interface is a Gortex context
func IsGortexContext(i interface{}) bool {
	_, ok := i.(Context)
	return ok
}

// IsEchoContext checks if the given interface is an Echo context
func IsEchoContext(i interface{}) bool {
	_, ok := i.(echo.Context)
	return ok
}