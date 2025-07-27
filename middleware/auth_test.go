package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yshengliao/gortex/pkg/auth"
	"github.com/yshengliao/gortex/transport/http"
)

func TestJWTAuth(t *testing.T) {
	// Create a JWT service for testing
	jwtService := auth.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour, "test-issuer")

	// Generate a valid token
	validToken, err := jwtService.GenerateAccessToken("user123", "testuser", "test@example.com", "user")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	middleware := JWTAuth(jwtService)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectClaims   bool
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: 200,
			expectClaims:   true,
		},
		{
			name:           "missing auth header",
			authHeader:     "",
			expectedStatus: 401,
			expectClaims:   false,
		},
		{
			name:           "invalid format",
			authHeader:     "InvalidFormat " + validToken,
			expectedStatus: 401,
			expectClaims:   false,
		},
		{
			name:           "missing bearer prefix",
			authHeader:     validToken,
			expectedStatus: 401,
			expectClaims:   false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: 401,
			expectClaims:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			var capturedClaims *auth.Claims
			handler := middleware(func(c Context) error {
				capturedClaims = GetClaims(c)
				if tt.expectClaims && capturedClaims == nil {
					t.Error("Expected claims to be set")
				}
				if !tt.expectClaims && capturedClaims != nil {
					t.Error("Expected no claims")
				}
				return c.String(200, "OK")
			})

			err := handler(ctx)
			
			if tt.expectedStatus == 401 {
				if err == nil {
					t.Error("Expected error for unauthorized request")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectClaims && capturedClaims != nil {
				if capturedClaims.UserID != "user123" {
					t.Errorf("Expected UserID user123, got %s", capturedClaims.UserID)
				}
				if capturedClaims.Username != "testuser" {
					t.Errorf("Expected Username testuser, got %s", capturedClaims.Username)
				}
			}
		})
	}
}

func TestJWTAuthSkipPaths(t *testing.T) {
	jwtService := auth.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour, "test-issuer")
	
	config := &AuthConfig{
		JWTService: jwtService,
		SkipPaths:  []string{"/public", "/health"},
	}
	middleware := JWTAuthWithConfig(config)

	tests := []struct {
		name      string
		path      string
		needsAuth bool
	}{
		{
			name:      "public path",
			path:      "/public/resource",
			needsAuth: false,
		},
		{
			name:      "health path",
			path:      "/health",
			needsAuth: false,
		},
		{
			name:      "protected path",
			path:      "/api/users",
			needsAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			// Don't set auth header
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			var called bool
			handler := middleware(func(c Context) error {
				called = true
				return c.String(200, "OK")
			})

			err := handler(ctx)

			if tt.needsAuth {
				if err == nil {
					t.Error("Expected authentication error")
				}
				if called {
					t.Error("Handler should not be called without auth")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for skipped path: %v", err)
				}
				if !called {
					t.Error("Handler should be called for skipped path")
				}
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	jwtService := auth.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour, "test-issuer")
	
	// Generate tokens with different roles
	adminToken, _ := jwtService.GenerateAccessToken("admin123", "admin", "admin@example.com", "admin")
	userToken, _ := jwtService.GenerateAccessToken("user123", "user", "user@example.com", "user")

	authMiddleware := JWTAuth(jwtService)
	adminMiddleware := RequireRole("admin")

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "admin access allowed",
			token:          adminToken,
			expectedStatus: 200,
		},
		{
			name:           "user access denied",
			token:          userToken,
			expectedStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			// Chain auth -> requireRole -> handler
			handler := authMiddleware(adminMiddleware(func(c Context) error {
				return c.String(200, "Admin area")
			}))

			err := handler(ctx)

			if tt.expectedStatus == 403 {
				if err == nil {
					t.Error("Expected forbidden error")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRequireGameID(t *testing.T) {
	jwtService := auth.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour, "test-issuer")
	
	// Generate tokens with and without game ID
	gameToken, _ := jwtService.GenerateGameToken("user123", "player1", "game456")
	normalToken, _ := jwtService.GenerateAccessToken("user123", "player1", "player@example.com", "user")

	authMiddleware := JWTAuth(jwtService)
	gameMiddleware := RequireGameID()

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "game token allowed",
			token:          gameToken,
			expectedStatus: 200,
		},
		{
			name:           "normal token denied",
			token:          normalToken,
			expectedStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/game/action", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			// Chain auth -> requireGameID -> handler
			handler := authMiddleware(gameMiddleware(func(c Context) error {
				return c.String(200, "Game action")
			}))

			err := handler(ctx)

			if tt.expectedStatus == 403 {
				if err == nil {
					t.Error("Expected forbidden error")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Mock session store for testing
type mockSessionStore struct {
	sessions map[string]map[string]interface{}
	valid    map[string]bool
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]map[string]interface{}),
		valid:    make(map[string]bool),
	}
}

func (s *mockSessionStore) Get(sessionID string) (map[string]interface{}, error) {
	if data, ok := s.sessions[sessionID]; ok {
		return data, nil
	}
	return nil, errors.New("session not found")
}

func (s *mockSessionStore) Validate(sessionID string) (bool, error) {
	if valid, ok := s.valid[sessionID]; ok {
		return valid, nil
	}
	return false, nil
}

func TestSessionAuth(t *testing.T) {
	store := newMockSessionStore()
	
	// Add test sessions
	validSessionID := "valid-session-123"
	store.sessions[validSessionID] = map[string]interface{}{
		"user_id":  "user123",
		"username": "testuser",
	}
	store.valid[validSessionID] = true

	invalidSessionID := "invalid-session-456"
	store.valid[invalidSessionID] = false

	middleware := SessionAuth(store)

	tests := []struct {
		name           string
		sessionID      string
		inCookie       bool
		expectedStatus int
		expectSession  bool
	}{
		{
			name:           "valid session in cookie",
			sessionID:      validSessionID,
			inCookie:       true,
			expectedStatus: 200,
			expectSession:  true,
		},
		{
			name:           "valid session in header",
			sessionID:      validSessionID,
			inCookie:       false,
			expectedStatus: 200,
			expectSession:  true,
		},
		{
			name:           "invalid session",
			sessionID:      invalidSessionID,
			inCookie:       true,
			expectedStatus: 401,
			expectSession:  false,
		},
		{
			name:           "missing session",
			sessionID:      "",
			inCookie:       false,
			expectedStatus: 401,
			expectSession:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			
			if tt.sessionID != "" {
				if tt.inCookie {
					req.AddCookie(&http.Cookie{
						Name:  "session_id",
						Value: tt.sessionID,
					})
				} else {
					req.Header.Set("session_id", tt.sessionID)
				}
			}
			
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			var capturedSession map[string]interface{}
			handler := middleware(func(c Context) error {
				if session, ok := c.Get("session").(map[string]interface{}); ok {
					capturedSession = session
				}
				return c.String(200, "OK")
			})

			err := handler(ctx)

			if tt.expectedStatus == 401 {
				if err == nil {
					t.Error("Expected error for unauthorized request")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectSession {
				if capturedSession == nil {
					t.Error("Expected session data to be set")
				} else {
					if capturedSession["user_id"] != "user123" {
						t.Errorf("Expected user_id user123, got %v", capturedSession["user_id"])
					}
				}
			}
		})
	}
}

func TestGetClaimsHelpers(t *testing.T) {
	// Test GetUserID and GetUsername helpers
	ctx := newMockContext(httptest.NewRequest("GET", "/", nil), httptest.NewRecorder())
	
	claims := &auth.Claims{
		UserID:   "user123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
		GameID:   "game456",
	}
	
	ctx.Set("jwt-claims", claims)
	
	if userID := GetUserID(ctx); userID != "user123" {
		t.Errorf("Expected UserID user123, got %s", userID)
	}
	
	if username := GetUsername(ctx); username != "testuser" {
		t.Errorf("Expected Username testuser, got %s", username)
	}
	
	// Test with no claims
	ctx.Set("jwt-claims", nil)
	
	if userID := GetUserID(ctx); userID != "" {
		t.Errorf("Expected empty UserID, got %s", userID)
	}
	
	if username := GetUsername(ctx); username != "" {
		t.Errorf("Expected empty Username, got %s", username)
	}
}

func TestGetClaimsFromContext(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "user123",
		Username: "testuser",
	}
	
	ctx := context.WithValue(context.Background(), "jwt-claims", claims)
	
	retrieved := GetClaimsFromContext(ctx)
	if retrieved == nil {
		t.Error("Expected to retrieve claims from context")
	} else if retrieved.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", retrieved.UserID)
	}
	
	// Test with no claims
	emptyCtx := context.Background()
	if GetClaimsFromContext(emptyCtx) != nil {
		t.Error("Expected nil claims from empty context")
	}
}