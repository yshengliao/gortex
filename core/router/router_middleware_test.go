package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	gortexMiddleware "github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// Test middleware that adds a header
func testMiddleware(name, value string) gortexMiddleware.MiddlewareFunc {
	return func(next gortexMiddleware.HandlerFunc) gortexMiddleware.HandlerFunc {
		return func(c httpctx.Context) error {
			c.Response().Header().Set(name, value)
			return next(c)
		}
	}
}

// Test middleware that tracks execution order
var middlewareOrder []string
var middlewareMu sync.Mutex

func orderMiddleware(name string) gortexMiddleware.MiddlewareFunc {
	return func(next gortexMiddleware.HandlerFunc) gortexMiddleware.HandlerFunc {
		return func(c httpctx.Context) error {
			middlewareMu.Lock()
			middlewareOrder = append(middlewareOrder, name)
			middlewareMu.Unlock()
			return next(c)
		}
	}
}

// Handler with middleware tag
type AuthenticatedHandler struct {
	Logger *zap.Logger
}

func (h *AuthenticatedHandler) GET(c httpctx.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "authenticated"})
}

// Nested group with middleware
type AdminGroup struct {
	Users *AuthenticatedHandler `url:"/users"`
}

type MiddlewareHandlersManager struct {
	Public *PublicHandler `url:"/public"`
	Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
}

type PublicHandler struct {
	Logger *zap.Logger
}

func (h *PublicHandler) GET(c httpctx.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "public"})
}

func TestMiddlewareInheritance(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zap.NewNop()
	Register(ctx, logger)

	// For now, we'll test with built-in middleware since custom registration isn't supported yet
	handlersManager := &MiddlewareHandlersManager{
		Public: &PublicHandler{Logger: logger},
		Admin: &AdminGroup{
			Users: &AuthenticatedHandler{Logger: logger},
		},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlersManager)
	assert.NoError(t, err)

	// For now, just test that routes are registered
	tests := []struct {
		name         string
		path         string
		expectedCode int
	}{
		{
			name:         "public route",
			path:         "/public",
			expectedCode: http.StatusOK,
		},
		{
			name:         "admin route",
			path:         "/admin/users",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedCode, rec.Code)
		})
	}
}

// Handler for order test
type OrderTestHandler struct {
	Logger *zap.Logger
}

func (h *OrderTestHandler) GET(c httpctx.Context) error {
	middlewareMu.Lock()
	middlewareOrder = append(middlewareOrder, "handler")
	middlewareMu.Unlock()
	return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
}

func TestMiddlewareExecutionOrder(t *testing.T) {
	// Note: This test assumes we extend parseMiddleware to support named middleware from context
	// For now, it will use the basic implementation
	t.Skip("Skipping until named middleware support is added")
}

// Handler for multi-level test
type MultiLevelHandler struct {
	Logger *zap.Logger
}

func (h *MultiLevelHandler) GET(c httpctx.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"level": "multi"})
}

// Test that middleware can be applied at different levels
func TestMultiLevelMiddleware(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zap.NewNop()
	Register(ctx, logger)

	// Multiple nested groups
	type Level3Group struct {
		Resource *MultiLevelHandler `url:"/resource" middleware:"logger"`
	}

	type Level2Group struct {
		Level3 *Level3Group `url:"/v1" middleware:"recover"`
	}

	type Level1Manager struct {
		Level2 *Level2Group `url:"/api"`
	}

	manager := &Level1Manager{
		Level2: &Level2Group{
			Level3: &Level3Group{
				Resource: &MultiLevelHandler{Logger: logger},
			},
		},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, manager)
	assert.NoError(t, err)

	// Test that the route is registered with all middleware
	req := httptest.NewRequest("GET", "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "multi", response["level"])
}
