package context

import (
	"github.com/yshengliao/gortex/core/handler"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// Context type aliases for easier migration
type (
	// HTTPContext is the HTTP context interface
	HTTPContext = httpctx.Context

	// HandlerFunc is the handler function type
	HandlerFunc = handler.HandlerFunc

	// MiddlewareFunc is the middleware function type
	MiddlewareFunc = httpctx.MiddlewareFunc
)
