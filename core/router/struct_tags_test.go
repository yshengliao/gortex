package router_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/middleware"
	"go.uber.org/zap"
)

// Test service for dependency injection
type TestService struct {
	Name string
}

func (s *TestService) GetName() string {
	return s.Name
}

// Test handlers with struct tags
type TaggedHandlers struct {
	// Basic handler with inject tag
	Basic *BasicHandler `url:"/basic"`

	// Handler with middleware tags
	Protected *ProtectedHandler `url:"/protected" middleware:"auth,requestid"`

	// Handler with rate limiting
	Limited *LimitedHandler `url:"/limited" ratelimit:"10/min"`

	// Handler with combined tags
	Advanced *AdvancedHandler `url:"/advanced" middleware:"requestid" ratelimit:"100/hour"`

	// Nested group with middleware
	Admin *AdminGroup `url:"/admin" middleware:"auth"`
}

type BasicHandler struct {
	// Test dependency injection
	Service *TestService `inject:""`
}

func (h *BasicHandler) GET(c context.Context) error {
	if h.Service != nil {
		return c.String(http.StatusOK, fmt.Sprintf("service injected: %s (ptr: %p)", h.Service.GetName(), h.Service))
	}
	return c.String(http.StatusOK, "no service")
}

type ProtectedHandler struct{}

func (h *ProtectedHandler) GET(c context.Context) error {
	// Check if request ID was set by middleware
	requestID := c.Request().Header.Get("X-Request-ID")
	if requestID != "" {
		return c.String(http.StatusOK, "protected with request ID: "+requestID)
	}
	return c.String(http.StatusOK, "protected")
}

type LimitedHandler struct{}

func (h *LimitedHandler) GET(c context.Context) error {
	return c.String(http.StatusOK, "limited")
}

type AdvancedHandler struct {
	Service *TestService `inject:""`
}

func (h *AdvancedHandler) GET(c context.Context) error {
	return c.String(http.StatusOK, "advanced")
}

type AdminGroup struct {
	Dashboard *DashboardHandler `url:"/dashboard"`
}

type DashboardHandler struct{}

func (h *DashboardHandler) GET(c context.Context) error {
	return c.String(http.StatusOK, "admin dashboard")
}

func TestStructTagsBasic(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create app context with test service
	ctx := app.NewContext()
	testService := &TestService{Name: "test-service"}
	// Register with the proper type
	app.Register(ctx, testService)
	app.Register(ctx, logger)

	// Debug: log the service
	t.Logf("Registered service: %s (ptr: %p)", testService.Name, testService)

	// Create app
	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	require.NoError(t, err)

	// Register routes with context for DI
	handlers := &TaggedHandlers{}
	err = app.RegisterRoutesFromStruct(application.Router(), handlers, ctx)
	require.NoError(t, err)

	// Test basic handler with injection
	req := httptest.NewRequest(http.MethodGet, "/basic", nil)
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	// Debug: print what we got
	t.Logf("Response status: %d", rec.Code)
	t.Logf("Response body: %s", rec.Body.String())
	assert.Equal(t, http.StatusOK, rec.Code)
	if rec.Code == http.StatusOK {
		assert.Contains(t, rec.Body.String(), "service injected:")
	}
}

func TestStructTagsMiddleware(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create app context
	ctx := app.NewContext()
	app.Register(ctx, logger)

	// Register middleware in context
	middlewareRegistry := make(map[string]middleware.MiddlewareFunc)
	middlewareRegistry["auth"] = func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			// Simple auth check
			if c.Request().Header.Get("Authorization") == "" {
				return c.String(http.StatusUnauthorized, "unauthorized")
			}
			return next(c)
		}
	}
	app.Register(ctx, middlewareRegistry)

	// Create app
	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	require.NoError(t, err)

	// Register routes
	handlers := &TaggedHandlers{}
	err = app.RegisterRoutesFromStruct(application.Router(), handlers, ctx)
	require.NoError(t, err)

	t.Run("Protected route without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "unauthorized", rec.Body.String())
	})

	t.Run("Protected route with auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Request ID middleware should have added a header
		assert.Contains(t, rec.Body.String(), "protected")
	})
}

func TestStructTagsRateLimit(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create app context
	ctx := app.NewContext()
	app.Register(ctx, logger)

	// Create app
	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	require.NoError(t, err)

	// Register routes
	handlers := &TaggedHandlers{}
	err = app.RegisterRoutesFromStruct(application.Router(), handlers, ctx)
	require.NoError(t, err)

	// Test rate limited route - should work for local requests (rate limit is skipped)
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "limited", rec.Body.String())
}

func TestStructTagsNestedGroup(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create app context
	ctx := app.NewContext()
	app.Register(ctx, logger)

	// Register auth middleware
	middlewareRegistry := make(map[string]middleware.MiddlewareFunc)
	middlewareRegistry["auth"] = func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			if c.Request().Header.Get("Authorization") == "" {
				return c.String(http.StatusUnauthorized, "unauthorized")
			}
			return next(c)
		}
	}
	app.Register(ctx, middlewareRegistry)

	// Create app
	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	require.NoError(t, err)

	// Register routes
	handlers := &TaggedHandlers{}
	err = app.RegisterRoutesFromStruct(application.Router(), handlers, ctx)
	require.NoError(t, err)

	t.Run("Admin route without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Admin route with auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "admin dashboard", rec.Body.String())
	})
}
