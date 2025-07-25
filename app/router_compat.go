package app

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// injectDependencies injects dependencies into handler fields
func injectDependencies(handler interface{}, ctx *Context) error {
	v := reflect.ValueOf(handler).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip if not settable
		if !fieldValue.CanSet() {
			continue
		}

		// Check for inject tag
		injectTag := field.Tag.Get("inject")
		if injectTag == "" {
			continue
		}

		// Try to get value from context based on type
		if ctx != nil {
			// Use reflection to get the service
			serviceType := fieldValue.Type()
			ctx.mu.RLock()
			if service, exists := ctx.services[serviceType]; exists {
				ctx.mu.RUnlock()
				fieldValue.Set(reflect.ValueOf(service))
			} else {
				ctx.mu.RUnlock()
			}
		}
		// Add more type injections as needed
	}

	return nil
}

// RegisterRoutesCompat registers routes using the router adapter for compatibility
func RegisterRoutesCompat(app *App, manager any) error {
	v := reflect.ValueOf(manager)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("handlers must be a pointer to struct")
	}

	t := v.Elem().Type()
	logger := app.logger

	// Iterate through all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Elem().Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get the URL tag
		urlTag := field.Tag.Get("url")
		if urlTag == "" {
			continue
		}

		// Check if it's a WebSocket handler
		hijackTag := field.Tag.Get("hijack")
		isWebSocket := hijackTag == "ws"

		handler := fieldValue.Interface()
		if handler == nil {
			continue
		}

		// Inject dependencies
		if err := injectDependencies(handler, app.ctx); err != nil {
			return fmt.Errorf("failed to inject dependencies for %s: %w", field.Name, err)
		}

		// Register handler methods
		if err := registerHandlerMethods(app, urlTag, handler, isWebSocket, logger); err != nil {
			return fmt.Errorf("failed to register handler %s: %w", field.Name, err)
		}

		if logger != nil {
			logger.Debug("Registered handler", 
				zap.String("field", field.Name), 
				zap.String("url", urlTag),
				zap.Bool("websocket", isWebSocket))
		}
	}

	return nil
}

// registerHandlerMethods registers all HTTP methods for a handler
func registerHandlerMethods(app *App, basePath string, handler any, isWebSocket bool, logger *zap.Logger) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// For WebSocket handlers, just register the WebSocket endpoint
	if isWebSocket {
		// Convert to Echo handler
		if echoHandler, ok := handler.(echo.HandlerFunc); ok {
			app.routerAdapter.RegisterEchoRoute("GET", basePath, echoHandler)
		} else {
			// Try to find a method that can serve as WebSocket handler
			for i := 0; i < handlerType.NumMethod(); i++ {
				method := handlerType.Method(i)
				if isWebSocketMethod(method) {
					methodFunc := handlerValue.Method(i).Interface()
					if echoFunc, ok := methodFunc.(func(echo.Context) error); ok {
						app.routerAdapter.RegisterEchoRoute("GET", basePath, echoFunc)
						break
					}
				}
			}
		}
		return nil
	}

	// Map of HTTP method names to their handler methods
	httpMethods := map[string]string{
		"GET":     "GET",
		"POST":    "POST",
		"PUT":     "PUT",
		"DELETE":  "DELETE",
		"PATCH":   "PATCH",
		"HEAD":    "HEAD",
		"OPTIONS": "OPTIONS",
	}

	methodsRegistered := 0

	// Check for standard HTTP method handlers
	for httpMethod, methodName := range httpMethods {
		method, exists := handlerType.MethodByName(methodName)
		if !exists {
			continue
		}

		// Verify it's a valid Echo handler
		if !isValidEchoHandler(method) {
			continue
		}

		// Get the actual method
		methodFunc := handlerValue.MethodByName(methodName).Interface()
		echoFunc, ok := methodFunc.(func(echo.Context) error)
		if !ok {
			continue
		}

		// Register with router adapter
		app.routerAdapter.RegisterEchoRoute(httpMethod, basePath, echoFunc)
		methodsRegistered++

		if logger != nil {
			logger.Debug("Registered route",
				zap.String("method", httpMethod),
				zap.String("path", basePath))
		}
	}

	// Also check for custom method names (non-HTTP methods become sub-paths)
	for i := 0; i < handlerType.NumMethod(); i++ {
		method := handlerType.Method(i)
		methodName := method.Name

		// Skip if it's a standard HTTP method
		if _, isHTTP := httpMethods[methodName]; isHTTP {
			continue
		}

		// Skip if not a valid handler
		if !isValidEchoHandler(method) {
			continue
		}

		// Convert method name to path (e.g., GetUser -> /user)
		subPath := methodNameToPath(methodName)
		fullPath := basePath + subPath

		// Get the actual method
		methodFunc := handlerValue.Method(i).Interface()
		echoFunc, ok := methodFunc.(func(echo.Context) error)
		if !ok {
			continue
		}

		// Try to determine HTTP method from name prefix
		httpMethod := "GET" // Default
		for prefix, method := range map[string]string{
			"Post":   "POST",
			"Put":    "PUT",
			"Delete": "DELETE",
			"Patch":  "PATCH",
		} {
			if strings.HasPrefix(methodName, prefix) {
				httpMethod = method
				break
			}
		}

		// Register with router adapter
		app.routerAdapter.RegisterEchoRoute(httpMethod, fullPath, echoFunc)
		methodsRegistered++

		if logger != nil {
			logger.Debug("Registered route",
				zap.String("method", httpMethod),
				zap.String("path", fullPath),
				zap.String("handler_method", methodName))
		}
	}

	if methodsRegistered == 0 && logger != nil {
		logger.Warn("No methods registered for handler", zap.String("path", basePath))
	}

	return nil
}

// Helper functions remain the same
func isValidEchoHandler(method reflect.Method) bool {
	// Should have exactly 2 parameters: receiver and echo.Context
	if method.Type.NumIn() != 2 {
		return false
	}

	// Second parameter should be echo.Context
	contextType := method.Type.In(1)
	if contextType.String() != "echo.Context" {
		return false
	}

	// Should return exactly one value: error
	if method.Type.NumOut() != 1 {
		return false
	}

	// Return type should be error
	errorType := method.Type.Out(0)
	return errorType.String() == "error"
}

func isWebSocketMethod(method reflect.Method) bool {
	// WebSocket methods typically have different signatures
	// This is a simplified check
	return method.Name == "WebSocket" || method.Name == "WS" || method.Name == "Hijack"
}

func methodNameToPath(name string) string {
	// Convert CamelCase to kebab-case
	var result strings.Builder
	for i, r := range name {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}
	return "/" + strings.ToLower(result.String())
}