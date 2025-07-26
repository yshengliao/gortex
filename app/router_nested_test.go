package app

import (
	gortexMiddleware "github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/router"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Test structs for nested routing
type NestedUserHandler struct{}

func (h *NestedUserHandler) GET(c context.Context) error {
	return c.String(http.StatusOK, "nested user")
}

func (h *NestedUserHandler) Profile(c context.Context) error {
	return c.String(http.StatusOK, "nested user profile")
}

type V1Group struct {
	Users *NestedUserHandler `url:"/users"`
}

func (g *V1Group) GET(c context.Context) error {
	return c.String(http.StatusOK, "v1 root")
}

type APIGroup struct {
	V1 *V1Group `url:"/v1"`
}

type NestedGroupHandlersManager struct {
	API *APIGroup `url:"/api"`
}

func TestNestedRouting(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewDevelopment()
	ctx := NewContext()
	Register(ctx, logger)

	manager := &NestedGroupHandlersManager{
		API: &APIGroup{
			V1: &V1Group{
				Users: &NestedUserHandler{},
			},
		},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, manager)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		path         string
		expectedBody string
	}{
		{
			name:         "Deeply nested handler method",
			path:         "/api/v1/users",
			expectedBody: "nested user",
		},
		{
			name:         "Deeply nested custom method",
			path:         "/api/v1/users/profile",
			expectedBody: "nested user profile",
		},
		{
			name:         "Intermediate group handler method",
			path:         "/api/v1",
			expectedBody: "v1 root",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Special handling for POST routes
			method := http.MethodGet
			if tc.path == "/api/v1/users/profile" {
				method = http.MethodPost // Custom methods are registered as POST
			}

			req := httptest.NewRequest(method, tc.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tc.expectedBody, rec.Body.String())
		})
	}
}

// Test middleware inheritance
type NestedMiddlewareHandler struct{}

func (h *NestedMiddlewareHandler) GET(c context.Context) error {
	// Add to middleware chain tracking
	if chain, ok := c.Get("middlewareChain").([]string); ok {
		chain = append(chain, "handler")
		c.Set("middlewareChain", chain)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"middlewareChain": c.Get("middlewareChain"),
	})
}

type V1MiddlewareGroup struct {
	Users *NestedMiddlewareHandler `url:"/users"`
}

type APIMiddlewareGroup struct {
	V1 *V1MiddlewareGroup `url:"/v1" middleware:"logger"`
}

type NestedMiddlewareHandlersManager struct {
	API *APIMiddlewareGroup `url:"/api" middleware:"recover"`
}

func TestNestedMiddlewareInheritance(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewDevelopment()
	ctx := NewContext()
	Register(ctx, logger)

	// Register test middleware
	testRecoverMiddleware := func(next gortexMiddleware.HandlerFunc) gortexMiddleware.HandlerFunc {
		return func(c context.Context) error {
			chain := []string{"recover"}
			c.Set("middlewareChain", chain)
			return next(c)
		}
	}

	testLoggerMiddleware := func(next gortexMiddleware.HandlerFunc) gortexMiddleware.HandlerFunc {
		return func(c context.Context) error {
			if chain, ok := c.Get("middlewareChain").([]string); ok {
				chain = append(chain, "logger")
				c.Set("middlewareChain", chain)
			}
			return next(c)
		}
	}

	// Create a middleware registry to store named middleware
	middlewareRegistry := make(map[string]gortexMiddleware.MiddlewareFunc)
	middlewareRegistry["recover"] = testRecoverMiddleware
	middlewareRegistry["logger"] = testLoggerMiddleware

	// Register middleware registry in context
	Register(ctx, middlewareRegistry)

	manager := &NestedMiddlewareHandlersManager{
		API: &APIMiddlewareGroup{
			V1: &V1MiddlewareGroup{
				Users: &NestedMiddlewareHandler{},
			},
		},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, manager)
	require.NoError(t, err)

	// Test that middleware is inherited correctly
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify middleware chain order
	chain, ok := response["middlewareChain"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"recover", "logger", "handler"}, chain)
}
