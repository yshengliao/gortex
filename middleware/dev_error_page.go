package middleware

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strings"

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
	Solutions      []string            `json:"solutions"`
	DocsLink       string              `json:"docs_link"`
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
		return func(c Context) error {
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
func extractErrorInfo(err error, c Context, config GortexDevErrorPageConfig) *ErrorInfo {
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

	// Generate solution suggestions
	errorInfo.Solutions = generateSolutions(err, errorInfo)
	errorInfo.DocsLink = "https://github.com/yshengliao/gortex/blob/main/README.md"

	return errorInfo
}

// generateSolutions provides helpful suggestions for common errors
func generateSolutions(err error, errorInfo *ErrorInfo) []string {
	var solutions []string
	errorMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errorMsg, "connection refused"):
		solutions = append(solutions, "Check if the target service is running")
		solutions = append(solutions, "Verify the host and port configuration")
		solutions = append(solutions, "Check firewall settings")

	case strings.Contains(errorMsg, "no such file or directory"):
		solutions = append(solutions, "Verify the file path is correct")
		solutions = append(solutions, "Check file permissions")
		solutions = append(solutions, "Ensure the file exists")

	case strings.Contains(errorMsg, "permission denied"):
		solutions = append(solutions, "Check file/directory permissions")
		solutions = append(solutions, "Run with appropriate user privileges")
		solutions = append(solutions, "Verify ownership of the resource")

	case strings.Contains(errorMsg, "bind: address already in use"):
		solutions = append(solutions, "Another process is using this port")
		solutions = append(solutions, "Use a different port number")
		solutions = append(solutions, "Stop the conflicting process")

	case strings.Contains(errorMsg, "panic"):
		solutions = append(solutions, "Check for nil pointer dereference")
		solutions = append(solutions, "Verify array/slice bounds")
		solutions = append(solutions, "Add proper error handling")

	case strings.Contains(errorMsg, "validation"):
		solutions = append(solutions, "Check input data format")
		solutions = append(solutions, "Verify required fields are present")
		solutions = append(solutions, "Review validation rules")

	case strings.Contains(errorMsg, "context deadline exceeded"):
		solutions = append(solutions, "Increase timeout value")
		solutions = append(solutions, "Check network connectivity")
		solutions = append(solutions, "Optimize slow operations")

	default:
		solutions = append(solutions, "Check the error message for specific details")
		solutions = append(solutions, "Review the stack trace for the error location")
		solutions = append(solutions, "Consult the Gortex documentation")
	}

	return solutions
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
func renderHTMLErrorPage(c Context, errorInfo *ErrorInfo) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Gortex Error - {{.Message}}</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root {
            --bg-color: #f8f9fa;
            --container-bg: #ffffff;
            --text-color: #212529;
            --error-header-bg: #dc3545;
            --error-message-bg: #f8d7da;
            --error-message-color: #721c24;
            --code-bg: #f8f9fa;
            --code-color: #495057;
            --border-color: #dee2e6;
            --notice-bg: #fff3cd;
            --notice-color: #856404;
            --solution-bg: #d1ecf1;
            --solution-color: #0c5460;
            --section-header-color: #495057;
        }
        
        @media (prefers-color-scheme: dark) {
            :root {
                --bg-color: #1a1a1a;
                --container-bg: #2d2d2d;
                --text-color: #e9ecef;
                --error-header-bg: #dc3545;
                --error-message-bg: #432424;
                --error-message-color: #f8d7da;
                --code-bg: #1e1e1e;
                --code-color: #d4edda;
                --border-color: #404040;
                --notice-bg: #664d03;
                --notice-color: #fff3cd;
                --solution-bg: #0c4a56;
                --solution-color: #b8daff;
                --section-header-color: #adb5bd;
            }
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            margin: 0;
            padding: 0;
            background-color: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        .error-header {
            background: linear-gradient(135deg, var(--error-header-bg), #b52d3a);
            color: white;
            padding: 40px;
            border-radius: 12px 12px 0 0;
            margin-bottom: 0;
            text-align: center;
        }
        .error-header h1 {
            margin: 0;
            font-size: 64px;
            font-weight: 300;
            text-shadow: 0 2px 4px rgba(0,0,0,0.3);
        }
        .error-header p {
            margin: 15px 0 0 0;
            font-size: 24px;
            opacity: 0.95;
            font-weight: 300;
        }
        .error-content {
            background-color: var(--container-bg);
            padding: 40px;
            border-radius: 0 0 12px 12px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
        }
        .error-message {
            background-color: var(--error-message-bg);
            color: var(--error-message-color);
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
            font-size: 16px;
            border-left: 4px solid var(--error-header-bg);
            word-break: break-word;
        }
        .section {
            margin-bottom: 35px;
        }
        .section h2 {
            font-size: 22px;
            margin-bottom: 20px;
            color: var(--section-header-color);
            font-weight: 600;
            border-bottom: 2px solid var(--border-color);
            padding-bottom: 8px;
        }
        .info-table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 10px;
        }
        .info-table td {
            padding: 12px;
            border-bottom: 1px solid var(--border-color);
            vertical-align: top;
        }
        .info-table td:first-child {
            font-weight: 600;
            width: 180px;
            color: var(--section-header-color);
            background-color: var(--code-bg);
        }
        .stack-trace {
            background-color: var(--code-bg);
            color: var(--code-color);
            padding: 20px;
            border-radius: 8px;
            overflow-x: auto;
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
            font-size: 13px;
            white-space: pre;
            border: 1px solid var(--border-color);
            line-height: 1.4;
        }
        .headers-table {
            width: 100%;
            font-size: 14px;
            border-collapse: collapse;
        }
        .headers-table td {
            padding: 8px 12px;
            border-bottom: 1px solid var(--border-color);
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
        }
        .headers-table td:first-child {
            font-weight: 600;
            background-color: var(--code-bg);
        }
        .dev-notice {
            background-color: var(--notice-bg);
            color: var(--notice-color);
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            border-left: 4px solid #ffc107;
            font-weight: 500;
        }
        .solutions {
            background-color: var(--solution-bg);
            color: var(--solution-color);
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #17a2b8;
        }
        .solutions h3 {
            margin-top: 0;
            margin-bottom: 15px;
            font-size: 18px;
        }
        .solutions ul {
            margin: 0;
            padding-left: 20px;
        }
        .solutions li {
            margin-bottom: 8px;
            line-height: 1.5;
        }
        .docs-link {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid var(--border-color);
        }
        .docs-link a {
            color: #007bff;
            text-decoration: none;
            font-weight: 500;
        }
        .docs-link a:hover {
            text-decoration: underline;
        }
        @media (max-width: 768px) {
            .container {
                padding: 10px;
            }
            .error-header {
                padding: 20px;
            }
            .error-header h1 {
                font-size: 48px;
            }
            .error-header p {
                font-size: 18px;
            }
            .error-content {
                padding: 20px;
            }
            .info-table td:first-child {
                width: 120px;
            }
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
                <strong>🔧 Development Mode:</strong> This detailed error page is only shown in development mode.
                In production, users will see a generic error message.
            </div>
            
            <div class="error-message">{{.Message}}</div>
            
            {{if .Solutions}}
            <div class="section">
                <div class="solutions">
                    <h3>💡 Possible Solutions</h3>
                    <ul>
                        {{range .Solutions}}
                        <li>{{.}}</li>
                        {{end}}
                    </ul>
                </div>
            </div>
            {{end}}
            
            {{if .RequestDetails}}
            <div class="section">
                <h2>📋 Request Information</h2>
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
                <h2>📤 Request Headers</h2>
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
                <h2>🔍 Stack Trace</h2>
                <div class="stack-trace">{{.StackTrace}}</div>
            </div>
            {{end}}
            
            <div class="docs-link">
                <p>📚 Need help? Check the <a href="{{.DocsLink}}" target="_blank">Gortex Documentation</a></p>
            </div>
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
		return func(c Context) (err error) {
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

					// Generate solutions for panic
					errorInfo.Solutions = generateSolutions(recoveredErr, errorInfo)
					errorInfo.DocsLink = "https://github.com/yshengliao/gortex/blob/main/README.md"

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
