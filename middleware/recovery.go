package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"github.com/yshengliao/gortex/transport/http"
)

// RecoveryConfig contains configuration for the recovery middleware
type RecoveryConfig struct {
	// Logger is used to log panics
	Logger *zap.Logger
	// StackSize is the maximum size of the stack trace
	StackSize int
	// DisableStackAll disables the stack trace for all goroutines
	DisableStackAll bool
	// DisablePrintStack disables printing the stack trace
	DisablePrintStack bool
}

// DefaultRecoveryConfig returns the default configuration
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		Logger:            nil,
		StackSize:         4 << 10, // 4 KB
		DisableStackAll:   false,
		DisablePrintStack: false,
	}
}

// Recovery returns a middleware that recovers from panics
func Recovery() MiddlewareFunc {
	return RecoveryWithConfig(DefaultRecoveryConfig())
}

// RecoveryWithConfig returns a middleware with custom configuration
func RecoveryWithConfig(config *RecoveryConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		config = DefaultRecoveryConfig()
	}
	if config.StackSize == 0 {
		config.StackSize = 4 << 10
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}

					// Get stack trace
					stack := make([]byte, config.StackSize)
					length := runtime.Stack(stack, !config.DisableStackAll)
					stack = stack[:length]

					// Log the panic
					if config.Logger != nil {
						config.Logger.Error("Panic recovered",
							zap.Error(err),
							zap.String("stack", string(stack)),
							zap.String("path", getRequestPath(c)))
					}

					// Print stack trace if not disabled
					if !config.DisablePrintStack {
						fmt.Printf("[PANIC RECOVER] %v\n%s\n", err, stack)
					}

					// Return error response
					errResponse := map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "PANIC",
							"message": "Internal server error",
						},
					}

					// In development, include stack trace
					if config.Logger != nil && config.Logger.Level().Enabled(zap.DebugLevel) {
						errResponse["error"].(map[string]interface{})["details"] = map[string]interface{}{
							"panic": err.Error(),
							"stack": formatStack(stack),
						}
					}

					// Write error response
					if jsonErr := c.JSON(http.StatusInternalServerError, errResponse); jsonErr != nil && config.Logger != nil {
						config.Logger.Error("Failed to send panic response",
							zap.Error(jsonErr))
					}
				}
			}()

			return next(c)
		}
	}
}

// formatStack formats the stack trace for better readability
func formatStack(stack []byte) []string {
	lines := strings.Split(string(stack), "\n")
	formatted := make([]string, 0, len(lines))
	
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			// Skip runtime internals
			if strings.Contains(line, "runtime/") ||
				strings.Contains(line, "net/http/") ||
				strings.Contains(line, "github.com/labstack/echo") {
				continue
			}
			formatted = append(formatted, line)
		}
	}
	
	return formatted
}

// getRequestPath safely gets the request path
func getRequestPath(c Context) string {
	if req, ok := c.Request().(*http.Request); ok {
		return req.URL.Path
	}
	return "unknown"
}