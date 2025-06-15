// Package app provides the core application framework for building HTTP and WebSocket servers
package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/config"
)

// App represents the main application instance
type App struct {
	e      *echo.Echo
	config *Config
	logger *zap.Logger
	ctx    *Context
}

// Config is re-exported from the config package for convenience
type Config = config.Config

// Option defines a functional option for App
type Option func(*App) error

// NewApp creates a new application instance with the given options
func NewApp(opts ...Option) (*App, error) {
	app := &App{
		e:   echo.New(),
		ctx: NewContext(),
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Configure Echo
	app.setupEcho()

	return app, nil
}

// WithConfig sets the application configuration
func WithConfig(cfg *Config) Option {
	return func(app *App) error {
		if cfg == nil {
			return fmt.Errorf("config cannot be nil")
		}
		app.config = cfg
		return nil
	}
}

// WithLogger sets the application logger
func WithLogger(logger *zap.Logger) Option {
	return func(app *App) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		app.logger = logger
		return nil
	}
}

// WithHandlers registers handlers using reflection
func WithHandlers(manager interface{}) Option {
	return func(app *App) error {
		return RegisterRoutesFromStruct(app.e, manager, app.ctx)
	}
}

// setupEcho configures the Echo instance with middleware and settings
func (app *App) setupEcho() {
	// Disable Echo's banner
	app.e.HideBanner = true
	app.e.HidePort = true

	// Set custom error handler
	app.e.HTTPErrorHandler = app.customErrorHandler

	// Apply middleware based on configuration
	if app.config == nil || app.config.Server.Recovery {
		app.e.Use(middleware.Recover())
	}
	if app.config != nil && app.config.Server.GZip {
		app.e.Use(middleware.Gzip())
	}
	if app.config != nil && app.config.Server.CORS {
		app.e.Use(middleware.CORS())
	}
	if app.config == nil {
		app.e.Use(middleware.Logger())
	}

	// Request ID middleware
	app.e.Use(middleware.RequestID())
}

// customErrorHandler handles errors in a consistent way
func (app *App) customErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "Internal Server Error"

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		message = fmt.Sprintf("%v", he.Message)
	}

	// Log the error
	if app.logger != nil {
		app.logger.Error("HTTP error",
			zap.Int("code", code),
			zap.String("message", message),
			zap.String("path", c.Request().URL.Path),
			zap.String("method", c.Request().Method),
			zap.Error(err))
	}

	// Send response
	if !c.Response().Committed {
		if c.Request().Method == http.MethodHead {
			c.NoContent(code)
		} else {
			c.JSON(code, map[string]interface{}{
				"error": message,
				"code":  code,
			})
		}
	}
}

// Echo returns the underlying Echo instance
func (app *App) Echo() *echo.Echo {
	return app.e
}

// Context returns the application context
func (app *App) Context() *Context {
	return app.ctx
}

// Run starts the HTTP server
func (app *App) Run() error {
	address := ":8080"
	if app.config != nil && app.config.Server.Address != "" {
		address = app.config.Server.Address
	}

	if app.logger != nil {
		app.logger.Info("Starting server", zap.String("address", address))
	}

	return app.e.Start(address)
}

// Shutdown gracefully shuts down the server
func (app *App) Shutdown(ctx context.Context) error {
	if app.logger != nil {
		app.logger.Info("Shutting down server")
	}
	return app.e.Shutdown(ctx)
}