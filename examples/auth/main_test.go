package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/auth"
	"github.com/yshengliao/gortex/validation"
	"go.uber.org/zap"
)

// TestAuthExample tests the authentication example
func TestAuthExample(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create config
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Server.Recovery = true

	// Create JWT service
	jwtSvc := auth.NewJWTService(
		"test-secret-key",
		time.Hour,      // Access token TTL
		24*time.Hour*7, // Refresh token TTL
		"test-auth-example",
	)

	// Create handlers
	handlersManager := &HandlersManager{
		Auth: &AuthHandler{
			Logger:     logger,
			JWTService: jwtSvc,
		},
		Profile: &ProfileHandler{
			Logger: logger,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
	)
	require.NoError(t, err)

	// Get the Echo instance for testing
	e := application.Echo()

	// Set custom validator (like in main.go)
	validator := validation.NewValidator()
	e.Validator = validator

	// Register services in DI container (like in main.go)
	app.Register(application.Context(), logger)
	app.Register(application.Context(), jwtSvc)

	// Apply authentication middleware to profile routes
	profileGroup := e.Group("/profile")
	profileGroup.Use(auth.Middleware(jwtSvc))
	// Override the /profile route with middleware applied since RegisterRoutes
	// does not apply the group middleware automatically.
	e.GET("/profile", handlersManager.Profile.GET, auth.Middleware(jwtSvc))

	var accessToken string

	t.Run("Login Success", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "admin",
			Password: "password",
		}
		body, _ := json.Marshal(loginReq)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		accessToken = response["access_token"].(string)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, response["refresh_token"])
		assert.Equal(t, float64(3600), response["expires_in"])

		// Debug: Validate the token to ensure it's correct
		claims, err := jwtSvc.ValidateToken(accessToken)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.Subject)
		assert.Equal(t, "admin", claims.Username)
	})

	t.Run("Login Invalid Credentials", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "admin",
			Password: "wrongpassword",
		}
		body, _ := json.Marshal(loginReq)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Invalid credentials", response["error"])
	})

	t.Run("Login Validation Error", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "", // Empty username
			Password: "password",
		}
		body, _ := json.Marshal(loginReq)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Profile with Valid Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Debug: Print the response to see what we're getting
		t.Logf("Profile response: %+v", response)

		// If we're getting an error response instead of profile data
		if _, ok := response["error"]; ok {
			t.Fatalf("Got error response: %v", response)
		}

		assert.Equal(t, "user-123", response["user_id"])
		assert.Equal(t, "admin", response["username"])
		assert.Equal(t, "admin@example.com", response["email"])
		assert.Equal(t, "admin", response["role"])
	})

	t.Run("Profile without Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Profile with Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Refresh Token", func(t *testing.T) {
		// First login to get refresh token
		loginReq := LoginRequest{
			Username: "admin",
			Password: "password",
		}
		body, _ := json.Marshal(loginReq)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		var loginResponse map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &loginResponse)
		refreshToken := loginResponse["refresh_token"].(string)

		// Now test refresh
		refreshReq := struct {
			RefreshToken string `json:"refresh_token"`
		}{
			RefreshToken: refreshToken,
		}
		body, _ = json.Marshal(refreshReq)

		req = httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response["access_token"])
		assert.Equal(t, float64(3600), response["expires_in"])
	})
}

// TestJWTTokenValidation tests JWT token generation and validation
func TestJWTTokenValidation(t *testing.T) {
	jwtSvc := auth.NewJWTService(
		"test-secret",
		time.Hour,
		24*time.Hour*7,
		"test-issuer",
	)

	t.Run("Generate and Validate Access Token", func(t *testing.T) {
		token, err := jwtSvc.GenerateAccessToken("123", "testuser", "test@example.com", "user")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate token
		validatedClaims, err := jwtSvc.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "123", validatedClaims.Subject)
		assert.Equal(t, "testuser", validatedClaims.Username)
		assert.Equal(t, "user", validatedClaims.Role)
	})

	t.Run("Invalid Token", func(t *testing.T) {
		_, err := jwtSvc.ValidateToken("invalid.token.here")
		assert.Error(t, err)
	})
}

// BenchmarkAuth benchmarks authentication operations
func BenchmarkAuth(b *testing.B) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	jwtSvc := auth.NewJWTService(
		"benchmark-secret",
		time.Hour,
		24*time.Hour*7,
		"benchmark-issuer",
	)

	b.Run("GenerateToken", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			jwtSvc.GenerateAccessToken("123", "benchuser", "bench@example.com", "user")
		}
	})

	b.Run("ValidateToken", func(b *testing.B) {
		token, _ := jwtSvc.GenerateAccessToken("123", "benchuser", "bench@example.com", "user")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			jwtSvc.ValidateToken(token)
		}
	})
}
