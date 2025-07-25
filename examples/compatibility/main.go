package main

import (
	"fmt"
	"log"
	"net/http"
	
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/config"
	"go.uber.org/zap"
)

// DemoHandler shows compatibility between Echo and Gortex
type DemoHandler struct {
	logger *zap.Logger
}

// Echo-style handler
func (h *DemoHandler) HelloEcho(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		name = "World"
	}
	
	// Using Echo Context API
	c.Set("handler", "echo")
	mode := c.Get("handler").(string)
	
	return c.JSON(http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Hello, %s from %s mode!", name, mode),
		"mode":    "echo",
	})
}

// HandlersManager for route registration
type HandlersManager struct {
	Demo *DemoHandler `url:"/demo"`
}

func main() {
	// Create configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:  ":8083",
			Recovery: true,
			CORS:     true,
		},
		Logger: config.LoggerConfig{
			Level: "debug",
		},
	}
	
	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	// Create demo handler
	demoHandler := &DemoHandler{
		logger: logger,
	}
	
	// Create handlers manager
	handlersManager := &HandlersManager{
		Demo: demoHandler,
	}
	
	// Create app with compatibility mode
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
		app.WithRuntimeMode(app.ModeEcho), // Can switch to ModeGortex or ModeDual
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}
	
	// Add a route to show current mode
	application.Echo().GET("/mode", func(c echo.Context) error {
		mode := "Echo"
		switch cfg.Logger.Level {
		case "debug":
			mode = "Echo (Debug)"
		}
		return c.JSON(http.StatusOK, map[string]string{
			"runtime_mode": mode,
			"message":      "Running in Echo compatibility mode",
			"router":      "Echo v4",
		})
	})
	
	logger.Info("Starting compatibility demo server on :8083")
	
	// Run the server
	if err := application.Run(); err != nil {
		log.Fatal("Server failed:", err)
	}
}