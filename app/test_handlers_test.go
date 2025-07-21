package app

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Test handler types for testing router functionality

// TestCodegenHandler is a test handler with standard HTTP methods
type TestCodegenHandler struct {
	Logger *zap.Logger
}

func (h *TestCodegenHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "GET"})
}

func (h *TestCodegenHandler) POST(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "POST"})
}

func (h *TestCodegenHandler) CustomAction(c echo.Context) error {
	return c.JSON(200, map[string]string{"method": "CustomAction"})
}

// TestCodegenWSHandler is a test WebSocket handler
type TestCodegenWSHandler struct {
	Logger *zap.Logger
}

func (h *TestCodegenWSHandler) HandleConnection(c echo.Context) error {
	return c.JSON(200, map[string]string{"type": "websocket"})
}

// HandlersManager contains all application handlers for testing
// This is used in tests to verify router functionality
type HandlersManager struct {
	API       *TestCodegenHandler   `url:"/api"`
	Users     *TestCodegenHandler   `url:"/users"`
	WebSocket *TestCodegenWSHandler `url:"/ws" hijack:"ws"`
	Root      *TestCodegenHandler   `url:"/"`
}