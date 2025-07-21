//go:build production
// +build production

package app

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// This test runs only in production mode to verify static routing
func TestProductionModeRouting(t *testing.T) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	// Define handlers for testing (replicating the generated types)
	handlers := &HandlersManager{
		API:   &TestCodegenHandler{Logger: logger},
		Users: &TestCodegenHandler{Logger: logger},
		Root:  &TestCodegenHandler{Logger: logger},
	}

	err := RegisterRoutes(e, handlers, ctx)
	assert.NoError(t, err)

	// Test a few key routes to ensure static registration works
	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"method":"GET"`)

	// Test custom endpoint
	req = httptest.NewRequest("POST", "/api/custom-action", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"method":"CustomAction"`)
}