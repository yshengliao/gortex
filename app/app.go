// Package app provides the core application framework for building HTTP and WebSocket servers
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/config"
	errorMiddleware "github.com/yshengliao/gortex/middleware"
)

// ShutdownHook is a function that gets called during shutdown
type ShutdownHook func(ctx context.Context) error

// App represents the main application instance
type App struct {
	e               *echo.Echo
	config          *Config
	logger          *zap.Logger
	ctx             *Context
	shutdownHooks   []ShutdownHook
	shutdownTimeout time.Duration
	mu              sync.RWMutex
}

// Config is re-exported from the config package for convenience
type Config = config.Config

// Option defines a functional option for App
type Option func(*App) error

// NewApp creates a new application instance with the given options
func NewApp(opts ...Option) (*App, error) {
	app := &App{
		e:               echo.New(),
		ctx:             NewContext(),
		shutdownHooks:   make([]ShutdownHook, 0),
		shutdownTimeout: 30 * time.Second, // Default 30 seconds
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

// WithShutdownTimeout sets the shutdown timeout duration
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(app *App) error {
		if timeout <= 0 {
			return fmt.Errorf("shutdown timeout must be positive")
		}
		app.shutdownTimeout = timeout
		return nil
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

// RegisterShutdownHook registers a function to be called during shutdown
func (app *App) RegisterShutdownHook(hook ShutdownHook) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.shutdownHooks = append(app.shutdownHooks, hook)
}

// OnShutdown is a convenience method for registering shutdown hooks
func (app *App) OnShutdown(fn func(context.Context) error) {
	app.RegisterShutdownHook(ShutdownHook(fn))
}

// Shutdown gracefully shuts down the server
func (app *App) Shutdown(ctx context.Context) error {
	if app.logger != nil {
		app.logger.Info("Starting graceful shutdown")
	}

	// Create a timeout context if none provided
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, app.shutdownTimeout)
		defer cancel()
	}

	var shutdownErr error

	// Run pre-shutdown hooks in parallel
	if err := app.runShutdownHooks(ctx); err != nil {
		if app.logger != nil {
			app.logger.Error("Error running shutdown hooks", zap.Error(err))
		}
		shutdownErr = err
	}

	// Shutdown the Echo server
	if app.logger != nil {
		app.logger.Info("Shutting down HTTP server")
	}
	
	if err := app.e.Shutdown(ctx); err != nil {
		if app.logger != nil {
			app.logger.Error("Error shutting down HTTP server", zap.Error(err))
		}
		// Server shutdown error takes precedence
		return err
	}

	if app.logger != nil {
		app.logger.Info("Graceful shutdown completed")
	}
	
	return shutdownErr
}

// runShutdownHooks executes all registered shutdown hooks
func (app *App) runShutdownHooks(ctx context.Context) error {
	app.mu.RLock()
	hooks := make([]ShutdownHook, len(app.shutdownHooks))
	copy(hooks, app.shutdownHooks)
	app.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	if app.logger != nil {
		app.logger.Info("Running shutdown hooks", zap.Int("count", len(hooks)))
	}

	// Run hooks in parallel with error collection
	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(idx int, h ShutdownHook) {
			defer wg.Done()
			
			if app.logger != nil {
				app.logger.Debug("Running shutdown hook", zap.Int("index", idx))
			}
			
			if err := h(ctx); err != nil {
				errChan <- fmt.Errorf("shutdown hook %d failed: %w", idx, err)
			}
		}(i, hook)
	}

	// Wait for all hooks to complete or context to expire
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All hooks completed
	case <-ctx.Done():
		// Context expired
		return fmt.Errorf("shutdown hooks timed out: %w", ctx.Err())
	}

	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", errs)
	}

	return nil
}