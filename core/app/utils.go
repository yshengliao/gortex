package app

import (
	"fmt"
	"reflect"
	"strings"
	
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Convert CamelCase to kebab-case
func camelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// isValidGortexHandler checks if a method is a valid Gortex handler
func isValidGortexHandler(method reflect.Method) bool {
	// Check method signature: func(receiver, context.Context) error
	t := method.Type

	// Should have exactly 2 parameters (receiver + context)
	if t.NumIn() != 2 {
		return false
	}

	// First param is the receiver, second should be httpctx.Context
	contextType := reflect.TypeOf((*httpctx.Context)(nil)).Elem()
	if !t.In(1).Implements(contextType) {
		return false
	}

	// Should return exactly one value
	if t.NumOut() != 1 {
		return false
	}

	// Return value should be error
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if !t.Out(0).Implements(errorType) {
		return false
	}

	return true
}

// methodNameToPath converts a method name to a URL path
func methodNameToPath(name string) string {
	return "/" + camelToKebab(name)
}

// extractMiddlewareNames extracts middleware names from a slice of middleware functions
func extractMiddlewareNames(middlewares []middleware.MiddlewareFunc) []string {
	names := make([]string, 0, len(middlewares))
	for i, mw := range middlewares {
		if mw != nil {
			// Try to extract name from function type
			// Since Go doesn't provide function names at runtime, we use generic names
			names = append(names, fmt.Sprintf("middleware_%d", i))
		}
	}
	return names
}
