package middleware

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strings"

	"github.com/yshengliao/gortex/context"
)

// GortexDevErrorPageConfig defines the config for development error page middleware
type GortexDevErrorPageConfig struct {
	// ShowStackTrace shows stack trace in error page
	ShowStackTrace bool

	// ShowRequestDetails shows request details in error page
	ShowRequestDetails bool

	// StackTraceLimit limits the number of stack frames to show
	StackTraceLimit int
}

// DefaultGortexDevErrorPageConfig returns default config
var DefaultGortexDevErrorPageConfig = GortexDevErrorPageConfig{
	ShowStackTrace:     true,
	ShowRequestDetails: true,
	StackTraceLimit:    10,
}

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Status         int                 `json:"status"`
	StatusText     string              `json:"status_text"`
	Message        string              `json:"message"`
	Type           string              `json:"type"`
	StackTrace     string              `json:"stack_trace"`
	RequestDetails map[string]string   `json:"request_details"`
	Headers        map[string][]string `json:"headers"`
}

// GortexDevErrorPage returns a development error page middleware
func GortexDevErrorPage() MiddlewareFunc {
	return GortexDevErrorPageWithConfig(DefaultGortexDevErrorPageConfig)
}

// GortexDevErrorPageWithConfig returns a development error page middleware with config
func GortexDevErrorPageWithConfig(config GortexDevErrorPageConfig) MiddlewareFunc {
	// Set defaults
	if config.StackTraceLimit == 0 {
		config.StackTraceLimit = DefaultGortexDevErrorPageConfig.StackTraceLimit
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c context.Context) error {
			// Execute the next handler
			err := next(c)

			// If no error, continue normally
			if err == nil {
				return nil
			}

			// Extract error information
			errorInfo := extractErrorInfo(err, c, config)

			// Check if client accepts HTML
			acceptHeader := c.Request().Header.Get("Accept")
			if strings.Contains(acceptHeader, "text/html") {
				return renderHTMLErrorPage(c, errorInfo)
			}

			// Return JSON error for API requests
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   errorInfo.Message,
				"details": errorInfo,
			})
		}
	}
}

// extractErrorInfo extracts detailed error information
func extractErrorInfo(err error, c context.Context, config GortexDevErrorPageConfig) *ErrorInfo {
	errorInfo := &ErrorInfo{
		Status:     http.StatusInternalServerError,
		StatusText: http.StatusText(http.StatusInternalServerError),
		Message:    err.Error(),
		Type:       fmt.Sprintf("%T", err),
	}

	// Extract stack trace if enabled
	if config.ShowStackTrace {
		errorInfo.StackTrace = getGortexStackTrace(config.StackTraceLimit)
	}

	// Extract request details if enabled
	if config.ShowRequestDetails {
		req := c.Request()
		errorInfo.RequestDetails = map[string]string{
			"method":      req.Method,
			"url":         req.URL.String(),
			"remote_addr": req.RemoteAddr,
			"user_agent":  req.UserAgent(),
			"referer":     req.Referer(),
		}
		errorInfo.Headers = req.Header
	}

	return errorInfo
}

// getGortexStackTrace captures the current stack trace
func getGortexStackTrace(limit int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Parse and limit stack frames
	lines := strings.Split(stack, "\n")
	if len(lines) > limit*2 { // Each frame is typically 2 lines
		lines = lines[:limit*2]
		lines = append(lines, "... (truncated)")
	}

	return strings.Join(lines, "\n")
}

// renderHTMLErrorPage renders an HTML error page
func renderHTMLErrorPage(c context.Context, errorInfo *ErrorInfo) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Gortex Error - {{.Message}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f5f5f5;
            color: #333;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        .error-header {
            background-color: #dc3545;
            color: white;
            padding: 30px;
            border-radius: 8px 8px 0 0;
            margin-bottom: 0;
        }
        .error-header h1 {
            margin: 0;
            font-size: 48px;
            font-weight: 300;
        }
        .error-header p {
            margin: 10px 0 0 0;
            font-size: 20px;
            opacity: 0.9;
        }
        .error-content {
            background-color: white;
            padding: 30px;
            border-radius: 0 0 8px 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .error-message {
            background-color: #f8d7da;
            color: #721c24;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
            font-family: monospace;
        }
        .section {
            margin-bottom: 30px;
        }
        .section h2 {
            font-size: 20px;
            margin-bottom: 15px;
            color: #495057;
        }
        .info-table {
            width: 100%;
            border-collapse: collapse;
        }
        .info-table td {
            padding: 8px;
            border-bottom: 1px solid #dee2e6;
        }
        .info-table td:first-child {
            font-weight: bold;
            width: 150px;
            color: #6c757d;
        }
        .stack-trace {
            background-color: #f8f9fa;
            padding: 15px;
            border-radius: 4px;
            overflow-x: auto;
            font-family: monospace;
            font-size: 12px;
            white-space: pre;
            color: #495057;
        }
        .headers-table {
            width: 100%;
            font-size: 14px;
        }
        .headers-table td {
            padding: 5px;
            border-bottom: 1px solid #dee2e6;
            font-family: monospace;
        }
        .dev-notice {
            background-color: #fff3cd;
            color: #856404;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="error-header">
            <h1>{{.Status}}</h1>
            <p>{{.StatusText}}</p>
        </div>
        <div class="error-content">
            <div class="dev-notice">
                <strong>Development Mode:</strong> This detailed error page is only shown in development mode.
            </div>
            
            <div class="error-message">{{.Message}}</div>
            
            {{if .RequestDetails}}
            <div class="section">
                <h2>Request Information</h2>
                <table class="info-table">
                    {{range $key, $value := .RequestDetails}}
                    <tr>
                        <td>{{$key}}</td>
                        <td>{{$value}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
            {{end}}
            
            {{if .Headers}}
            <div class="section">
                <h2>Request Headers</h2>
                <table class="headers-table">
                    {{range $name, $values := .Headers}}
                    {{range $values}}
                    <tr>
                        <td>{{$name}}</td>
                        <td>{{.}}</td>
                    </tr>
                    {{end}}
                    {{end}}
                </table>
            </div>
            {{end}}
            
            {{if .StackTrace}}
            <div class="section">
                <h2>Stack Trace</h2>
                <div class="stack-trace">{{.StackTrace}}</div>
            </div>
            {{end}}
        </div>
    </div>
</body>
</html>`

	t, err := template.New("error").Parse(tmpl)
	if err != nil {
		// Fallback to simple error response
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", errorInfo.Message))
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, errorInfo); err != nil {
		// Fallback to simple error response
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", errorInfo.Message))
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return c.String(http.StatusInternalServerError, buf.String())
}

// RecoverWithErrorPage is a recovery middleware that shows error pages
func RecoverWithErrorPage() MiddlewareFunc {
	return RecoverWithErrorPageConfig(DefaultGortexDevErrorPageConfig)
}

// RecoverWithErrorPageConfig is a recovery middleware with custom config
func RecoverWithErrorPageConfig(config GortexDevErrorPageConfig) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c context.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					var recoveredErr error
					switch v := r.(type) {
					case error:
						recoveredErr = v
					case string:
						recoveredErr = fmt.Errorf("%s", v)
					default:
						recoveredErr = fmt.Errorf("%v", v)
					}

					// Create error info for panic
					errorInfo := &ErrorInfo{
						Status:     http.StatusInternalServerError,
						StatusText: http.StatusText(http.StatusInternalServerError),
						Message:    recoveredErr.Error(),
						Type:       "panic",
					}

					if config.ShowStackTrace {
						errorInfo.StackTrace = getGortexStackTrace(config.StackTraceLimit)
					}

					if config.ShowRequestDetails {
						req := c.Request()
						errorInfo.RequestDetails = map[string]string{
							"method":      req.Method,
							"url":         req.URL.String(),
							"remote_addr": req.RemoteAddr,
							"user_agent":  req.UserAgent(),
							"referer":     req.Referer(),
						}
						errorInfo.Headers = req.Header
					}

					// Check if client accepts HTML
					acceptHeader := c.Request().Header.Get("Accept")
					if strings.Contains(acceptHeader, "text/html") {
						err = renderHTMLErrorPage(c, errorInfo)
					} else {
						err = c.JSON(http.StatusInternalServerError, map[string]interface{}{
							"error":   errorInfo.Message,
							"details": errorInfo,
						})
					}
				}
			}()

			return next(c)
		}
	}
}
