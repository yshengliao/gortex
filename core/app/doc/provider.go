package doc

import (
	"net/http"
)

// RouteInfo represents information about a registered route
type RouteInfo struct {
	Method      string                 // HTTP method (GET, POST, etc.)
	Path        string                 // Route path (e.g., "/users/:id")
	Handler     string                 // Handler function name
	Params      []ParamInfo            // Path and query parameters
	Middleware  []string               // Applied middleware names
	Tags        []string               // API tags for grouping
	Description string                 // Route description
	Metadata    map[string]interface{} // Additional metadata from struct tags
}

// ParamInfo represents information about a route parameter
type ParamInfo struct {
	Name        string // Parameter name
	Type        string // Parameter type (path, query, header, body)
	DataType    string // Data type (string, int, bool, etc.)
	Required    bool   // Whether the parameter is required
	Description string // Parameter description
	Example     string // Example value
}

// DocProvider defines the interface for API documentation providers
type DocProvider interface {
	// Generate creates documentation from the provided routes
	Generate(routes []RouteInfo) ([]byte, error)

	// ContentType returns the MIME type of the generated documentation
	ContentType() string

	// UIHandler returns an HTTP handler for serving the documentation UI
	// Return nil if no UI is provided
	UIHandler() http.Handler

	// Endpoints returns the documentation endpoints to register
	// For example: {"/_docs": docHandler, "/_docs/ui": uiHandler}
	Endpoints() map[string]http.Handler
}

// HandlerMetadata represents metadata extracted from handler struct tags
type HandlerMetadata struct {
	Group       string   // API group (e.g., "User Management")
	Version     string   // API version (e.g., "v1")
	Description string   // Handler group description
	Tags        []string // Tags for categorization
	BasePath    string   // Base path for all routes in this handler
}

// DocConfig represents configuration for documentation generation
type DocConfig struct {
	Title       string // API title
	Version     string // API version
	Description string // API description
	BasePath    string // Base path for all APIs
	Servers     []ServerInfo
	Contact     *ContactInfo
	License     *LicenseInfo
}

// ServerInfo represents server information for the API
type ServerInfo struct {
	URL         string
	Description string
}

// ContactInfo represents contact information
type ContactInfo struct {
	Name  string
	Email string
	URL   string
}

// LicenseInfo represents license information
type LicenseInfo struct {
	Name string
	URL  string
}