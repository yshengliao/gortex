package swagger

import (
	"fmt"
	"net/http"
)

// SwaggerUIHandler serves the Swagger UI
type SwaggerUIHandler struct {
	swaggerJSONPath string
}

// NewSwaggerUIHandler creates a new Swagger UI handler
func NewSwaggerUIHandler(swaggerJSONPath string) *SwaggerUIHandler {
	return &SwaggerUIHandler{
		swaggerJSONPath: swaggerJSONPath,
	}
}

// ServeHTTP serves the Swagger UI HTML page
func (h *SwaggerUIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// For now, we'll serve a simple HTML page that uses the Swagger UI CDN
	// In production, you might want to embed the Swagger UI assets
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <title>API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "%s",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                validatorUrl: null,
                tryItOutEnabled: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                onComplete: function() {
                    console.log("Swagger UI loaded");
                }
            });
        };
    </script>
</body>
</html>`, h.swaggerJSONPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// EmbeddedSwaggerUI provides an option to use embedded Swagger UI assets
// This is a placeholder for future implementation where we might embed
// the Swagger UI distribution files directly in the binary
type EmbeddedSwaggerUI struct {
	// In a real implementation, this would contain embedded file system
	// with Swagger UI assets
}

// NewEmbeddedSwaggerUI creates a new embedded Swagger UI handler
func NewEmbeddedSwaggerUI() *EmbeddedSwaggerUI {
	// TODO: Implement embedded Swagger UI using embed package
	return &EmbeddedSwaggerUI{}
}

// Handler returns an HTTP handler for the embedded Swagger UI
func (e *EmbeddedSwaggerUI) Handler(swaggerJSONPath string) http.Handler {
	// For now, fallback to the CDN version
	return NewSwaggerUIHandler(swaggerJSONPath)
}