package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/auth"
)

func TestMiddleware(t *testing.T) {
	e := echo.New()
	jwtService := auth.NewJWTService("test-secret", time.Hour, 24*time.Hour, "test")

	// Generate a valid token
	validToken, err := jwtService.GenerateAccessToken("user123", "testuser", "test@example.com", "player")
	require.NoError(t, err)

	// Test handler that requires authentication
	handler := func(c echo.Context) error {
		claims := auth.GetClaims(c)
		if claims == nil {
			return c.JSON(500, map[string]string{"error": "no claims"})
		}
		return c.JSON(200, map[string]string{
			"user_id":  claims.UserID,
			"username": claims.Username,
		})
	}

	// Apply middleware
	e.GET("/protected", handler, auth.Middleware(jwtService))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: 200,
			expectedBody:   `"user_id":"user123"`,
		},
		{
			name:           "Missing header",
			authHeader:     "",
			expectedStatus: 401,
			expectedBody:   "missing authorization header",
		},
		{
			name:           "Invalid format",
			authHeader:     "InvalidFormat",
			expectedStatus: 401,
			expectedBody:   "invalid authorization header format",
		},
		{
			name:           "Invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: 401,
			expectedBody:   "invalid or expired token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.expectedBody)
		})
	}
}

func TestGetClaims(t *testing.T) {
	e := echo.New()
	c := e.NewContext(nil, nil)

	// No claims set
	claims := auth.GetClaims(c)
	assert.Nil(t, claims)

	// Set claims
	expectedClaims := &auth.Claims{
		UserID:   "user123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "player",
	}
	c.Set(auth.ClaimsContextKey, expectedClaims)

	// Get claims
	claims = auth.GetClaims(c)
	assert.NotNil(t, claims)
	assert.Equal(t, expectedClaims.UserID, claims.UserID)
	assert.Equal(t, expectedClaims.Username, claims.Username)
}

func TestGetUserID(t *testing.T) {
	e := echo.New()
	c := e.NewContext(nil, nil)

	// No claims
	userID := auth.GetUserID(c)
	assert.Empty(t, userID)

	// With claims
	c.Set(auth.ClaimsContextKey, &auth.Claims{UserID: "user123"})
	userID = auth.GetUserID(c)
	assert.Equal(t, "user123", userID)
}

func TestGetUsername(t *testing.T) {
	e := echo.New()
	c := e.NewContext(nil, nil)

	// No claims
	username := auth.GetUsername(c)
	assert.Empty(t, username)

	// With claims
	c.Set(auth.ClaimsContextKey, &auth.Claims{Username: "testuser"})
	username = auth.GetUsername(c)
	assert.Equal(t, "testuser", username)
}

func TestRequireRole(t *testing.T) {
	e := echo.New()
	
	handler := func(c echo.Context) error {
		return c.JSON(200, map[string]string{"message": "success"})
	}

	// Apply middleware
	e.GET("/admin", handler, auth.RequireRole("admin"))

	tests := []struct {
		name           string
		claims         *auth.Claims
		expectedStatus int
	}{
		{
			name:           "No claims",
			claims:         nil,
			expectedStatus: 401,
		},
		{
			name:           "Wrong role",
			claims:         &auth.Claims{UserID: "user123", Role: "player"},
			expectedStatus: 403,
		},
		{
			name:           "Correct role",
			claims:         &auth.Claims{UserID: "user123", Role: "admin"},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			if tt.claims != nil {
				c.Set(auth.ClaimsContextKey, tt.claims)
			}
			
			// Execute middleware and handler
			h := auth.RequireRole("admin")(handler)
			err := h(c)
			
			if tt.expectedStatus >= 400 {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, tt.expectedStatus, httpErr.Code)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestRequireGameID(t *testing.T) {
	e := echo.New()
	
	handler := func(c echo.Context) error {
		return c.JSON(200, map[string]string{"message": "success"})
	}

	// Apply middleware
	e.GET("/game", handler, auth.RequireGameID())

	tests := []struct {
		name           string
		claims         *auth.Claims
		expectedStatus int
	}{
		{
			name:           "No claims",
			claims:         nil,
			expectedStatus: 401,
		},
		{
			name:           "No game ID",
			claims:         &auth.Claims{UserID: "user123"},
			expectedStatus: 403,
		},
		{
			name:           "With game ID",
			claims:         &auth.Claims{UserID: "user123", GameID: "game001"},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/game", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			if tt.claims != nil {
				c.Set(auth.ClaimsContextKey, tt.claims)
			}
			
			// Execute middleware and handler
			h := auth.RequireGameID()(handler)
			err := h(c)
			
			if tt.expectedStatus >= 400 {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, tt.expectedStatus, httpErr.Code)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, rec.Code)
			}
		})
	}
}