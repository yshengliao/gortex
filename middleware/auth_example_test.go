package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/yshengliao/gortex/pkg/auth"
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// mockContext for examples
type mockContext struct {
	values   map[string]interface{}
	request  *http.Request
	response http.ResponseWriter
}

func (m *mockContext) Param(name string) string                      { return "" }
func (m *mockContext) QueryParam(name string) string                 { return "" }
func (m *mockContext) Bind(i interface{}) error                      { return nil }
func (m *mockContext) JSON(code int, i interface{}) error            { return nil }
func (m *mockContext) String(code int, s string) error               { return nil }
func (m *mockContext) Get(key string) interface{}                    { return m.values[key] }
func (m *mockContext) Set(key string, val interface{})               { m.values[key] = val }
func (m *mockContext) Request() interface{}                          { return m.request }
func (m *mockContext) Response() interface{}                         { return m.response }

// Example of using JWT authentication middleware
func ExampleJWTAuth() {
	// Create JWT service
	jwtService := auth.NewJWTService("secret-key", 1*time.Hour, 24*time.Hour, "my-app")

	// Create middleware
	authMiddleware := middleware.JWTAuth(jwtService)

	// Create a protected handler
	protectedHandler := func(c Context) error {
		claims := middleware.GetClaims(c)
		if claims == nil {
			return fmt.Errorf("no claims found")
		}
		fmt.Printf("User ID: %s\n", claims.UserID)
		return nil
	}

	// Wrap with auth middleware
	handler := authMiddleware(protectedHandler)

	// Generate a test token
	token, _ := jwtService.GenerateAccessToken("123", "john", "john@example.com", "user")

	// Create test request with auth header
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	
	// Create context and execute
	ctx := &mockContext{
		values:   make(map[string]interface{}),
		request:  req,
		response: rec,
	}
	
	// Execute handler
	err := handler(ctx)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Output: User ID: 123
}

// Example of role-based access control
func ExampleRequireRole() {
	// Create JWT service
	jwtService := auth.NewJWTService("secret-key", 1*time.Hour, 24*time.Hour, "my-app")

	// Create middlewares
	authMiddleware := middleware.JWTAuth(jwtService)
	adminMiddleware := middleware.RequireRole("admin")

	// Create an admin-only handler
	adminHandler := func(c Context) error {
		fmt.Println("Admin access granted")
		return nil
	}

	// Chain middlewares
	handler := authMiddleware(adminMiddleware(adminHandler))

	// Generate admin token
	adminToken, _ := jwtService.GenerateAccessToken("1", "admin", "admin@example.com", "admin")

	// Create test request
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	
	// Create context and execute
	ctx := &mockContext{
		values:   make(map[string]interface{}),
		request:  req,
		response: rec,
	}
	
	// Execute handler
	handler(ctx)

	// Output: Admin access granted
}

// Example of getting claims from context
func ExampleGetClaims() {
	// Create context with claims
	ctx := &mockContext{
		values: map[string]interface{}{
			"jwt-claims": &auth.Claims{
				UserID:   "123",
				Username: "john",
				Email:    "john@example.com",
				Role:     "user",
			},
		},
	}

	// Get claims
	claims := middleware.GetClaims(ctx)
	if claims != nil {
		fmt.Printf("UserID: %s, Username: %s\n", claims.UserID, claims.Username)
	}

	// Output: UserID: 123, Username: john
}

// Example of session authentication
func ExampleSessionAuth() {
	// Create a mock session store
	store := &MockSessionStore{
		sessions: map[string]map[string]interface{}{
			"valid-session": {
				"user_id":  "123",
				"username": "john",
			},
		},
	}

	// Create session middleware
	sessionMiddleware := middleware.SessionAuth(store)

	// Create a protected handler
	handler := sessionMiddleware(func(c Context) error {
		session := c.Get("session").(map[string]interface{})
		fmt.Printf("Session user: %s\n", session["username"])
		return nil
	})

	// Create test request with session cookie
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: "valid-session",
	})
	rec := httptest.NewRecorder()
	
	// Create context and execute
	ctx := &mockContext{
		values:   make(map[string]interface{}),
		request:  req,
		response: rec,
	}
	
	// Execute handler
	handler(ctx)

	// Output: Session user: john
}

// MockSessionStore for examples
type MockSessionStore struct {
	sessions map[string]map[string]interface{}
}

func (s *MockSessionStore) Get(sessionID string) (map[string]interface{}, error) {
	if data, ok := s.sessions[sessionID]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (s *MockSessionStore) Validate(sessionID string) (bool, error) {
	_, exists := s.sessions[sessionID]
	return exists, nil
}