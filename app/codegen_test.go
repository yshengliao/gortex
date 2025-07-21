package app

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Handler types are now defined in handlers.go

// Create a test file with HandlersManager
func createTestFile(t *testing.T) string {
	content := `package main

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type HandlersManager struct {
	API       *TestCodegenHandler   ` + "`url:\"/api\"`" + `
	Users     *TestCodegenHandler   ` + "`url:\"/users\"`" + `
	WebSocket *TestCodegenWSHandler ` + "`url:\"/ws\" hijack:\"ws\"`" + `
	Root      *TestCodegenHandler   ` + "`url:\"/\"`" + `
}

type TestCodegenHandler struct {
	Logger *zap.Logger
}

func (h *TestCodegenHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "GET"})
}

func (h *TestCodegenHandler) POST(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "POST"})
}

func (h *TestCodegenHandler) CustomAction(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "CustomAction"})
}

type TestCodegenWSHandler struct {
	Logger *zap.Logger
}

func (h *TestCodegenWSHandler) HandleConnection(c echo.Context) error {
	return c.JSON(200, map[string]string{"type": "websocket"})
}
`

	file, err := os.CreateTemp("", "test_handlers_*.go")
	assert.NoError(t, err)
	
	_, err = file.WriteString(content)
	assert.NoError(t, err)
	file.Close()

	return file.Name()
}

func TestRouteCodeGenerator_AnalyzeHandlersFromAST(t *testing.T) {
	testFile := createTestFile(t)
	defer os.Remove(testFile)

	generator := NewRouteCodeGenerator("app")
	err := generator.AnalyzeHandlersFromAST(testFile)
	assert.NoError(t, err)

	// Should have found 4 handlers
	assert.Len(t, generator.Handlers, 4)

	// Check each handler
	expectedHandlers := map[string]struct {
		URLPattern  string
		IsWebSocket bool
		TypeName    string
	}{
		"API":       {"/api", false, "TestCodegenHandler"},
		"Users":     {"/users", false, "TestCodegenHandler"},
		"WebSocket": {"/ws", true, "TestCodegenWSHandler"},
		"Root":      {"/", false, "TestCodegenHandler"},
	}

	for _, handler := range generator.Handlers {
		expected, exists := expectedHandlers[handler.Name]
		assert.True(t, exists, "Unexpected handler: %s", handler.Name)
		assert.Equal(t, expected.URLPattern, handler.URLPattern)
		assert.Equal(t, expected.IsWebSocket, handler.IsWebSocket)
		assert.Equal(t, expected.TypeName, handler.TypeName)
	}
}

func TestRouteCodeGenerator_AnalyzeHandlerMethods(t *testing.T) {
	generator := NewRouteCodeGenerator("app")
	
	// Add handlers manually for testing
	generator.Handlers = []HandlerInfo{
		{
			Name:        "API",
			URLPattern:  "/api",
			IsWebSocket: false,
			TypeName:    "TestCodegenHandler",
			Methods:     []MethodInfo{},
		},
		{
			Name:        "WebSocket",
			URLPattern:  "/ws",
			IsWebSocket: true,
			TypeName:    "TestCodegenWSHandler",
			Methods:     []MethodInfo{},
		},
	}

	// Analyze methods for HTTP handler
	err := generator.AnalyzeHandlerMethods(&TestCodegenHandler{})
	assert.NoError(t, err)

	// Find the API handler
	var apiHandler *HandlerInfo
	for i := range generator.Handlers {
		if generator.Handlers[i].Name == "API" {
			apiHandler = &generator.Handlers[i]
			break
		}
	}

	assert.NotNil(t, apiHandler)
	assert.Len(t, apiHandler.Methods, 3) // GET, POST, CustomAction

	// Check standard HTTP methods
	foundMethods := make(map[string]MethodInfo)
	for _, method := range apiHandler.Methods {
		foundMethods[method.Name] = method
	}

	assert.Contains(t, foundMethods, "GET")
	assert.Equal(t, "GET", foundMethods["GET"].HTTPMethod)
	assert.Equal(t, "/api", foundMethods["GET"].Path)

	assert.Contains(t, foundMethods, "POST")
	assert.Equal(t, "POST", foundMethods["POST"].HTTPMethod)
	assert.Equal(t, "/api", foundMethods["POST"].Path)

	assert.Contains(t, foundMethods, "CustomAction")
	assert.Equal(t, "POST", foundMethods["CustomAction"].HTTPMethod)
	assert.Equal(t, "/api/custom-action", foundMethods["CustomAction"].Path)

	// Analyze WebSocket handler
	err = generator.AnalyzeHandlerMethods(&TestCodegenWSHandler{})
	assert.NoError(t, err)

	// Find the WebSocket handler
	var wsHandler *HandlerInfo
	for i := range generator.Handlers {
		if generator.Handlers[i].Name == "WebSocket" {
			wsHandler = &generator.Handlers[i]
			break
		}
	}

	assert.NotNil(t, wsHandler)
	assert.Len(t, wsHandler.Methods, 1)
	assert.Equal(t, "HandleConnection", wsHandler.Methods[0].Name)
	assert.Equal(t, "GET", wsHandler.Methods[0].HTTPMethod)
	assert.Equal(t, "/ws", wsHandler.Methods[0].Path)
}

func TestRouteCodeGenerator_GenerateCode(t *testing.T) {
	generator := NewRouteCodeGenerator("app")
	
	// Add test handlers
	generator.Handlers = []HandlerInfo{
		{
			Name:        "API",
			URLPattern:  "/api",
			IsWebSocket: false,
			TypeName:    "TestCodegenHandler",
			Methods: []MethodInfo{
				{Name: "GET", HTTPMethod: "GET", Path: "/api"},
				{Name: "POST", HTTPMethod: "POST", Path: "/api"},
				{Name: "CustomAction", HTTPMethod: "POST", Path: "/api/custom-action"},
			},
		},
		{
			Name:        "WebSocket",
			URLPattern:  "/ws",
			IsWebSocket: true,
			TypeName:    "TestCodegenWSHandler",
			Methods: []MethodInfo{
				{Name: "HandleConnection", HTTPMethod: "GET", Path: "/ws"},
			},
		},
	}

	code := generator.GenerateCode()

	// Check that code contains expected elements
	assert.Contains(t, code, "//go:build production")
	assert.Contains(t, code, "package app")
	assert.Contains(t, code, "func RegisterRoutesStatic")
	assert.Contains(t, code, "m.API.GET")
	assert.Contains(t, code, "m.API.POST")
	assert.Contains(t, code, "m.API.CustomAction")
	assert.Contains(t, code, "m.WebSocket.HandleConnection")

	// Check specific route registrations
	assert.Contains(t, code, `e.GET("/api", m.API.GET)`)
	assert.Contains(t, code, `e.POST("/api", m.API.POST)`)
	assert.Contains(t, code, `e.POST("/api/custom-action", m.API.CustomAction)`)
	assert.Contains(t, code, `e.GET("/ws", m.WebSocket.HandleConnection)`)

	// Should not contain reflection runtime calls
	assert.NotContains(t, code, "reflect.Value")
	assert.NotContains(t, code, "reflect.Call")
	assert.NotContains(t, code, ".Call(")
}

func TestExtractTag(t *testing.T) {
	tests := []struct {
		name     string
		tagString string
		key      string
		expected string
	}{
		{
			name:     "simple url tag",
			tagString: "`url:\"/api\"`",
			key:      "url",
			expected: "/api",
		},
		{
			name:     "multiple tags",
			tagString: "`url:\"/ws\" hijack:\"ws\"`",
			key:      "url",
			expected: "/ws",
		},
		{
			name:     "hijack tag",
			tagString: "`url:\"/ws\" hijack:\"ws\"`",
			key:      "hijack",
			expected: "ws",
		},
		{
			name:     "missing tag",
			tagString: "`url:\"/api\"`",
			key:      "hijack",
			expected: "",
		},
		{
			name:     "empty tag string",
			tagString: "",
			key:      "url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTag(tt.tagString, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRouteCodeGenerator_WriteToFile(t *testing.T) {
	generator := NewRouteCodeGenerator("app")
	
	// Add a simple handler for testing
	generator.Handlers = []HandlerInfo{
		{
			Name:        "Test",
			URLPattern:  "/test",
			IsWebSocket: false,
			TypeName:    "TestHandler",
			Methods: []MethodInfo{
				{Name: "GET", HTTPMethod: "GET", Path: "/test"},
			},
		},
	}

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "generated_routes_*.go")
	assert.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = generator.WriteToFile(tmpFile.Name())
	assert.NoError(t, err)

	// Read back and verify
	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "//go:build production")
	assert.Contains(t, contentStr, "func RegisterRoutesStatic")
	assert.Contains(t, contentStr, `e.GET("/test", m.Test.GET)`)
}

// Integration test that tests the full pipeline
func TestCodeGenerationFullPipeline(t *testing.T) {
	testFile := createTestFile(t)
	defer os.Remove(testFile)

	generator := NewRouteCodeGenerator("app")
	
	// Step 1: Analyze AST
	err := generator.AnalyzeHandlersFromAST(testFile)
	assert.NoError(t, err)

	// Step 2: Analyze methods for each unique handler type
	// We need to analyze each unique handler type separately
	handlerTypes := make(map[string]interface{})
	for _, handler := range generator.Handlers {
		if handler.TypeName == "TestCodegenHandler" {
			handlerTypes[handler.TypeName] = &TestCodegenHandler{}
		} else if handler.TypeName == "TestCodegenWSHandler" {
			handlerTypes[handler.TypeName] = &TestCodegenWSHandler{}
		}
	}
	
	for _, handlerInstance := range handlerTypes {
		err = generator.AnalyzeHandlerMethods(handlerInstance)
		assert.NoError(t, err)
	}

	// Step 3: Generate code
	code := generator.GenerateCode()
	
	// Verify the generated code has all expected routes
	expectedRoutes := []string{
		`e.GET("/api", m.API.GET)`,
		`e.POST("/api", m.API.POST)`,
		`e.POST("/api/custom-action", m.API.CustomAction)`,
		`e.GET("/users", m.Users.GET)`,
		`e.POST("/users", m.Users.POST)`,
		`e.POST("/users/custom-action", m.Users.CustomAction)`,
		`e.GET("/", m.Root.GET)`,
		`e.POST("/", m.Root.POST)`,
		`e.POST("/custom-action", m.Root.CustomAction)`,
		`e.GET("/ws", m.WebSocket.HandleConnection)`,
	}

	for _, route := range expectedRoutes {
		assert.Contains(t, code, route, "Missing route: %s", route)
	}

	// Check that it's properly formatted
	lines := strings.Split(code, "\n")
	assert.Greater(t, len(lines), 20, "Generated code seems too short")
	
	// Should have proper build tags
	assert.Equal(t, "//go:build production", lines[0])
	
	// Should import echo
	assert.Contains(t, code, `"github.com/labstack/echo/v4"`)
}