package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap"
)

// HandlersManager defines all application handlers
type HandlersManager struct {
	WebSocket       *WebSocketHandler       `url:"/ws" hijack:"ws"`
	WebSocketMetrics *WebSocketMetricsHandler `url:"/api/ws/metrics"`
}

// WebSocketHandler manages WebSocket connections
type WebSocketHandler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = fmt.Sprintf("user-%d", time.Now().Unix())
	}

	client := hub.NewClient(h.Hub, conn, userID, h.Logger)
	h.Hub.RegisterClient(client)

	go client.WritePump()
	go client.ReadPump()

	return nil
}

// WebSocketMetricsHandler provides WebSocket metrics endpoint
type WebSocketMetricsHandler struct {
	Hub *hub.Hub
}

func (h *WebSocketMetricsHandler) GET(c echo.Context) error {
	metrics := h.Hub.GetMetrics()
	sentRate, receivedRate := h.Hub.GetMessageRate()

	return c.JSON(200, map[string]interface{}{
		"metrics": metrics,
		"rates": map[string]float64{
			"messages_sent_per_second":     sentRate,
			"messages_received_per_second": receivedRate,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// SimulatedChatBot sends periodic messages to simulate activity
type SimulatedChatBot struct {
	hub    *hub.Hub
	logger *zap.Logger
	stop   chan struct{}
}

func NewSimulatedChatBot(hub *hub.Hub, logger *zap.Logger) *SimulatedChatBot {
	return &SimulatedChatBot{
		hub:    hub,
		logger: logger,
		stop:   make(chan struct{}),
	}
}

func (b *SimulatedChatBot) Start() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		messageTypes := []string{"chat", "status", "notification", "system"}
		messageIndex := 0

		for {
			select {
			case <-ticker.C:
				// Send different types of messages
				msgType := messageTypes[messageIndex%len(messageTypes)]
				messageIndex++

				msg := &hub.Message{
					Type: msgType,
					Data: map[string]interface{}{
						"from":      "bot",
						"timestamp": time.Now().Unix(),
					},
				}

				switch msgType {
				case "chat":
					msg.Data["text"] = fmt.Sprintf("Automated message #%d", messageIndex)
				case "status":
					msg.Data["status"] = "online"
					msg.Data["users_online"] = b.hub.GetConnectedClients()
				case "notification":
					msg.Data["title"] = "System Notification"
					msg.Data["body"] = fmt.Sprintf("This is notification #%d", messageIndex)
				case "system":
					metrics := b.hub.GetMetrics()
					msg.Data["metrics"] = map[string]interface{}{
						"connections": metrics.CurrentConnections,
						"uptime":      metrics.Uptime.String(),
					}
				}

				b.hub.Broadcast(msg)
				b.logger.Info("Bot sent message", 
					zap.String("type", msgType),
					zap.Int("index", messageIndex))

			case <-b.stop:
				return
			}
		}
	}()
}

func (b *SimulatedChatBot) Stop() {
	close(b.stop)
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"

	// Initialize WebSocket hub
	wsHub := hub.NewHub(logger)

	// Create handlers
	handlers := &HandlersManager{
		WebSocket: &WebSocketHandler{
			Hub:    wsHub,
			Logger: logger,
		},
		WebSocketMetrics: &WebSocketMetricsHandler{
			Hub: wsHub,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Register services
	app.Register(application.Context(), logger)
	app.Register(application.Context(), wsHub)

	// Start WebSocket hub
	go wsHub.Run()

	// Start simulated chat bot
	bot := NewSimulatedChatBot(wsHub, logger)
	bot.Start()
	defer bot.Stop()

	// Start periodic metrics logger
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			metrics := wsHub.GetMetrics()
			sentRate, receivedRate := wsHub.GetMessageRate()

			logger.Info("WebSocket metrics snapshot",
				zap.Int("current_connections", metrics.CurrentConnections),
				zap.Int64("total_connections", metrics.TotalConnections),
				zap.Int64("messages_sent", metrics.MessagesSent),
				zap.Int64("messages_received", metrics.MessagesReceived),
				zap.Float64("sent_rate_per_sec", sentRate),
				zap.Float64("received_rate_per_sec", receivedRate),
				zap.Duration("uptime", metrics.Uptime),
			)

			// Log message type breakdown
			for msgType, count := range metrics.MessageTypes {
				logger.Info("Message type count",
					zap.String("type", msgType),
					zap.Int64("count", count),
				)
			}
		}
	}()

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		logger.Info("WebSocket metrics example started", 
			zap.String("address", cfg.Server.Address),
			zap.String("metrics_endpoint", "/api/ws/metrics"),
			zap.String("websocket_endpoint", "/ws"),
		)
		
		logger.Info("Test the metrics endpoint with: curl http://localhost:8080/api/ws/metrics")
		
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}