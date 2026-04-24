
package app

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	appcontext "github.com/yshengliao/gortex/core/context"
	"github.com/yshengliao/gortex/core/app/doc"
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// RegisterRoutes registers routes from a HandlersManager struct
// In development mode (default), this uses reflection for instant feedback
// In production mode (go build -tags production), this uses generated static registration
func RegisterRoutes(app *App, manager any) error {
	// Populate routeInfos whenever logging is enabled OR the app is in
	// development mode (so /_routes and /_monitor can show real data).
	if app.enableRoutesLog || app.developmentMode {
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
func registerRoutesRecursive(r httpctx.GortexRouter, manager any, ctx *appcontext.Context, pathPrefix string, app *App) error {
	return registerRoutesRecursiveWithMiddleware(r, manager, ctx, pathPrefix, []middleware.MiddlewareFunc{}, app)
}

// registerRoutesRecursiveWithMiddleware recursively registers routes with middleware inheritance
func registerRoutesRecursiveWithMiddleware(r httpctx.GortexRouter, manager any, ctx *appcontext.Context, pathPrefix string, parentMiddleware []middleware.MiddlewareFunc, app *App) error {
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
	logger, _ := appcontext.Get[*zap.Logger](ctx)

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
func registerWebSocketHandler(r httpctx.GortexRouter, pattern string, handler any) error {
	// Look for HandleConnection method
	method := reflect.ValueOf(handler).MethodByName("HandleConnection")
	if !method.IsValid() {
		return fmt.Errorf("WebSocket handler must have HandleConnection method")
	}

	r.GET(pattern, func(c httpctx.Context) error {
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
func registerHTTPHandlerWithMiddleware(r httpctx.GortexRouter, basePath string, handler any, handlerType reflect.Type, middleware []middleware.MiddlewareFunc, app *App) error {
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
func registerMethodWithMiddleware(r httpctx.GortexRouter, httpMethod, path string, handler any, method reflect.Method, middleware []middleware.MiddlewareFunc, app *App) {
	handlerFunc := createHandlerFunc(handler, method)

	// Collect route info if app is provided, logging is enabled, or dev mode is on.
	if app != nil && (app.enableRoutesLog || app.developmentMode) {
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
	
	// Collect documentation info if app has doc provider
	if app != nil && app.docProvider != nil {
		handlerType := reflect.TypeOf(handler).Elem()
		routeInfo := doc.RouteInfo{
			Method:      httpMethod,
			Path:        path,
			Handler:     handlerType.Name() + "." + method.Name,
			Middleware:  extractMiddlewareNames(middleware),
			Description: fmt.Sprintf("%s %s", httpMethod, path),
			Metadata:    make(map[string]interface{}),
		}
		app.AddDocumentationRoute(routeInfo)
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

// registerCustomMethodWithMiddleware registers a non-standard handler method as
// an HTTP POST route.
//
// # Naming convention
//
// Any Go method whose name does not match a standard HTTP verb (GET, POST, PUT,
// DELETE, PATCH, HEAD, OPTIONS) is treated as a "custom" method. It is always
// registered as HTTP POST at <basePath>/<kebab-case-method-name>, regardless
// of what the Go method name implies.
//
// Example:
//
//	type UserHandler struct{}
//	func (h *UserHandler) GetProfile(c types.Context) error { ... }
//	// → registered as: POST /users/get-profile   (NOT GET /users/get-profile)
//
// If you need a specific HTTP method for a custom endpoint, define the method
// using a standard name (GET, POST, …) or add a `method` struct tag in a
// future Gortex version once that tag is implemented.
func registerCustomMethodWithMiddleware(r httpctx.GortexRouter, path string, handler any, method reflect.Method, middleware []middleware.MiddlewareFunc, app *App) {
	handlerFunc := createHandlerFunc(handler, method)

	// Collect route info for custom methods (same condition as standard methods).
	if app != nil && (app.enableRoutesLog || app.developmentMode) {
		handlerName := reflect.TypeOf(handler).Elem().Name()
		var middlewareNames []string
		if len(middleware) > 0 {
			middlewareNames = []string{fmt.Sprintf("%d middleware", len(middleware))}
		} else {
			middlewareNames = []string{}
		}

		app.routeInfos = append(app.routeInfos, RouteLogInfo{
			Method:      "POST", // Custom methods are always registered as POST.
			Path:        path,
			Handler:     handlerName + "." + method.Name,
			Middlewares: middlewareNames,
		})
	}

	// Collect documentation info if app has doc provider.
	if app != nil && app.docProvider != nil {
		handlerType := reflect.TypeOf(handler).Elem()
		routeInfo := doc.RouteInfo{
			Method:      "POST", // Custom methods are always registered as POST.
			Path:        path,
			Handler:     handlerType.Name() + "." + method.Name,
			Middleware:  extractMiddlewareNames(middleware),
			Description: fmt.Sprintf("POST %s (custom method: %s)", path, method.Name),
			Metadata:    make(map[string]interface{}),
		}
		app.AddDocumentationRoute(routeInfo)
	}

	r.POST(path, handlerFunc, middleware...)
}


// createHandlerFunc creates a gortex.HandlerFunc from a reflect.Method
func createHandlerFunc(handler any, method reflect.Method) middleware.HandlerFunc {
	// Check if method uses automatic parameter binding
	methodType := method.Type
	usesBinder := false

	// Check if method has parameters beyond receiver and gortex.Context
	if methodType.NumIn() > 2 {
		usesBinder = true
	}

	return func(c httpctx.Context) error {
		// Get DI context from gortex context if available
		var diContext *appcontext.Context
		if ctx := c.Get("di_context"); ctx != nil {
			if diCtx, ok := ctx.(*appcontext.Context); ok {
				diContext = diCtx
			}
		}

		// Create parameter binder if needed
		var binder *appcontext.ParameterBinder
		if usesBinder {
			if diContext != nil {
				binder = appcontext.NewParameterBinderWithContext(diContext)
			} else {
				binder = appcontext.NewParameterBinder()
			}
		}

		var args []reflect.Value

		if usesBinder {
			// Use parameter binder for automatic binding
			params, err := binder.BindMethodParams(c, method)
			if err != nil {
				return httpctx.NewHTTPError(http.StatusBadRequest, err.Error())
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
func parseMiddleware(tag string, ctx *appcontext.Context) []middleware.MiddlewareFunc {
	// This is a simple implementation that could be extended
	// to support multiple middleware separated by commas
	middlewares := []middleware.MiddlewareFunc{}

	// Split by comma for multiple middleware
	names := strings.Split(tag, ",")
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// First try to look up middleware from registry in context
		if registry, err := appcontext.Get[map[string]middleware.MiddlewareFunc](ctx); err == nil {
			if mw, ok := registry[name]; ok {
				middlewares = append(middlewares, mw)
				continue
			}
		}

		// Fall back to predefined middleware
		switch name {
		case "auth":
			// Try to get auth middleware from context
			if authMW, err := appcontext.Get[middleware.MiddlewareFunc](ctx); err == nil {
				middlewares = append(middlewares, authMW)
			}
		case "requestid":
			// Add request ID middleware
			middlewares = append(middlewares, middleware.RequestID())
		case "recover":
			// Add recovery middleware with error page in dev mode
			if config, _ := appcontext.Get[*Config](ctx); config != nil && config.Logger.Level == "debug" {
				middlewares = append(middlewares, middleware.RecoverWithErrorPage())
			} else {
				// Simple recovery for production
				middlewares = append(middlewares, func(next middleware.HandlerFunc) middleware.HandlerFunc {
					return func(c httpctx.Context) error {
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
			if logger, _ := appcontext.Get[*zap.Logger](ctx); logger != nil {
				logger.Warn("RBAC middleware requested but not configured", zap.String("middleware", name))
			}
		}
	}

	return middlewares
}

// parseRateLimit parses rate limit tag and returns rate limit middleware
func parseRateLimit(tag string, ctx *appcontext.Context) middleware.MiddlewareFunc {
	// Parse formats like "100/min", "10/sec", "1000/hour"
	parts := strings.Split(tag, "/")
	if len(parts) != 2 {
		if logger, _ := appcontext.Get[*zap.Logger](ctx); logger != nil {
			logger.Warn("Invalid rate limit format", zap.String("tag", tag))
		}
		return nil
	}

	// Parse limit number
	limit, err := strconv.Atoi(parts[0])
	if err != nil {
		if logger, _ := appcontext.Get[*zap.Logger](ctx); logger != nil {
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
		if logger, _ := appcontext.Get[*zap.Logger](ctx); logger != nil {
			logger.Warn("Unknown rate limit time unit", zap.String("unit", parts[1]))
		}
		return nil
	}

	// Create rate limit config
	config := &middleware.GortexRateLimitConfig{
		Rate:  limit,
		Burst: burst,
		SkipFunc: func(c httpctx.Context) bool {
			// Skip rate limiting for local/internal requests
			remoteAddr := c.Request().RemoteAddr
			return strings.HasPrefix(remoteAddr, "127.0.0.1") || strings.HasPrefix(remoteAddr, "::1")
		},
	}

	return middleware.GortexRateLimitWithConfig(config)
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

// injectDependencies is a placeholder for future DI support.
//
// It inspects struct fields tagged with `inject` but does NOT perform actual
// injection — the feature is not yet implemented. If a field with an `inject`
// tag is nil, an error is returned so the caller fails loudly rather than
// proceeding with a nil pointer that would panic later.
func injectDependencies(v reflect.Value, ctx *appcontext.Context) error {
	// Handle pointer
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	// Only process structs
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Fast path: skip if no field in this struct (non-recursive) has
	// an `inject` tag. This is the common case and avoids the
	// per-field Lookup overhead for every route registration.
	if !hasInjectTag(v.Type()) {
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

		// Check for inject tag. Use Lookup so that `inject:""` (empty value)
		// is detected — Get("inject") would return "" for both a missing tag
		// and an empty-value tag, making them indistinguishable.
		if _, hasInject := fieldType.Tag.Lookup("inject"); hasInject {
			// Injection is not yet implemented. If the field is still nil,
			// return an error to prevent a later nil-pointer panic.
			if field.Kind() == reflect.Ptr && field.IsNil() {
				return fmt.Errorf(
					"field %s.%s has `inject` tag but DI injection is not implemented; "+
						"set the field manually before calling RegisterRoutes or remove the `inject` tag",
					t.Name(), fieldType.Name,
				)
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

// hasInjectTag reports whether any exported field of t carries an
// `inject` struct tag. Used as a fast-path check so that the
// reflection walk can be skipped entirely for handler structs that
// don't use DI.
func hasInjectTag(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if _, ok := f.Tag.Lookup("inject"); ok {
			return true
		}
	}
	return false
}

// Helper functions are now in utils.go
