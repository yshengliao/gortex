package router

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Test handlers for routes logging
type RoutesLogHandlersManager struct {
	Home  *RoutesLogHomeHandler `url:"/"`
	User  *RoutesLogUserHandler `url:"/users/:id"`
	Admin *RoutesLogAdminGroup  `url:"/admin" middleware:"auth"`
	API   *RoutesLogAPIGroup    `url:"/api"`
}

type RoutesLogHomeHandler struct{}

func (h *RoutesLogHomeHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"message": "Home"})
}

type RoutesLogUserHandler struct{}

func (h *RoutesLogUserHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"user": c.Param("id")})
}

func (h *RoutesLogUserHandler) POST(c httpctx.Context) error {
	return c.JSON(201, map[string]string{"created": "user"})
}

func (h *RoutesLogUserHandler) Profile(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"profile": "data"})
}

type RoutesLogAdminGroup struct {
	Dashboard *RoutesLogDashboardHandler `url:"/dashboard"`
	Users     *RoutesLogUsersHandler     `url:"/users"`
}

type RoutesLogDashboardHandler struct{}

func (h *RoutesLogDashboardHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"message": "Dashboard"})
}

type RoutesLogUsersHandler struct{}

func (h *RoutesLogUsersHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"message": "Admin Users"})
}

type RoutesLogAPIGroup struct {
	V1 *RoutesLogAPIv1Group `url:"/v1"`
}

type RoutesLogAPIv1Group struct {
	Products *RoutesLogProductsHandler `url:"/products/:id"`
}

type RoutesLogProductsHandler struct{}

func (h *RoutesLogProductsHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{"product": c.Param("id")})
}

func TestRoutesLogger(t *testing.T) {
	t.Run("WithRoutesLogger enables route logging", func(t *testing.T) {
		// Create a buffer to capture log output
		var buf bytes.Buffer

		// Create a custom logger that writes to our buffer
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(&buf),
			zapcore.InfoLevel,
		)
		logger := zap.New(core)

		// Create handlers
		handlers := &RoutesLogHandlersManager{}

		// Create app with routes logger enabled
		cfg := &Config{}
		cfg.Server.Address = ":18181"
		cfg.Logger.Level = "info"

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
			WithRoutesLogger(), // Enable routes logging
			WithHandlers(handlers),
		)
		require.NoError(t, err)
		require.NotNil(t, app)

		// Check that routes were logged
		logOutput := buf.String()
		assert.Contains(t, logOutput, "Registered routes")
		assert.Contains(t, logOutput, "GET")
		assert.Contains(t, logOutput, "/")
		assert.Contains(t, logOutput, "RoutesLogHomeHandler")
		assert.Contains(t, logOutput, "/users/:id")
		assert.Contains(t, logOutput, "RoutesLogUserHandler")
		assert.Contains(t, logOutput, "/users/:id/profile")
		assert.Contains(t, logOutput, "/admin/dashboard")
		assert.Contains(t, logOutput, "/admin/users")
		assert.Contains(t, logOutput, "/api/v1/products/:id")

		// Debug: print how many routes were collected
		t.Logf("Collected %d routes", len(app.routeInfos))
		for i, route := range app.routeInfos {
			t.Logf("Route %d: %s %s -> %s (middlewares: %v)", i, route.Method, route.Path, route.Handler, route.Middlewares)
		}

		// Verify route infos were collected
		assert.Greater(t, len(app.routeInfos), 0)

		// Check specific routes
		foundHome := false
		foundUserGET := false
		foundUserPOST := false
		foundUserProfile := false
		foundAdminDashboard := false

		for _, route := range app.routeInfos {
			switch {
			case route.Method == "GET" && route.Path == "/":
				foundHome = true
				assert.Equal(t, "RoutesLogHomeHandler", route.Handler)
			case route.Method == "GET" && route.Path == "/users/:id":
				foundUserGET = true
				assert.Equal(t, "RoutesLogUserHandler", route.Handler)
			case route.Method == "POST" && route.Path == "/users/:id":
				foundUserPOST = true
				assert.Equal(t, "RoutesLogUserHandler", route.Handler)
			case route.Method == "POST" && route.Path == "/users/:id/profile":
				foundUserProfile = true
				assert.Equal(t, "RoutesLogUserHandler.Profile", route.Handler)
			case route.Method == "GET" && route.Path == "/admin/dashboard":
				foundAdminDashboard = true
				assert.Equal(t, "RoutesLogDashboardHandler", route.Handler)
				// Should have middleware (but we're not actually parsing middleware names yet)
				// assert.NotEmpty(t, route.Middlewares)
			}
		}

		assert.True(t, foundHome, "Home route not found")
		assert.True(t, foundUserGET, "User GET route not found")
		assert.True(t, foundUserPOST, "User POST route not found")
		assert.True(t, foundUserProfile, "User profile route not found")
		assert.True(t, foundAdminDashboard, "Admin dashboard route not found")
	})

	t.Run("Without WithRoutesLogger, no routes are logged", func(t *testing.T) {
		// Create a buffer to capture log output
		var buf bytes.Buffer

		// Create a custom logger
		encoderConfig := zapcore.EncoderConfig{
			MessageKey:  "msg",
			LevelKey:    "level",
			EncodeLevel: zapcore.LowercaseLevelEncoder,
			LineEnding:  zapcore.DefaultLineEnding,
		}

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(&buf),
			zapcore.InfoLevel,
		)
		logger := zap.New(core)

		// Create handlers
		handlers := &RoutesLogHandlersManager{}

		// Create app without routes logger
		cfg := &Config{}
		cfg.Server.Address = ":18182"
		cfg.Logger.Level = "info"

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
			WithHandlers(handlers), // No WithRoutesLogger()
		)
		require.NoError(t, err)
		require.NotNil(t, app)

		// Check that routes were NOT logged
		logOutput := buf.String()
		assert.NotContains(t, logOutput, "Registered routes")

		// Route infos should not be collected
		assert.Empty(t, app.routeInfos)
	})

	t.Run("Route table formatting", func(t *testing.T) {
		// Create a buffer to capture log output
		var buf bytes.Buffer

		// Create a custom logger
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(&buf),
			zapcore.InfoLevel,
		)
		logger := zap.New(core)

		// Create simple handlers for predictable output
		type RoutesLogSimpleAPIGroup struct {
			Users *RoutesLogUserHandler `url:"/users/:id"`
		}

		type SimpleHandlers struct {
			Home *RoutesLogHomeHandler    `url:"/"`
			API  *RoutesLogSimpleAPIGroup `url:"/api"`
		}

		handlers := &SimpleHandlers{}

		// Create app with routes logger
		cfg := &Config{}
		cfg.Server.Address = ":18183"

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
			WithRoutesLogger(),
			WithHandlers(handlers),
		)
		require.NoError(t, err)
		require.NotNil(t, app)

		// Check table formatting
		logOutput := buf.String()
		lines := strings.Split(logOutput, "\n")

		// Find the table in the output
		tableFound := false
		for _, line := range lines {
			if strings.Contains(line, "┌") && strings.Contains(line, "┬") && strings.Contains(line, "┐") {
				tableFound = true
				break
			}
		}

		assert.True(t, tableFound, "Route table not found in output")

		// Verify table contains expected elements
		assert.Contains(t, logOutput, "│ Method")
		assert.Contains(t, logOutput, "│ Path")
		assert.Contains(t, logOutput, "│ Handler")
		assert.Contains(t, logOutput, "│ Middlewares")
		assert.Contains(t, logOutput, "├")
		assert.Contains(t, logOutput, "┼")
		assert.Contains(t, logOutput, "┤")
		assert.Contains(t, logOutput, "└")
		assert.Contains(t, logOutput, "┴")
		assert.Contains(t, logOutput, "┘")
	})
}
