//go:build !production
// +build !production

package app

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	appcontext "github.com/yshengliao/gortex/core/context"
	"github.com/yshengliao/gortex/core/handler"
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// RegisterRoutes registers routes from a HandlersManager struct
// In development mode (default), this uses reflection for instant feedback
// In production mode (go build -tags production), this uses generated static registration
func RegisterRoutes(app *App, manager any) error {
	// Clear existing route infos if logging is enabled
	if app.enableRoutesLog {
		app.routeInfos = make([]RouteLogInfo, 0)
	}
	return RegisterRoutesFromStruct(app.router, manager, app.ctx, app)
}

// RegisterRoutesFromStruct registers routes from a struct using reflection
func RegisterRoutesFromStruct(r httpctx.GortexRouter, manager any, ctx *appcontext.Context, app ...*App) error {
	var appInstance *App
	if len(app) > 0 {
		appInstance = app[0]
	}
	return registerRoutesRecursive(r, manager, ctx, "", appInstance)
}

// registerRoutesRecursive recursively registers routes from structs
func registerRoutesRecursive(r router.GortexRouter, manager any, ctx *Context, pathPrefix string, app *App) error {
	return registerRoutesRecursiveWithMiddleware(r, manager, ctx, pathPrefix, []gortexMiddleware.MiddlewareFunc{}, app)
}

// registerRoutesRecursiveWithMiddleware recursively registers routes with middleware inheritance
func registerRoutesRecursiveWithMiddleware(r router.GortexRouter, manager any, ctx *Context, pathPrefix string, parentMiddleware []gortexMiddleware.MiddlewareFunc, app *App) error {
	v := reflect.ValueOf(manager)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("handlers must be a pointer to struct")
	}

	// Auto-initialize the manager if needed
	if err := autoInitHandlers(v, true); err != nil {
		return fmt.Errorf("failed to auto-initialize handlers: %w", err)
	}

	// Inject dependencies
	if err := injectDependencies(v, ctx); err != nil {
		return fmt.Errorf("failed to inject dependencies: %w", err)
	}

	t := v.Elem().Type()
	logger, _ := Get[*zap.Logger](ctx)

	// Iterate through all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Elem().Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		handler := fieldValue.Interface()
		handlerType := reflect.TypeOf(handler)

		// Process only fields that are pointers to structs (our handlers/groups)
		if handlerType.Kind() == reflect.Ptr && handlerType.Elem().Kind() == reflect.Struct {
			// Get the URL tag, skip if not present
			urlTag := field.Tag.Get("url")
			if urlTag == "" {
				continue
			}

			// Combine with path prefix
			fullPath := pathPrefix + urlTag

			// Check for middleware tag on the group/handler and build the middleware chain.
			currentMiddleware := parentMiddleware
			if middlewareTag := field.Tag.Get("middleware"); middlewareTag != "" {
				if mw := parseMiddleware(middlewareTag, ctx); mw != nil {
					currentMiddleware = append(currentMiddleware, mw...)
				}
			}

			// Check for ratelimit tag
			if rateLimitTag := field.Tag.Get("ratelimit"); rateLimitTag != "" {
				if rlMiddleware := parseRateLimit(rateLimitTag, ctx); rlMiddleware != nil {
					currentMiddleware = append(currentMiddleware, rlMiddleware)
				}
			}

			// Check if it's a WebSocket handler.
			isWebSocket := field.Tag.Get("hijack") == "ws"

			if logger != nil {
				logger.Info("Processing handler/group",
					zap.String("field", field.Name),
					zap.String("url", fullPath),
					zap.Bool("websocket", isWebSocket))
			}

			if isWebSocket {
				// WebSocket handlers are terminal. Register and move to the next field.
				if err := registerWebSocketHandler(r, fullPath, handler); err != nil {
					return fmt.Errorf("failed to register WebSocket handler %s: %w", field.Name, err)
				}
				continue
			}

			// 1. Register any HTTP methods defined directly on this struct (e.g., GET, POST, CustomMethod).
			if err := registerHTTPHandlerWithMiddleware(r, fullPath, handler, handlerType, currentMiddleware, app); err != nil {
				return fmt.Errorf("failed to register HTTP handler %s: %w", field.Name, err)
			}

			// 2. Recursively process nested handler groups
			// Check if this is a group (has nested handlers with url tags)
			if isHandlerGroup(handler) {
				if logger != nil {
					logger.Info("Processing nested handler group",
						zap.String("field", field.Name),
						zap.String("prefix", fullPath))
				}
				if err := registerRoutesRecursiveWithMiddleware(r, handler, ctx, fullPath, currentMiddleware, app); err != nil {
					return fmt.Errorf("failed to register nested routes for %s: %w", field.Name, err)
				}
			}
		}
	}

	return nil
}

// registerWebSocketHandler registers a WebSocket handler
func registerWebSocketHandler(r router.GortexRouter, pattern string, handler any) error {
	// Look for HandleConnection method
	method := reflect.ValueOf(handler).MethodByName("HandleConnection")
	if !method.IsValid() {
		return fmt.Errorf("WebSocket handler must have HandleConnection method")
	}

	r.GET(pattern, func(c gortexContext.Context) error {
		// Call the HandleConnection method
		args := []reflect.Value{reflect.ValueOf(c)}
		results := method.Call(args)

		if len(results) > 0 && results[0].Interface() != nil {
			if err, ok := results[0].Interface().(error); ok {
				return err
			}
		}
		return nil
	})

	return nil
}

// registerHTTPHandlerWithMiddleware registers HTTP handlers with middleware
func registerHTTPHandlerWithMiddleware(r router.GortexRouter, basePath string, handler any, handlerType reflect.Type, middleware []gortexMiddleware.MiddlewareFunc, app *App) error {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		if m, ok := handlerType.MethodByName(method); ok {
			registerMethodWithMiddleware(r, method, basePath, handler, m, middleware, app)
		}
	}

	// Handle sub-routes
	for i := 0; i < handlerType.NumMethod(); i++ {
		method := handlerType.Method(i)
		methodName := method.Name

		// Skip standard HTTP methods
		if contains(methods, methodName) {
			continue
		}

		// Convert method name to route path
		routePath := camelToKebab(methodName)
		fullPath := strings.TrimSuffix(basePath, "/") + "/" + routePath

		// Register the route with proper parameter handling
		registerCustomMethodWithMiddleware(r, fullPath, handler, method, middleware, app)
	}

	return nil
}

// registerMethodWithMiddleware registers a standard HTTP method with middleware
func registerMethodWithMiddleware(r router.GortexRouter, httpMethod, path string, handler any, method reflect.Method, middleware []gortexMiddleware.MiddlewareFunc, app *App) {
	handlerFunc := createHandlerFunc(handler, method)

	// Collect route info if app is provided and logging is enabled
	if app != nil && app.enableRoutesLog {
		handlerName := reflect.TypeOf(handler).Elem().Name()
		var middlewareNames []string
		// TODO: Extract middleware names - for now just count them
		if len(middleware) > 0 {
			middlewareNames = []string{fmt.Sprintf("%d middleware", len(middleware))}
		} else {
			middlewareNames = []string{}
		}

		app.routeInfos = append(app.routeInfos, RouteLogInfo{
			Method:      httpMethod,
			Path:        path,
			Handler:     handlerName,
			Middlewares: middlewareNames,
		})
	}

	switch httpMethod {
	case "GET":
		r.GET(path, handlerFunc, middleware...)
	case "POST":
		r.POST(path, handlerFunc, middleware...)
	case "PUT":
		r.PUT(path, handlerFunc, middleware...)
	case "DELETE":
		r.DELETE(path, handlerFunc, middleware...)
	case "PATCH":
		r.PATCH(path, handlerFunc, middleware...)
	case "HEAD":
		r.HEAD(path, handlerFunc, middleware...)
	case "OPTIONS":
		r.OPTIONS(path, handlerFunc, middleware...)
	}
}

// registerCustomMethodWithMiddleware registers a custom method with middleware
func registerCustomMethodWithMiddleware(r router.GortexRouter, path string, handler any, method reflect.Method, middleware []gortexMiddleware.MiddlewareFunc, app *App) {
	handlerFunc := createHandlerFunc(handler, method)

	// Collect route info for custom methods
	if app != nil && app.enableRoutesLog {
		handlerName := reflect.TypeOf(handler).Elem().Name()
		var middlewareNames []string
		if len(middleware) > 0 {
			middlewareNames = []string{fmt.Sprintf("%d middleware", len(middleware))}
		} else {
			middlewareNames = []string{}
		}

		app.routeInfos = append(app.routeInfos, RouteLogInfo{
			Method:      "POST", // Custom methods are registered as POST
			Path:        path,
			Handler:     handlerName + "." + method.Name,
			Middlewares: middlewareNames,
		})
	}

	r.POST(path, handlerFunc, middleware...)
}

// createHandlerFunc creates a gortex.HandlerFunc from a reflect.Method
func createHandlerFunc(handler any, method reflect.Method) gortexMiddleware.HandlerFunc {
	// Check if method uses automatic parameter binding
	methodType := method.Type
	usesBinder := false

	// Check if method has parameters beyond receiver and gortex.Context
	if methodType.NumIn() > 2 {
		usesBinder = true
	}

	return func(c gortexContext.Context) error {
		// Get DI context from gortex context if available
		var diContext *Context
		if ctx := c.Get("di_context"); ctx != nil {
			if diCtx, ok := ctx.(*Context); ok {
				diContext = diCtx
			}
		}

		// Create parameter binder if needed
		var binder *ParameterBinder
		if usesBinder {
			if diContext != nil {
				binder = NewParameterBinderWithContext(diContext)
			} else {
				binder = NewParameterBinder()
			}
		}

		var args []reflect.Value

		if usesBinder {
			// Use parameter binder for automatic binding
			params, err := binder.BindMethodParams(c, method)
			if err != nil {
				return gortexContext.NewHTTPError(http.StatusBadRequest, err.Error())
			}
			args = append([]reflect.Value{reflect.ValueOf(handler)}, params...)
		} else {
			// Legacy mode: just pass gortex.Context
			args = []reflect.Value{reflect.ValueOf(handler), reflect.ValueOf(c)}
		}

		results := method.Func.Call(args)

		if len(results) > 0 && results[0].Interface() != nil {
			if err, ok := results[0].Interface().(error); ok {
				return err
			}
		}
		return nil
	}
}

// parseMiddleware parses middleware from tag string
func parseMiddleware(tag string, ctx *Context) []gortexMiddleware.MiddlewareFunc {
	// This is a simple implementation that could be extended
	// to support multiple middleware separated by commas
	middlewares := []gortexMiddleware.MiddlewareFunc{}

	// Split by comma for multiple middleware
	names := strings.Split(tag, ",")
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// First try to look up middleware from registry in context
		if registry, err := Get[map[string]gortexMiddleware.MiddlewareFunc](ctx); err == nil {
			if mw, ok := registry[name]; ok {
				middlewares = append(middlewares, mw)
				continue
			}
		}

		// Fall back to predefined middleware
		switch name {
		case "auth":
			// Try to get auth middleware from context
			if authMW, err := Get[gortexMiddleware.MiddlewareFunc](ctx); err == nil {
				middlewares = append(middlewares, authMW)
			}
		case "requestid":
			// Add request ID middleware
			middlewares = append(middlewares, gortexMiddleware.RequestID())
		case "recover":
			// Add recovery middleware with error page in dev mode
			if config, _ := Get[*Config](ctx); config != nil && config.Logger.Level == "debug" {
				middlewares = append(middlewares, gortexMiddleware.RecoverWithErrorPage())
			} else {
				// Simple recovery for production
				middlewares = append(middlewares, func(next gortexMiddleware.HandlerFunc) gortexMiddleware.HandlerFunc {
					return func(c gortexContext.Context) error {
						defer func() {
							if r := recover(); r != nil {
								c.Response().WriteHeader(http.StatusInternalServerError)
							}
						}()
						return next(c)
					}
				})
			}
		case "rbac":
			// Role-based access control would need to be configured
			if logger, _ := Get[*zap.Logger](ctx); logger != nil {
				logger.Warn("RBAC middleware requested but not configured", zap.String("middleware", name))
			}
		}
	}

	return middlewares
}

// parseRateLimit parses rate limit tag and returns rate limit middleware
func parseRateLimit(tag string, ctx *Context) gortexMiddleware.MiddlewareFunc {
	// Parse formats like "100/min", "10/sec", "1000/hour"
	parts := strings.Split(tag, "/")
	if len(parts) != 2 {
		if logger, _ := Get[*zap.Logger](ctx); logger != nil {
			logger.Warn("Invalid rate limit format", zap.String("tag", tag))
		}
		return nil
	}

	// Parse limit number
	limit, err := strconv.Atoi(parts[0])
	if err != nil {
		if logger, _ := Get[*zap.Logger](ctx); logger != nil {
			logger.Warn("Invalid rate limit number", zap.String("tag", tag), zap.Error(err))
		}
		return nil
	}

	// Parse time unit
	var burst int
	switch strings.ToLower(parts[1]) {
	case "sec", "second":
		burst = limit
	case "min", "minute":
		burst = limit / 60
		if burst < 1 {
			burst = 1
		}
	case "hour":
		burst = limit / 3600
		if burst < 1 {
			burst = 1
		}
	default:
		if logger, _ := Get[*zap.Logger](ctx); logger != nil {
			logger.Warn("Unknown rate limit time unit", zap.String("unit", parts[1]))
		}
		return nil
	}

	// Create rate limit config
	config := &gortexMiddleware.GortexRateLimitConfig{
		Rate:  limit,
		Burst: burst,
		SkipFunc: func(c gortexContext.Context) bool {
			// Skip rate limiting for local/internal requests
			remoteAddr := c.Request().RemoteAddr
			return strings.HasPrefix(remoteAddr, "127.0.0.1") || strings.HasPrefix(remoteAddr, "::1")
		},
	}

	return gortexMiddleware.GortexRateLimitWithConfig(config)
}

// isHandlerGroup checks if a handler is a group (has nested fields with url tags)
func isHandlerGroup(handler any) bool {
	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return false
	}

	t := v.Elem().Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Elem().Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Check if it's a pointer to struct with url tag
		if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			if fieldValue.Elem().Kind() == reflect.Struct && field.Tag.Get("url") != "" {
				return true
			}
		}
	}

	return false
}

// autoInitHandlers recursively initializes nil pointer fields in handlers
func autoInitHandlers(v reflect.Value, checkURLTag bool) error {
	// Handle pointer
	if v.Kind() == reflect.Ptr && v.IsNil() {
		if !v.CanSet() {
			return fmt.Errorf("cannot set nil pointer")
		}
		v.Set(reflect.New(v.Type().Elem()))
	}

	// Get the actual struct (dereference pointer if needed)
	elem := v
	if v.Kind() == reflect.Ptr {
		elem = v.Elem()
	}

	// Only process structs
	if elem.Kind() != reflect.Struct {
		return nil
	}

	// Recursively process all fields
	t := elem.Type()
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Check url tag if we're at the top level
		urlTag := fieldType.Tag.Get("url")
		shouldInit := !checkURLTag || urlTag != ""

		// Initialize nil pointer fields
		if field.Kind() == reflect.Ptr && field.IsNil() && shouldInit {
			// Only initialize if it's a struct pointer (handlers/groups)
			if field.Type().Elem().Kind() == reflect.Struct {
				field.Set(reflect.New(field.Type().Elem()))

				// Recursively initialize nested structures (don't check url tags in nested structs)
				if err := autoInitHandlers(field, false); err != nil {
					return fmt.Errorf("failed to auto-initialize field %s: %w", fieldType.Name, err)
				}
			}
		} else if field.Kind() == reflect.Ptr && !field.IsNil() {
			// Recursively process already initialized pointers
			if field.Type().Elem().Kind() == reflect.Struct {
				if err := autoInitHandlers(field, false); err != nil {
					return fmt.Errorf("failed to auto-initialize field %s: %w", fieldType.Name, err)
				}
			}
		}
	}

	return nil
}

// injectDependencies injects dependencies into handler fields with inject tag
func injectDependencies(v reflect.Value, ctx *Context) error {
	// Handle pointer
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	// Only process structs
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Check for inject tag
		if injectTag := fieldType.Tag.Get("inject"); injectTag != "" {
			// Try to inject from DI container
			if ctx != nil {
				ctx.mu.RLock()
				if service, ok := ctx.services[field.Type()]; ok {
					ctx.mu.RUnlock()
					field.Set(reflect.ValueOf(service))
				} else {
					ctx.mu.RUnlock()
					// If not in container and field is nil, try to create an instance
					if field.Kind() == reflect.Ptr && field.IsNil() {
						// Log warning but don't fail - allow partial injection
						if logger, _ := Get[*zap.Logger](ctx); logger != nil {
							logger.Warn("Service not found in DI container",
								zap.String("field", fieldType.Name),
								zap.String("type", field.Type().String()),
								zap.String("looking_for", field.Type().String()),
								zap.Any("available_types", getAvailableTypes(ctx)))
						}
					}
				}
			}
		}

		// Recursively process nested structs
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if field.Type().Elem().Kind() == reflect.Struct {
				if err := injectDependencies(field, ctx); err != nil {
					return fmt.Errorf("failed to inject dependencies in field %s: %w", fieldType.Name, err)
				}
			}
		} else if field.Kind() == reflect.Struct {
			if err := injectDependencies(field.Addr(), ctx); err != nil {
				return fmt.Errorf("failed to inject dependencies in field %s: %w", fieldType.Name, err)
			}
		}
	}

	return nil
}

// getAvailableTypes returns available types in the DI container for debugging
func getAvailableTypes(ctx *Context) []string {
	if ctx == nil {
		return nil
	}
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	types := make([]string, 0, len(ctx.services))
	for t := range ctx.services {
		types = append(types, t.String())
	}
	return types
}

// Helper functions are now in utils.go
