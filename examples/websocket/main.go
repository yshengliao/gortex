package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap"
)

// HandlersManager defines all handlers with declarative routing
type HandlersManager struct {
	WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
}

// WebSocketHandler demonstrates WebSocket handling
type WebSocketHandler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Configure appropriately for production
		},
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	// Create client
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	client := hub.NewClient(h.Hub, conn, clientID, h.Logger)
	h.Hub.RegisterClient(client)

	// Start client message pumps
	go client.WritePump()
	go client.ReadPump()

	h.Logger.Info("Client connected", zap.String("client_id", clientID))
	return nil
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize WebSocket hub
	wsHub := hub.NewHub(logger)

	// Create handlers
	handlers := &HandlersManager{
		WebSocket: &WebSocketHandler{
			Hub:    wsHub,
			Logger: logger,
		},
	}

	// Create application with functional options
	cfg := &app.Config{}
	cfg.Server.Address = ":8082"
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Start WebSocket hub
	go wsHub.Run()

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("WebSocket server started on :8082", zap.String("websocket", "/ws"))
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}
}