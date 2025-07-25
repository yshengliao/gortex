package compat

import (
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/router"
)

// RuntimeMode defines the framework runtime mode
type RuntimeMode int

const (
	ModeEcho RuntimeMode = iota  // Use Echo (default for compatibility)
	ModeGortex                   // Use Gortex
	ModeDual                     // Dual system for testing
)

// EchoAdapter provides Echo Context to Gortex Context conversion
type EchoAdapter struct {
	router router.Router
	mode   RuntimeMode
}

// NewEchoAdapter creates a new Echo adapter
func NewEchoAdapter(r router.Router) *EchoAdapter {
	return &EchoAdapter{
		router: r,
		mode:   ModeEcho, // Default to Echo mode for compatibility
	}
}

// SetRuntimeMode sets the runtime mode
func (a *EchoAdapter) SetRuntimeMode(mode RuntimeMode) {
	a.mode = mode
}

// WrapEchoHandler converts Echo Handler to Gortex Handler
func WrapEchoHandler(h echo.HandlerFunc) router.HandlerFunc {
	return func(c router.Context) error {
		// Create Echo Context wrapper
		echoCtx := newEchoContextWrapper(c)
		return h(echoCtx)
	}
}

// WrapGortexHandler converts Gortex Handler to Echo Handler
func WrapGortexHandler(h router.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Create Gortex Context wrapper
		gortexCtx := &gortexContextWrapper{
			echoContext: c,
		}
		return h(gortexCtx)
	}
}


