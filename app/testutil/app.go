package testutil

import (
	"testing"

	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/config"
	"go.uber.org/zap"
)

// NewTestApp creates a new app instance for testing with minimal configuration
func NewTestApp(t *testing.T, handlers interface{}) *app.App {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{}
	cfg.Server.Address = ":0"  // Random port
	cfg.Logger.Level = "error" // Reduce noise

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}

	return application
}

// NewTestAppWithConfig creates a new app instance with custom configuration
func NewTestAppWithConfig(t *testing.T, cfg *config.Config, handlers interface{}) *app.App {
	t.Helper()

	logger, _ := zap.NewDevelopment()

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}

	return application
}

// NewMinimalApp creates a minimal app for basic testing
func NewMinimalApp(t *testing.T) *app.App {
	t.Helper()

	logger, _ := zap.NewDevelopment()

	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create minimal app: %v", err)
	}

	return application
}
