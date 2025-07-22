package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

// DevErrorPageConfig defines the config for development error page middleware
type DevErrorPageConfig struct {
	// ShowStackTrace shows stack trace in error page
	ShowStackTrace bool

	// ShowRequestDetails shows request details in error page
	ShowRequestDetails bool

	// StackTraceLimit limits the number of stack frames to show
	StackTraceLimit int
}

// DefaultDevErrorPageConfig returns default config
var DefaultDevErrorPageConfig = DevErrorPageConfig{
	ShowStackTrace:     true,
	ShowRequestDetails: true,
	StackTraceLimit:    10,
}

// DevErrorPage returns a development error page middleware
func DevErrorPage() echo.MiddlewareFunc {
	return DevErrorPageWithConfig(DefaultDevErrorPageConfig)
}

// DevErrorPageWithConfig returns a development error page middleware with config
func DevErrorPageWithConfig(config DevErrorPageConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.StackTraceLimit == 0 {
		config.StackTraceLimit = DefaultDevErrorPageConfig.StackTraceLimit
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Capture original writer
			originalWriter := c.Response().Writer
			
			// Create response capture
			buf := new(bytes.Buffer)
			tee := io.MultiWriter(originalWriter, buf)
			c.Response().Writer = &devErrorResponseWriter{
				Writer:         tee,
				ResponseWriter: originalWriter,
				statusCode:     http.StatusOK,
				buffer:         buf,
			}

			// Defer to catch panics
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("panic: %v", r)
					
					// Get stack trace
					stackTrace := ""
					if config.ShowStackTrace {
						stackTrace = getStackTrace(config.StackTraceLimit)
					}

					// Clear buffer and render error page
					buf.Reset()
					c.Response().Writer = originalWriter
					renderErrorPage(c, http.StatusInternalServerError, err, stackTrace, config)
				}
			}()

			// Process request
			err := next(c)
			
			// Get actual status code
			rw := c.Response().Writer.(*devErrorResponseWriter)
			status := rw.statusCode
			
			// Check if we should render error page
			if status >= 400 && err == nil {
				// Try to recreate error from response
				var errorMsg string
				if buf.Len() > 0 {
					// Try to parse JSON error response
					var jsonResp map[string]any
					if json.Unmarshal(buf.Bytes(), &jsonResp) == nil {
						if errObj, ok := jsonResp["error"].(map[string]any); ok {
							if msg, ok := errObj["message"].(string); ok {
								errorMsg = msg
							}
						}
					}
				}
				if errorMsg == "" {
					errorMsg = http.StatusText(status)
				}
				err = fmt.Errorf("%s", errorMsg)
			}
			
			if err != nil || status >= 400 {
				// Check if client accepts HTML
				accept := c.Request().Header.Get("Accept")
				if strings.Contains(accept, "text/html") {
					// Get stack trace for 500 errors
					stackTrace := ""
					if status == http.StatusInternalServerError && config.ShowStackTrace {
						stackTrace = getStackTrace(config.StackTraceLimit)
					}

					// Clear buffer and render error page
					buf.Reset()
					c.Response().Writer = originalWriter
					c.Response().Committed = false
					renderErrorPage(c, status, err, stackTrace, config)
					return nil
				}
			}

			// Restore original writer
			c.Response().Writer = originalWriter
			
			return err
		}
	}
}

// devErrorResponseWriter wraps http.ResponseWriter to capture status code
type devErrorResponseWriter struct {
	io.Writer
	http.ResponseWriter
	statusCode int
	buffer     *bytes.Buffer
}

func (w *devErrorResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *devErrorResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.Writer.Write(b)
}

// getStackTrace returns the current stack trace
func getStackTrace(limit int) string {
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

// renderErrorPage renders a development error page
func renderErrorPage(c echo.Context, status int, err error, stackTrace string, config DevErrorPageConfig) {
	// Check if response was already written
	if c.Response().Committed {
		return
	}
	
	// Check if client accepts HTML
	accept := c.Request().Header.Get("Accept")
	if !strings.Contains(accept, "text/html") {
		// Return JSON error for non-HTML clients
		c.JSON(status, map[string]any{
			"error":   err.Error(),
			"status":  status,
			"path":    c.Request().URL.Path,
			"method":  c.Request().Method,
		})
		return
	}

	// Create error page data
	data := struct {
		Status             int
		StatusText         string
		Error              string
		Method             string
		Path               string
		QueryString        string
		RequestID          string
		StackTrace         string
		ShowStackTrace     bool
		ShowRequestDetails bool
		Headers            map[string]string
		RemoteIP           string
		UserAgent          string
	}{
		Status:             status,
		StatusText:         http.StatusText(status),
		Error:              err.Error(),
		Method:             c.Request().Method,
		Path:               c.Request().URL.Path,
		QueryString:        c.Request().URL.RawQuery,
		RequestID:          c.Response().Header().Get(echo.HeaderXRequestID),
		StackTrace:         stackTrace,
		ShowStackTrace:     config.ShowStackTrace && stackTrace != "",
		ShowRequestDetails: config.ShowRequestDetails,
		RemoteIP:           c.RealIP(),
		UserAgent:          c.Request().UserAgent(),
	}

	// Collect headers if showing request details
	if config.ShowRequestDetails {
		data.Headers = make(map[string]string)
		for key, values := range c.Request().Header {
			data.Headers[key] = strings.Join(values, ", ")
		}
	}

	// Render HTML error page
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	
	tmpl := template.Must(template.New("error").Parse(errorPageTemplate))
	tmpl.Execute(c.Response(), data)
}

// errorPageTemplate is the HTML template for error pages
const errorPageTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Error {{.Status}}: {{.StatusText}}</title>
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
            
            <div class="error-message">{{.Error}}</div>
            
            <div class="section">
                <h2>Request Information</h2>
                <table class="info-table">
                    <tr>
                        <td>Method</td>
                        <td>{{.Method}}</td>
                    </tr>
                    <tr>
                        <td>Path</td>
                        <td>{{.Path}}</td>
                    </tr>
                    {{if .QueryString}}
                    <tr>
                        <td>Query String</td>
                        <td>{{.QueryString}}</td>
                    </tr>
                    {{end}}
                    {{if .RequestID}}
                    <tr>
                        <td>Request ID</td>
                        <td>{{.RequestID}}</td>
                    </tr>
                    {{end}}
                    <tr>
                        <td>Remote IP</td>
                        <td>{{.RemoteIP}}</td>
                    </tr>
                    <tr>
                        <td>User Agent</td>
                        <td>{{.UserAgent}}</td>
                    </tr>
                </table>
            </div>
            
            {{if .ShowRequestDetails}}
            <div class="section">
                <h2>Request Headers</h2>
                <table class="headers-table">
                    {{range $key, $value := .Headers}}
                    <tr>
                        <td>{{$key}}</td>
                        <td>{{$value}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
            {{end}}
            
            {{if .ShowStackTrace}}
            <div class="section">
                <h2>Stack Trace</h2>
                <div class="stack-trace">{{.StackTrace}}</div>
            </div>
            {{end}}
        </div>
    </div>
</body>
</html>`