//go:build !production
// +build !production

package app

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
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

		// Get the URL tag
		urlTag := field.Tag.Get("url")
		if urlTag == "" {
			continue
		}

		// Check if it's a WebSocket handler
		hijackTag := field.Tag.Get("hijack")
		isWebSocket := hijackTag == "ws"

		handler := fieldValue.Interface()
		handlerType := reflect.TypeOf(handler)

		if logger != nil {
			logger.Info("Registering handler",
				zap.String("field", field.Name),
				zap.String("url", urlTag),
				zap.Bool("websocket", isWebSocket))
		}

		if isWebSocket {
			// Handle WebSocket
			if err := registerWebSocketHandler(e, urlTag, handler); err != nil {
				return fmt.Errorf("failed to register WebSocket handler %s: %w", field.Name, err)
			}
		} else {
			// Handle HTTP routes
			if err := registerHTTPHandler(e, urlTag, handler, handlerType); err != nil {
				return fmt.Errorf("failed to register HTTP handler %s: %w", field.Name, err)
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

// registerHTTPHandler registers HTTP handlers for standard methods
func registerHTTPHandler(e *echo.Echo, basePath string, handler any, handlerType reflect.Type) error {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		if m, ok := handlerType.MethodByName(method); ok {
			registerMethod(e, method, basePath, handler, m)
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

		// Register the route
		registerCustomMethod(e, fullPath, handler, method)
	}

	return nil
}

// registerMethod registers a standard HTTP method
func registerMethod(e *echo.Echo, httpMethod, path string, handler any, method reflect.Method) {
	handlerFunc := createHandlerFunc(handler, method)
	
	switch httpMethod {
	case "GET":
		e.GET(path, handlerFunc)
	case "POST":
		e.POST(path, handlerFunc)
	case "PUT":
		e.PUT(path, handlerFunc)
	case "DELETE":
		e.DELETE(path, handlerFunc)
	case "PATCH":
		e.PATCH(path, handlerFunc)
	case "HEAD":
		e.HEAD(path, handlerFunc)
	case "OPTIONS":
		e.OPTIONS(path, handlerFunc)
	}
}

// registerCustomMethod registers a custom method as POST by default
func registerCustomMethod(e *echo.Echo, path string, handler any, method reflect.Method) {
	handlerFunc := createHandlerFunc(handler, method)
	e.POST(path, handlerFunc)
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

// Helper functions are now in utils.go