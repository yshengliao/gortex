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
	Home      *HomeHandler      `url:"/"`
	WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
	Metrics   *MetricsHandler   `url:"/metrics"`
}

// HomeHandler serves a simple WebSocket client page
type HomeHandler struct{}

func (h *HomeHandler) GET(c echo.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Gortex WebSocket Example</title>
</head>
<body>
    <h1>Gortex WebSocket Example</h1>
    <div id="status">Disconnected</div>
    <div id="messages"></div>
    <input type="text" id="messageInput" placeholder="Type a message...">
    <button onclick="sendMessage()">Send</button>
    
    <script>
        const ws = new WebSocket('ws://localhost:8080/ws');
        const status = document.getElementById('status');
        const messages = document.getElementById('messages');
        
        ws.onopen = () => {
            status.textContent = 'Connected';
            status.style.color = 'green';
        };
        
        ws.onclose = () => {
            status.textContent = 'Disconnected';
            status.style.color = 'red';
        };
        
        ws.onmessage = (event) => {
            const msg = document.createElement('div');
            msg.textContent = new Date().toLocaleTimeString() + ': ' + event.data;
            messages.appendChild(msg);
        };
        
        function sendMessage() {
            const input = document.getElementById('messageInput');
            if (input.value && ws.readyState === WebSocket.OPEN) {
                ws.send(input.value);
                input.value = '';
            }
        }
        
        document.getElementById('messageInput').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') sendMessage();
        });
    </script>
</body>
</html>`
	return c.HTML(200, html)
}

// WebSocketHandler manages WebSocket connections
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

	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())
	client := hub.NewClient(h.Hub, conn, userID, h.Logger)
	h.Hub.RegisterClient(client)

	// Start client message pumps
	go client.WritePump()
	go client.ReadPump()

	return nil
}

// MetricsHandler provides WebSocket metrics
type MetricsHandler struct {
	Hub *hub.Hub
}

func (h *MetricsHandler) GET(c echo.Context) error {
	metrics := h.Hub.GetMetrics()
	return c.JSON(200, map[string]interface{}{
		"current_connections": metrics.CurrentConnections,
		"total_connections":   metrics.TotalConnections,
		"messages_sent":       metrics.MessagesSent,
		"messages_received":   metrics.MessagesReceived,
		"uptime":              metrics.Uptime.String(),
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Server.Recovery = true

	// Initialize WebSocket hub
	wsHub := hub.NewHub(logger)

	// Create handlers
	handlers := &HandlersManager{
		Home: &HomeHandler{},
		WebSocket: &WebSocketHandler{
			Hub:    wsHub,
			Logger: logger,
		},
		Metrics: &MetricsHandler{
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

	// Start WebSocket hub
	go wsHub.Run()

	// Register shutdown hook for WebSocket hub
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Shutting down WebSocket hub")
		return wsHub.ShutdownWithTimeout(5 * time.Second)
	})

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		logger.Info("WebSocket server started", 
			zap.String("address", cfg.Server.Address),
			zap.String("websocket", "/ws"),
			zap.String("metrics", "/metrics"),
		)
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