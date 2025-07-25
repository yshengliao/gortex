package abtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock handlers for testing
type UserHandler struct{}

func (h *UserHandler) GetUser(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"id":   id,
		"name": "John Doe",
		"email": "john@example.com",
	})
}

func (h *UserHandler) CreateUser(c echo.Context) error {
	var user map[string]interface{}
	if err := c.Bind(&user); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	
	user["id"] = "123"
	return c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(c echo.Context) error {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	
	updates["id"] = id
	updates["updated"] = true
	return c.JSON(http.StatusOK, updates)
}

// Setup functions for Echo and Gortex apps
func setupEchoApp() *echo.Echo {
	e := echo.New()
	
	// Add custom middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Server", "Echo")
			return next(c)
		}
	})
	
	// Register routes
	handler := &UserHandler{}
	e.GET("/users/:id", handler.GetUser)
	e.POST("/users", handler.CreateUser)
	e.PUT("/users/:id", handler.UpdateUser)
	
	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"server": "echo",
		})
	})
	
	return e
}

func setupGortexApp() *echo.Echo {
	// For now, Gortex uses Echo under the hood
	// In future, this would be a pure Gortex app
	e := echo.New()
	
	// Add custom middleware (slightly different to test comparison)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Server", "Gortex")
			c.Response().Header().Set("X-Version", "1.0.0")
			return next(c)
		}
	})
	
	// Register routes (same as Echo)
	handler := &UserHandler{}
	e.GET("/users/:id", handler.GetUser)
	e.POST("/users", handler.CreateUser)
	e.PUT("/users/:id", handler.UpdateUser)
	
	// Health check with slightly different response
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"server": "gortex",
			"version": "1.0.0",
		})
	})
	
	return e
}

func TestIntegrationABTesting(t *testing.T) {
	// Setup both apps
	echoApp := setupEchoApp()
	gortexApp := setupGortexApp()
	
	// Create dual executor
	executor := NewDualExecutor(echoApp, gortexApp)
	
	t.Run("GET request comparison", func(t *testing.T) {
		result, err := executor.ExecuteRequest(http.MethodGet, "/users/123", nil)
		require.NoError(t, err)
		
		// Both should return 200 OK
		assert.Equal(t, http.StatusOK, result.EchoResponse.StatusCode)
		assert.Equal(t, http.StatusOK, result.GortexResponse.StatusCode)
		
		// Bodies should be identical (same handler logic)
		var echoBody, gortexBody map[string]interface{}
		err = json.Unmarshal(result.EchoResponse.Body, &echoBody)
		require.NoError(t, err)
		err = json.Unmarshal(result.GortexResponse.Body, &gortexBody)
		require.NoError(t, err)
		assert.Equal(t, echoBody, gortexBody)
		
		// Headers will differ due to middleware
		assert.False(t, result.IsIdentical)
		
		// Check specific header differences
		var serverHeaderDiff *Difference
		for i := range result.Differences {
			if result.Differences[i].Field == "x-server" {
				serverHeaderDiff = &result.Differences[i]
				break
			}
		}
		assert.NotNil(t, serverHeaderDiff)
		assert.Equal(t, []string{"Echo"}, serverHeaderDiff.EchoValue)
		assert.Equal(t, []string{"Gortex"}, serverHeaderDiff.GortexValue)
	})
	
	t.Run("POST request comparison", func(t *testing.T) {
		reqBody := map[string]string{
			"name": "Jane Doe",
			"email": "jane@example.com",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		
		result, err := executor.ExecuteRequest(http.MethodPost, "/users", 
			bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		
		// Both should return 201 Created
		assert.Equal(t, http.StatusCreated, result.EchoResponse.StatusCode)
		assert.Equal(t, http.StatusCreated, result.GortexResponse.StatusCode)
		
		// Verify request was captured
		assert.Equal(t, bodyBytes, result.RequestBody)
	})
	
	t.Run("health check endpoint differences", func(t *testing.T) {
		result, err := executor.ExecuteRequest(http.MethodGet, "/health", nil)
		require.NoError(t, err)
		
		// Status codes should match
		assert.Equal(t, http.StatusOK, result.EchoResponse.StatusCode)
		assert.Equal(t, http.StatusOK, result.GortexResponse.StatusCode)
		
		// But bodies will differ
		assert.False(t, result.IsIdentical)
		
		// Check body differences
		var echoHealth, gortexHealth map[string]string
		err = json.Unmarshal(result.EchoResponse.Body, &echoHealth)
		require.NoError(t, err)
		err = json.Unmarshal(result.GortexResponse.Body, &gortexHealth)
		require.NoError(t, err)
		
		assert.Equal(t, "echo", echoHealth["server"])
		assert.Equal(t, "gortex", gortexHealth["server"])
		assert.Contains(t, gortexHealth, "version") // Gortex has extra field
	})
	
	t.Run("batch testing with recorder", func(t *testing.T) {
		// Create a fresh executor for this test
		batchExecutor := NewDualExecutor(echoApp, gortexApp)
		
		// Test multiple endpoints
		testCases := []struct {
			method string
			path   string
			body   interface{}
		}{
			{http.MethodGet, "/users/1", nil},
			{http.MethodGet, "/users/2", nil},
			{http.MethodPost, "/users", map[string]string{"name": "Test User"}},
			{http.MethodPut, "/users/1", map[string]string{"name": "Updated User"}},
			{http.MethodGet, "/health", nil},
		}
		
		for _, tc := range testCases {
			var body io.Reader
			if tc.body != nil {
				bodyBytes, _ := json.Marshal(tc.body)
				body = bytes.NewReader(bodyBytes)
			}
			
			_, err := batchExecutor.ExecuteRequest(tc.method, tc.path, body)
			require.NoError(t, err)
		}
		
		// Get summary
		recorder := batchExecutor.GetRecorder()
		summary := recorder.GetSummary()
		
		// All requests should have been recorded
		assert.Equal(t, len(testCases), summary.TotalRequests)
		
		// None should be identical due to header differences
		assert.Equal(t, 0, summary.IdenticalCount)
		assert.Equal(t, len(testCases), summary.DifferentCount)
		
		// All differences should be header-related
		assert.Greater(t, summary.DifferenceTypes["header"], 0)
		
		// Print summary for debugging
		t.Log(summary.String())
		
		// Check failures
		failures := recorder.GetFailures()
		assert.Len(t, failures, len(testCases))
	})
}

func TestABTestingErrorHandling(t *testing.T) {
	echoApp := echo.New()
	gortexApp := echo.New()
	
	// Echo returns 404
	echoApp.GET("/test", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "Not found")
	})
	
	// Gortex returns 500
	gortexApp.GET("/test", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal error")
	})
	
	executor := NewDualExecutor(echoApp, gortexApp)
	
	result, err := executor.ExecuteRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	
	// Should detect status code difference
	assert.False(t, result.IsIdentical)
	assert.Equal(t, http.StatusNotFound, result.EchoResponse.StatusCode)
	assert.Equal(t, http.StatusInternalServerError, result.GortexResponse.StatusCode)
	
	// Check for status difference
	var statusDiff *Difference
	for i := range result.Differences {
		if result.Differences[i].Type == "status" {
			statusDiff = &result.Differences[i]
			break
		}
	}
	assert.NotNil(t, statusDiff)
	assert.Contains(t, statusDiff.Description, "Status code mismatch")
}