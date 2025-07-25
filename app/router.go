//go:build !production
// +build !production

package app

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// RegisterRoutes registers routes from a HandlersManager struct
// In development mode (default), this uses reflection for instant feedback
// In production mode (go build -tags production), this uses generated static registration
func RegisterRoutes(e *echo.Echo, manager any, ctx *Context) error {
	return RegisterRoutesFromStruct(e, manager, ctx)
}

// RegisterRoutesFromStruct registers routes from a struct using reflection
func RegisterRoutesFromStruct(e *echo.Echo, manager any, ctx *Context) error {
	return registerRoutesRecursive(e, manager, ctx, "")
}

// registerRoutesRecursive recursively registers routes from structs
func registerRoutesRecursive(e *echo.Echo, manager any, ctx *Context, pathPrefix string) error {
	return registerRoutesRecursiveWithMiddleware(e, manager, ctx, pathPrefix, []echo.MiddlewareFunc{})
}

// registerRoutesRecursiveWithMiddleware recursively registers routes with middleware inheritance
func registerRoutesRecursiveWithMiddleware(e *echo.Echo, manager any, ctx *Context, pathPrefix string, parentMiddleware []echo.MiddlewareFunc) error {
	v := reflect.ValueOf(manager)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("handlers must be a pointer to struct")
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
				if err := registerWebSocketHandler(e, fullPath, handler); err != nil {
					return fmt.Errorf("failed to register WebSocket handler %s: %w", field.Name, err)
				}
				continue
			}

			// 1. Register any HTTP methods defined directly on this struct (e.g., GET, POST, CustomMethod).
			if err := registerHTTPHandlerWithMiddleware(e, fullPath, handler, handlerType, currentMiddleware); err != nil {
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
				if err := registerRoutesRecursiveWithMiddleware(e, handler, ctx, fullPath, currentMiddleware); err != nil {
					return fmt.Errorf("failed to register nested routes for %s: %w", field.Name, err)
				}
			}
		}
	}

	return nil
}

// registerWebSocketHandler registers a WebSocket handler
func registerWebSocketHandler(e *echo.Echo, pattern string, handler any) error {
	// Look for HandleConnection method
	method := reflect.ValueOf(handler).MethodByName("HandleConnection")
	if !method.IsValid() {
		return fmt.Errorf("WebSocket handler must have HandleConnection method")
	}

	e.GET(pattern, func(c echo.Context) error {
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
func registerHTTPHandlerWithMiddleware(e *echo.Echo, basePath string, handler any, handlerType reflect.Type, middleware []echo.MiddlewareFunc) error {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		if m, ok := handlerType.MethodByName(method); ok {
			registerMethodWithMiddleware(e, method, basePath, handler, m, middleware)
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
		registerCustomMethodWithMiddleware(e, fullPath, handler, method, middleware)
	}

	return nil
}

// registerMethodWithMiddleware registers a standard HTTP method with middleware
func registerMethodWithMiddleware(e *echo.Echo, httpMethod, path string, handler any, method reflect.Method, middleware []echo.MiddlewareFunc) {
	handlerFunc := createHandlerFunc(handler, method)

	switch httpMethod {
	case "GET":
		e.GET(path, handlerFunc, middleware...)
	case "POST":
		e.POST(path, handlerFunc, middleware...)
	case "PUT":
		e.PUT(path, handlerFunc, middleware...)
	case "DELETE":
		e.DELETE(path, handlerFunc, middleware...)
	case "PATCH":
		e.PATCH(path, handlerFunc, middleware...)
	case "HEAD":
		e.HEAD(path, handlerFunc, middleware...)
	case "OPTIONS":
		e.OPTIONS(path, handlerFunc, middleware...)
	}
}

// registerCustomMethodWithMiddleware registers a custom method with middleware
func registerCustomMethodWithMiddleware(e *echo.Echo, path string, handler any, method reflect.Method, middleware []echo.MiddlewareFunc) {
	handlerFunc := createHandlerFunc(handler, method)
	e.POST(path, handlerFunc, middleware...)
}

// createHandlerFunc creates an echo.HandlerFunc from a reflect.Method
func createHandlerFunc(handler any, method reflect.Method) echo.HandlerFunc {
	return func(c echo.Context) error {
		args := []reflect.Value{reflect.ValueOf(handler), reflect.ValueOf(c)}
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
func parseMiddleware(tag string, ctx *Context) []echo.MiddlewareFunc {
	// This is a simple implementation that could be extended
	// to support multiple middleware separated by commas
	middlewares := []echo.MiddlewareFunc{}

	// Split by comma for multiple middleware
	names := strings.Split(tag, ",")
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// First try to look up middleware from registry in context
		if registry, err := Get[map[string]echo.MiddlewareFunc](ctx); err == nil {
			if mw, ok := registry[name]; ok {
				middlewares = append(middlewares, mw)
				continue
			}
		}

		// Fall back to predefined middleware
		switch name {
		case "auth":
			// Try to get auth middleware from context
			if authMW, err := Get[echo.MiddlewareFunc](ctx); err == nil {
				middlewares = append(middlewares, authMW)
			}
		case "logger":
			middlewares = append(middlewares, middleware.Logger())
		case "recover":
			middlewares = append(middlewares, middleware.Recover())
			// Add more predefined middleware as needed
		}
	}

	return middlewares
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

// Helper functions are now in utils.go
