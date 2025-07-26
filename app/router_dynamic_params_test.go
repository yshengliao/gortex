package app

import (
	"github.com/yshengliao/gortex/router"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Test handler with dynamic parameters
type DynamicParamHandler struct {
	Logger *zap.Logger
}

func (h *DynamicParamHandler) GET(c context.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"id": id,
	})
}

func (h *DynamicParamHandler) GetProfile(c context.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"id":     id,
		"action": "profile",
	})
}

// Test handler with multiple dynamic parameters
type GameHandler struct {
	Logger *zap.Logger
}

func (h *GameHandler) GET(c context.Context) error {
	gameID := c.Param("gameid")
	return c.JSON(http.StatusOK, map[string]string{
		"gameid": gameID,
	})
}

func (h *GameHandler) PlaceBet(c context.Context) error {
	gameID := c.Param("gameid")
	betID := c.Param("betid")
	return c.JSON(http.StatusOK, map[string]string{
		"gameid": gameID,
		"betid":  betID,
	})
}

// Test handlers manager with dynamic routes
type DynamicHandlersManager struct {
	User *DynamicParamHandler `url:"/users/:id"`
	Game *GameHandler         `url:"/api/v1/:gameid"`
}

func TestDynamicRouteParameters(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
		expectedBody map[string]string
	}{
		{
			name:         "user with ID",
			method:       "GET",
			path:         "/users/123",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"id": "123"},
		},
		{
			name:         "user profile",
			method:       "POST",
			path:         "/users/456/get-profile",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"id": "456", "action": "profile"},
		},
		{
			name:         "game with ID",
			method:       "GET",
			path:         "/api/v1/sg006",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"gameid": "sg006"},
		},
		{
			name:         "game place bet",
			method:       "POST",
			path:         "/api/v1/sg006/place-bet",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"gameid": "sg006", "betid": ""},
		},
	}

	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zap.NewNop()
	Register(ctx, logger)

	handlersManager := &DynamicHandlersManager{
		User: &DynamicParamHandler{Logger: logger},
		Game: &GameHandler{Logger: logger},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlersManager)
	assert.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedCode, rec.Code)
			if tt.expectedBody != nil {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)
				for k, v := range tt.expectedBody {
					if v != "" {
						assert.Equal(t, v, response[k])
					}
				}
			}
		})
	}
}

// Test nested route groups with dynamic parameters
type APIv1Group struct {
	Game *GameHandler `url:"/:gameid"`
}

type NestedHandlersManager struct {
	APIv1 *APIv1Group `url:"/api/v1"`
}

func TestNestedGroupsWithDynamicParams(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zap.NewNop()
	Register(ctx, logger)

	handlersManager := &NestedHandlersManager{
		APIv1: &APIv1Group{
			Game: &GameHandler{Logger: logger},
		},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlersManager)
	assert.NoError(t, err)

	// Debug: print registered routes info
	t.Logf("Routes registered successfully")

	// Test nested routes work correctly
	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
		expectedBody map[string]string
	}{
		{
			name:         "nested game with ID",
			method:       "GET",
			path:         "/api/v1/sg006",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"gameid": "sg006"},
		},
		{
			name:         "nested game place bet",
			method:       "POST",
			path:         "/api/v1/sg006/place-bet",
			expectedCode: http.StatusOK,
			expectedBody: map[string]string{"gameid": "sg006"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedCode, rec.Code)
			if tt.expectedBody != nil {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)
				for k, v := range tt.expectedBody {
					if v != "" {
						assert.Equal(t, v, response[k])
					}
				}
			}
		})
	}
}

// Test wildcard routes
type StaticHandler struct {
	Logger *zap.Logger
}

func (h *StaticHandler) GET(c context.Context) error {
	filepath := c.Param("*")
	return c.JSON(http.StatusOK, map[string]string{
		"filepath": filepath,
	})
}

type WildcardHandlersManager struct {
	Static *StaticHandler `url:"/static/*"`
}

func TestWildcardRoutes(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zap.NewNop()
	Register(ctx, logger)

	handlersManager := &WildcardHandlersManager{
		Static: &StaticHandler{Logger: logger},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlersManager)
	assert.NoError(t, err)

	tests := []struct {
		name         string
		path         string
		expectedFile string
	}{
		{
			name:         "root file",
			path:         "/static/index.html",
			expectedFile: "index.html",
		},
		{
			name:         "nested file",
			path:         "/static/css/style.css",
			expectedFile: "css/style.css",
		},
		{
			name:         "deep nested file",
			path:         "/static/js/vendor/lib.js",
			expectedFile: "js/vendor/lib.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			var response map[string]string
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFile, response["filepath"])
		})
	}
}
