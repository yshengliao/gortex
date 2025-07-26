package app

import (
	gortexctx "github.com/yshengliao/gortex/context"
)

// Context type aliases for easier migration
type (
	// GortexContext is the new context interface
	GortexContext = gortexctx.Context
	
	// GortexHandlerFunc is the new handler function type
	GortexHandlerFunc = gortexctx.HandlerFunc
	
	// GortexMiddlewareFunc is the new middleware function type
	GortexMiddlewareFunc = gortexctx.MiddlewareFunc
)

