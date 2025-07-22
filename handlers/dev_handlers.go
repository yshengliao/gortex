package handlers

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

// DevHandlers provides development-only endpoints
type DevHandlers struct {
	Logger *zap.Logger
}

// Routes returns a debug endpoint showing all registered routes
func (h *DevHandlers) Routes(c echo.Context) error {
	e := c.Echo()
	routes := e.Routes()

	// Group routes by path
	routeMap := make(map[string][]string)
	for _, route := range routes {
		if methods, exists := routeMap[route.Path]; exists {
			routeMap[route.Path] = append(methods, route.Method)
		} else {
			routeMap[route.Path] = []string{route.Method}
		}
	}

	// Sort paths
	var paths []string
	for path := range routeMap {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Build route information
	type RouteInfo struct {
		Path    string   `json:"path"`
		Methods []string `json:"methods"`
	}

	var routeList []RouteInfo
	for _, path := range paths {
		methods := routeMap[path]
		sort.Strings(methods)
		routeList = append(routeList, RouteInfo{
			Path:    path,
			Methods: methods,
		})
	}

	// System information
	systemInfo := map[string]interface{}{
		"go_version":     runtime.Version(),
		"num_goroutines": runtime.NumGoroutine(),
		"num_cpu":        runtime.NumCPU(),
	}

	// Return debug information
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"total_routes": len(routeList),
		"routes":       routeList,
		"system":       systemInfo,
		"debug_mode":   true,
		"echo_debug":   e.Debug,
	})
}

// Error returns a test error page for development
func (h *DevHandlers) Error(c echo.Context) error {
	errorType := c.QueryParam("type")
	
	switch errorType {
	case "panic":
		panic("Development test panic")
	case "validation":
		return response.BadRequest(c, "Validation error test", map[string]string{
			"field1": "required",
			"field2": "invalid format",
		})
	case "unauthorized":
		return response.Unauthorized(c, "Authentication required")
	case "forbidden":
		return response.Forbidden(c, "Access denied")
	case "notfound":
		return response.NotFound(c, "Resource not found")
	case "conflict":
		return response.Conflict(c, "Resource conflict")
	case "internal":
		return fmt.Errorf("internal server error test")
	default:
		return response.Success(c, http.StatusOK, map[string]interface{}{
			"message": "Use ?type=<error_type> to test different error responses",
			"available_types": []string{
				"panic",
				"validation",
				"unauthorized",
				"forbidden",
				"notfound",
				"conflict",
				"internal",
			},
		})
	}
}

// Config returns the current configuration (with sensitive data masked)
func (h *DevHandlers) Config(c echo.Context) error {
	// Get config from context if available
	config := c.Get("config")
	if config == nil {
		return response.Success(c, http.StatusOK, map[string]string{
			"message": "No configuration loaded",
		})
	}

	// Mask sensitive configuration
	maskedConfig := maskSensitiveConfig(config)

	return response.Success(c, http.StatusOK, maskedConfig)
}

// maskSensitiveConfig masks sensitive configuration values
func maskSensitiveConfig(config interface{}) interface{} {
	// This is a simplified version - in production you'd want more sophisticated masking
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return config
	}

	sensitiveKeys := []string{
		"secret",
		"password",
		"key",
		"token",
		"credential",
	}

	masked := make(map[string]interface{})
	for key, value := range configMap {
		lowerKey := strings.ToLower(key)
		shouldMask := false
		
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitive) {
				shouldMask = true
				break
			}
		}

		if shouldMask {
			if str, ok := value.(string); ok && str != "" {
				masked[key] = "[MASKED]"
			} else {
				masked[key] = value
			}
		} else if nestedMap, ok := value.(map[string]interface{}); ok {
			masked[key] = maskSensitiveConfig(nestedMap)
		} else {
			masked[key] = value
		}
	}

	return masked
}