package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gortexMiddleware "github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// Test handlers with auto parameter binding
type AutoBindHandler struct {
	Logger *zap.Logger
}

type UserRequest struct {
	ID       int    `json:"id" bind:"id,path"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active" bind:"active,query"`
}

type GameBetRequest struct {
	GameID   string  `json:"game_id" bind:"game_id,path"`
	UserID   int     `json:"user_id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Traditional Gortex handler for comparison
func (h *AutoBindHandler) TraditionalGET(c httpctx.Context) error {
	id := c.Param("id")
	return c.JSON(200, map[string]string{"id": id, "method": "traditional"})
}

// New auto-binding handler - simple case
func (h *AutoBindHandler) GET(c httpctx.Context, id int) error {
	return c.JSON(200, map[string]any{"id": id, "method": "auto-bind"})
}

// New auto-binding handler - struct binding
func (h *AutoBindHandler) POST(c httpctx.Context, req *UserRequest) error {
	h.Logger.Info("Auto-bound request", zap.Any("request", req))
	return c.JSON(200, req)
}

// New auto-binding handler - complex binding
func (h *AutoBindHandler) PlaceBet(c httpctx.Context, req *GameBetRequest) error {
	h.Logger.Info("Placing bet",
		zap.String("game_id", req.GameID),
		zap.Int("user_id", req.UserID),
		zap.Float64("amount", req.Amount))

	return c.JSON(200, map[string]any{
		"success": true,
		"bet":     req,
		"message": "Bet placed successfully",
	})
}

type AutoBindHandlersManager struct {
	Users *AutoBindHandler `url:"/users/:id"`
	Games *AutoGameHandler `url:"/games/:game_id"`
}

type AutoGameHandler struct {
	Logger *zap.Logger
}

func (h *AutoGameHandler) Users(c httpctx.Context, req *GameBetRequest) error {
	return c.JSON(200, req)
}

func TestRouterWithAutoBinder(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewDevelopment()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &AutoBindHandler{Logger: logger}

	// Test traditional vs auto-binding
	t.Run("primitive parameter binding", func(t *testing.T) {
		// Register a single handler
		err := registerHTTPHandlerWithMiddleware(r, "/test/:id", handler,
			reflect.TypeOf(handler), []gortexMiddleware.MiddlewareFunc{}, nil)
		require.NoError(t, err)

		// Test the route
		req := httptest.NewRequest(http.MethodGet, "/test/123", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var result map[string]any
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, float64(123), result["id"])
		assert.Equal(t, "auto-bind", result["method"])
	})

	t.Run("struct parameter binding", func(t *testing.T) {
		// Create request body
		body := map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/test/456?active=true",
			bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Manually set up context for testing
		ctx := context.NewContext(req, rec)
		ctx.SetPath("/test/:id")
		ctx.SetParamNames("id")
		ctx.SetParamValues("456")

		// Use createHandlerFunc to get proper binding
		method, _ := reflect.TypeOf(handler).MethodByName("POST")
		handlerFunc := createHandlerFunc(handler, method)

		err := handlerFunc(ctx)
		require.NoError(t, err)

		var result UserRequest
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, 456, result.ID)
		assert.Equal(t, "John Doe", result.Name)
		assert.Equal(t, "john@example.com", result.Email)
		assert.Equal(t, true, result.IsActive)
	})
}

func TestRouterBinderIntegration(t *testing.T) {
	r := router.NewGortexRouter()
	logger, _ := zap.NewDevelopment()
	ctx := NewContext()
	Register(ctx, logger)

	// Create handlers
	handlers := &AutoBindHandlersManager{
		Users: &AutoBindHandler{Logger: logger},
		Games: &AutoGameHandler{Logger: logger},
	}

	// Create a test app to register routes
	testApp := &App{router: r, ctx: ctx}
	err := RegisterRoutes(testApp, handlers)
	require.NoError(t, err)

	t.Run("nested route with auto binding", func(t *testing.T) {
		// Create request body
		body := map[string]any{
			"user_id":  456,
			"amount":   250.50,
			"currency": "USD",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/games/GAME123/users",
			bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var result GameBetRequest
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, "GAME123", result.GameID)
		assert.Equal(t, 456, result.UserID)
		assert.Equal(t, 250.50, result.Amount)
		assert.Equal(t, "USD", result.Currency)
	})
}
