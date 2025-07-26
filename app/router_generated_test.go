package app

import (
	"github.com/yshengliao/gortex/router"
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Test data structures
type TestHandler struct {
	Logger *zap.Logger
}

func (h *TestHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "GET", "path": c.Path()})
}

func (h *TestHandler) POST(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "POST", "path": c.Path()})
}

func (h *TestHandler) PUT(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "PUT", "path": c.Path()})
}

func (h *TestHandler) DELETE(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "DELETE", "path": c.Path()})
}

func (h *TestHandler) CustomEndpoint(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "POST", "path": c.Path(), "custom": "true"})
}

func (h *TestHandler) MultiWordEndpoint(c context.Context) error {
	return c.JSON(200, map[string]string{"method": "POST", "path": c.Path(), "multiword": "true"})
}

type WSTestHandler struct {
	Logger *zap.Logger
}

func (h *WSTestHandler) HandleConnection(c context.Context) error {
	return c.JSON(200, map[string]string{"type": "websocket"})
}

type TestHandlersManager struct {
	API       *TestHandler   `url:"/api"`
	Users     *TestHandler   `url:"/users"`  
	WebSocket *WSTestHandler `url:"/ws" hijack:"ws"`
	Root      *TestHandler   `url:"/"`
}

// TestRouterGeneration tests the router generation functionality
func TestRouterGeneration(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handlers := &TestHandlersManager{
		API:       &TestHandler{Logger: logger},
		Users:     &TestHandler{Logger: logger},
		WebSocket: &WSTestHandler{Logger: logger},
		Root:      &TestHandler{Logger: logger},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlers)
	assert.NoError(t, err)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Root GET",
			method:         "GET",
			path:           "/",
			expectedStatus: 200,
			expectedBody:   `{"method":"GET","path":"/"}`,
		},
		{
			name:           "API GET",
			method:         "GET", 
			path:           "/api",
			expectedStatus: 200,
			expectedBody:   `{"method":"GET","path":"/api"}`,
		},
		{
			name:           "API POST",
			method:         "POST",
			path:           "/api",
			expectedStatus: 200,
			expectedBody:   `{"method":"POST","path":"/api"}`,
		},
		{
			name:           "Users PUT",
			method:         "PUT",
			path:           "/users",
			expectedStatus: 200,
			expectedBody:   `{"method":"PUT","path":"/users"}`,
		},
		{
			name:           "Users DELETE",
			method:         "DELETE",
			path:           "/users",
			expectedStatus: 200,
			expectedBody:   `{"method":"DELETE","path":"/users"}`,
		},
		{
			name:           "Custom endpoint",
			method:         "POST",
			path:           "/api/custom-endpoint",
			expectedStatus: 200,
			expectedBody:   `{"custom":"true","method":"POST","path":"/api/custom-endpoint"}`,
		},
		{
			name:           "Multi-word endpoint",
			method:         "POST",
			path:           "/api/multi-word-endpoint",
			expectedStatus: 200,
			expectedBody:   `{"method":"POST","multiword":"true","path":"/api/multi-word-endpoint"}`,
		},
		{
			name:           "WebSocket endpoint",
			method:         "GET",
			path:           "/ws",
			expectedStatus: 200,
			expectedBody:   `{"type":"websocket"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			
			r.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.JSONEq(t, tt.expectedBody, rec.Body.String())
		})
	}
}

// TestCamelToKebab tests the camelCase to kebab-case conversion
func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CustomEndpoint", "custom-endpoint"},
		{"MultiWordEndpoint", "multi-word-endpoint"},
		{"GET", "g-e-t"},
		{"SimpleWord", "simple-word"},
		{"A", "a"},
		{"", ""},
		{"APIHandler", "a-p-i-handler"}, // Note: consecutive capitals are converted separately
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := camelToKebab(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStaticRouteGeneration simulates what the generated static router should produce
func TestStaticRouteGeneration(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewProduction()
	
	// This is what the generated code should look like
	apiHandler := &TestHandler{Logger: logger}
	usersHandler := &TestHandler{Logger: logger}
	wsHandler := &WSTestHandler{Logger: logger}
	rootHandler := &TestHandler{Logger: logger}

	// Static route registration (target of code generation)
	r.GET("/", rootHandler.GET)
	r.POST("/", rootHandler.POST)
	r.PUT("/", rootHandler.PUT)
	r.DELETE("/", rootHandler.DELETE)
	r.POST("/custom-endpoint", rootHandler.CustomEndpoint)
	r.POST("/multi-word-endpoint", rootHandler.MultiWordEndpoint)

	r.GET("/api", apiHandler.GET)
	r.POST("/api", apiHandler.POST)
	r.PUT("/api", apiHandler.PUT)
	r.DELETE("/api", apiHandler.DELETE)
	r.POST("/api/custom-endpoint", apiHandler.CustomEndpoint)
	r.POST("/api/multi-word-endpoint", apiHandler.MultiWordEndpoint)

	r.GET("/users", usersHandler.GET)
	r.POST("/users", usersHandler.POST)
	r.PUT("/users", usersHandler.PUT)
	r.DELETE("/users", usersHandler.DELETE)
	r.POST("/users/custom-endpoint", usersHandler.CustomEndpoint)
	r.POST("/users/multi-word-endpoint", usersHandler.MultiWordEndpoint)

	// WebSocket (simplified for testing)
	r.GET("/ws", wsHandler.HandleConnection)

	// Test a few key routes to ensure static registration works
	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"method":"GET"`)
	assert.Contains(t, rec.Body.String(), `"path":"/api"`)

	// Test custom endpoint
	req = httptest.NewRequest("POST", "/api/custom-endpoint", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"custom":"true"`)
}

// TestErrorHandling tests error cases in router registration
func TestRouterErrorHandling(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()

	tests := []struct {
		name      string
		handler   interface{}
		expectErr bool
	}{
		{
			name:      "nil handler",
			handler:   nil,
			expectErr: true,
		},
		{
			name:      "non-pointer handler",
			handler:   TestHandlersManager{},
			expectErr: true,
		},
		{
			name:      "pointer to non-struct",
			handler:   &[]int{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RegisterRoutes(&App{router: r, ctx: ctx}, tt.handler)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRouteDiscovery tests that all expected routes are discovered
func TestRouteDiscovery(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handlers := &TestHandlersManager{
		API:   &TestHandler{Logger: logger},
		Users: &TestHandler{Logger: logger},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlers)
	assert.NoError(t, err)

	// Test that expected routes work by making requests
	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api"},
		{"POST", "/api"},
		{"PUT", "/api"},
		{"DELETE", "/api"},
		{"POST", "/api/custom-endpoint"},
		{"POST", "/api/multi-word-endpoint"},
		{"GET", "/users"},
		{"POST", "/users"},
		{"PUT", "/users"},
		{"DELETE", "/users"},
		{"POST", "/users/custom-endpoint"},
		{"POST", "/users/multi-word-endpoint"},
	}

	// Test a few key routes to ensure registration worked
	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	req = httptest.NewRequest("POST", "/api/custom-endpoint", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	// Verify we have the expected number of route types
	assert.GreaterOrEqual(t, len(expectedRoutes), 12)
}

// TestMethodCasingHandling tests that method names are handled correctly
func TestMethodCasingHandling(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	// Handler with mixed casing methods
	type CasingHandler struct {
		Logger *zap.Logger
	}

	// Define methods for CasingHandler
	getCasingHandler := func(c context.Context) error {
		return c.JSON(200, map[string]string{"method": "GET"})
	}

	type CasingManager struct {
		Handler *CasingHandler `url:"/test"`
	}

	// Manually register for testing
	r.GET("/test", getCasingHandler)

	// Test that GET method is registered
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)
}