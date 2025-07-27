package router

import (
	"encoding/json"
	"github.com/yshengliao/gortex/transport/http"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Comprehensive test handlers
type CompUserHandler struct {
	Logger *zap.Logger
}

func (h *CompUserHandler) GET(c context.Context) error {
	userID := c.Param("id")
	if userID == "" {
		// List all users
		return c.JSON(http.StatusOK, map[string]interface{}{
			"users": []string{"user1", "user2"},
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"id":   userID,
		"name": "User " + userID,
	})
}

func (h *CompUserHandler) POST(c context.Context) error {
	return c.JSON(http.StatusCreated, map[string]string{
		"message": "User created",
	})
}

func (h *CompUserHandler) Profile(c context.Context) error {
	userID := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"id":      userID,
		"profile": "Profile of " + userID,
	})
}

func (h *CompUserHandler) Settings(c context.Context) error {
	userID := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"id":       userID,
		"settings": "Settings for " + userID,
	})
}

type CompGameHandler struct {
	Logger *zap.Logger
}

func (h *CompGameHandler) GET(c context.Context) error {
	gameID := c.Param("gameid")
	return c.JSON(http.StatusOK, map[string]string{
		"gameid": gameID,
		"name":   "Game " + gameID,
	})
}

func (h *CompGameHandler) PlaceBet(c context.Context) error {
	gameID := c.Param("gameid")
	var bet map[string]interface{}
	c.Bind(&bet)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"gameid": gameID,
		"bet":    bet,
		"status": "placed",
	})
}

func (h *CompGameHandler) GetBets(c context.Context) error {
	gameID := c.Param("gameid")
	return c.JSON(http.StatusOK, map[string]interface{}{
		"gameid": gameID,
		"bets":   []string{"bet1", "bet2"},
	})
}

type CompStaticHandler struct {
	Logger *zap.Logger
}

func (h *CompStaticHandler) GET(c context.Context) error {
	// Try different ways to get the wildcard parameter
	filepath := c.Param("*")
	if filepath == "" {
		// Try getting from path
		path := c.Path()
		if strings.HasPrefix(path, "/static/") {
			filepath = strings.TrimPrefix(path, "/static/")
		}
	}
	return c.JSON(http.StatusOK, map[string]string{
		"file": filepath,
		"type": "static",
	})
}

// Nested groups
type CompAPIv1Group struct {
	Game  *CompGameHandler `url:"/games/:gameid"`
	Users *CompUserHandler `url:"/users/:id"`
}

type CompAPIv2Group struct {
	Game *CompGameHandler `url:"/game/:gameid" middleware:"logger"`
}

type CompAdminGroup struct {
	Users *CompUserHandler `url:"/users/:id" middleware:"recover"`
}

// Root handlers manager
type ComprehensiveHandlersManager struct {
	Users  *CompUserHandler   `url:"/users/:id"`
	APIv1  *CompAPIv1Group    `url:"/api/v1"`
	APIv2  *CompAPIv2Group    `url:"/api/v2" middleware:"logger"`
	Admin  *CompAdminGroup    `url:"/admin" middleware:"recover,logger"`
	Static *CompStaticHandler `url:"/static/*"`
}

func TestComprehensiveDynamicRoutes(t *testing.T) {
	r := router.NewGortexRouter()
	ctx := NewContext()
	logger := zaptest.NewLogger(t)
	Register(ctx, logger)

	handlersManager := &ComprehensiveHandlersManager{
		Users: &CompUserHandler{Logger: logger},
		APIv1: &CompAPIv1Group{
			Game:  &CompGameHandler{Logger: logger},
			Users: &CompUserHandler{Logger: logger},
		},
		APIv2: &CompAPIv2Group{
			Game: &CompGameHandler{Logger: logger},
		},
		Admin: &CompAdminGroup{
			Users: &CompUserHandler{Logger: logger},
		},
		Static: &CompStaticHandler{Logger: logger},
	}

	err := RegisterRoutes(&App{router: r, ctx: ctx}, handlersManager)
	assert.NoError(t, err)

	// Debug: print registered routes info
	t.Logf("Routes registered successfully")

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
		expectedJSON map[string]interface{}
	}{
		// Basic dynamic routes
		{
			name:         "GET user by ID",
			method:       "GET",
			path:         "/users/123",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"id":   "123",
				"name": "User 123",
			},
		},
		{
			name:         "POST create user",
			method:       "POST",
			path:         "/users/new",
			expectedCode: http.StatusCreated,
			expectedJSON: map[string]interface{}{
				"message": "User created",
			},
		},
		{
			name:         "GET user profile",
			method:       "POST", // Custom methods are POST by default
			path:         "/users/456/profile",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"id":      "456",
				"profile": "Profile of 456",
			},
		},
		{
			name:         "GET user settings",
			method:       "POST",
			path:         "/users/789/settings",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"id":       "789",
				"settings": "Settings for 789",
			},
		},
		// Nested group routes
		{
			name:         "APIv1 GET game",
			method:       "GET",
			path:         "/api/v1/games/sg006",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"gameid": "sg006",
				"name":   "Game sg006",
			},
		},
		{
			name:         "APIv1 place bet",
			method:       "POST",
			path:         "/api/v1/games/sg006/place-bet",
			body:         `{"amount": 100}`,
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"gameid": "sg006",
				"status": "placed",
			},
		},
		{
			name:         "APIv1 get user",
			method:       "GET",
			path:         "/api/v1/users/101",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"id":   "101",
				"name": "User 101",
			},
		},
		{
			name:         "APIv2 GET game with middleware",
			method:       "GET",
			path:         "/api/v2/game/pg001",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"gameid": "pg001",
				"name":   "Game pg001",
			},
		},
		{
			name:         "Admin users with middleware",
			method:       "GET",
			path:         "/admin/users/999",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"id":   "999",
				"name": "User 999",
			},
		},
		// Wildcard routes
		{
			name:         "Static file root",
			method:       "GET",
			path:         "/static/index.html",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"file": "index.html",
				"type": "static",
			},
		},
		{
			name:         "Static file nested",
			method:       "GET",
			path:         "/static/css/styles.css",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"file": "css/styles.css",
				"type": "static",
			},
		},
		{
			name:         "Static file deep nested",
			method:       "GET",
			path:         "/static/js/vendor/lib/util.js",
			expectedCode: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"file": "js/vendor/lib/util.js",
				"type": "static",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedCode, rec.Code)
			if rec.Code == tt.expectedCode && tt.expectedJSON != nil {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)
				for k, v := range tt.expectedJSON {
					assert.Equal(t, v, response[k], "Key: %s", k)
				}
			}
		})
	}
}

// Multiple parameter handler
type MultiParamHandler struct {
	Logger *zap.Logger
}

func (h *MultiParamHandler) GET(c context.Context) error {
	category := c.Param("category")
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{
		"category": category,
		"id":       id,
	})
}

// Edge case handler
type EdgeHandler struct {
	Logger *zap.Logger
}

func (h *EdgeHandler) GET(c context.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{"id": id})
}

// Test edge cases
func TestDynamicRouteEdgeCases(t *testing.T) {
	t.Run("Empty parameter value", func(t *testing.T) {
		r := router.NewGortexRouter()
		ctx := NewContext()
		logger := zap.NewNop()
		Register(ctx, logger)

		type EdgeManager struct {
			Handler *EdgeHandler `url:"/test/:id"`
		}

		manager := &EdgeManager{
			Handler: &EdgeHandler{Logger: logger},
		}

		err := RegisterRoutes(&App{router: r, ctx: ctx}, manager)
		assert.NoError(t, err)

		// Test with empty parameter - should return 404
		req := httptest.NewRequest("GET", "/test/", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Multiple parameters", func(t *testing.T) {
		r := router.NewGortexRouter()
		ctx := NewContext()
		logger := zap.NewNop()
		Register(ctx, logger)

		type MultiParamManager struct {
			Handler *MultiParamHandler `url:"/:category/:id"`
		}

		manager := &MultiParamManager{
			Handler: &MultiParamHandler{Logger: logger},
		}

		err := RegisterRoutes(&App{router: r, ctx: ctx}, manager)
		assert.NoError(t, err)

		req := httptest.NewRequest("GET", "/products/123", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "products", response["category"])
		assert.Equal(t, "123", response["id"])
	})
}
