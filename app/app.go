// Package app provides the core application framework for building HTTP and WebSocket servers
package app

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/config"
	errorMiddleware "github.com/yshengliao/gortex/middleware"
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
		return RegisterRoutes(app.e, manager, app.ctx)
	}
}

// setupEcho configures the Echo instance with middleware and settings
func (app *App) setupEcho() {
	// Disable Echo's banner
	app.e.HideBanner = true
	app.e.HidePort = true

	// Error handler middleware will handle all errors consistently
	// Remove the custom error handler in favor of middleware

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

	// Request ID middleware (must come before error handler)
	// Use our custom request ID middleware for enhanced functionality
	app.e.Use(errorMiddleware.RequestID())

	// Error handler middleware for consistent error responses
	// Hide internal server error details in production (when logger level is not debug)
	hideDetails := true
	if app.config != nil && app.config.Logger.Level == "debug" {
		hideDetails = false
	}
	
	errorConfig := &errorMiddleware.ErrorHandlerConfig{
		Logger: app.logger,
		HideInternalServerErrorDetails: hideDetails,
		DefaultMessage: "An internal error occurred",
	}
	app.e.Use(errorMiddleware.ErrorHandlerWithConfig(errorConfig))
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