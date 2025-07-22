//go:build production
// +build production

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
// In production mode (go build -tags production), this uses optimized reflection
func RegisterRoutes(e *echo.Echo, manager any, ctx *Context) error {
	return RegisterRoutesOptimized(e, manager, ctx)
}

// RegisterRoutesOptimized uses reflection but with optimizations for production
func RegisterRoutesOptimized(e *echo.Echo, manager any, ctx *Context) error {
	v := reflect.ValueOf(manager)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("handlers must be a pointer to struct")
	}

	structVal := v.Elem()
	structType := structVal.Type()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldVal := structVal.Field(i)

		if !fieldVal.IsValid() || fieldVal.IsNil() {
			continue
		}

		// Get URL pattern from tag
		urlTag := field.Tag.Get("url")
		if urlTag == "" {
			continue
		}

		// Check if it's a WebSocket handler
		isWebSocket := field.Tag.Get("hijack") == "ws"

		logger, _ := Get[*zap.Logger](ctx)
		logger.Info("Registering handler", 
			zap.String("field", field.Name), 
			zap.String("url", urlTag), 
			zap.Bool("websocket", isWebSocket))

		handler := fieldVal.Interface()
		handlerType := reflect.TypeOf(handler)

		if isWebSocket {
			// Register WebSocket handler
			if method, exists := handlerType.MethodByName("HandleConnection"); exists {
				handlerFunc := createOptimizedHandlerFunc(handler, method)
				e.GET(urlTag, handlerFunc)
			}
			continue
		}

		// Register HTTP methods with optimized handler creation
		httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		
		for _, methodName := range httpMethods {
			if method, exists := handlerType.MethodByName(methodName); exists {
				handlerFunc := createOptimizedHandlerFunc(handler, method)
				switch methodName {
				case "GET":
					e.GET(urlTag, handlerFunc)
				case "POST":
					e.POST(urlTag, handlerFunc)
				case "PUT":
					e.PUT(urlTag, handlerFunc)
				case "DELETE":
					e.DELETE(urlTag, handlerFunc)
				case "PATCH":
					e.PATCH(urlTag, handlerFunc)
				case "HEAD":
					e.HEAD(urlTag, handlerFunc)
				case "OPTIONS":
					e.OPTIONS(urlTag, handlerFunc)
				}
			}
		}

		// Register custom methods (sub-routes) with optimizations
		for j := 0; j < handlerType.NumMethod(); j++ {
			method := handlerType.Method(j)
			methodName := method.Name

			// Skip standard HTTP methods and special methods
			if contains(httpMethods, methodName) || !method.IsExported() || methodName == "HandleConnection" {
				continue
			}

			// Convert method name to route path
			routePath := camelToKebab(methodName)
			fullPath := strings.TrimSuffix(urlTag, "/") + "/" + routePath

			handlerFunc := createOptimizedHandlerFunc(handler, method)
			e.POST(fullPath, handlerFunc) // Custom methods default to POST
		}
	}

	return nil
}

// createOptimizedHandlerFunc creates an optimized echo.HandlerFunc
// This avoids some of the overhead of the reflection version
func createOptimizedHandlerFunc(handler any, method reflect.Method) echo.HandlerFunc {
	// Pre-compute the reflection values to avoid doing it on every request
	handlerVal := reflect.ValueOf(handler)
	methodFunc := method.Func
	
	return func(c echo.Context) error {
		// Call the method directly with pre-computed values
		results := methodFunc.Call([]reflect.Value{handlerVal, reflect.ValueOf(c)})
		
		if len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return err
			}
		}
		return nil
	}
}