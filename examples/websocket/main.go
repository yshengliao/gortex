package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yshengliao/gortex/app"
	gortexContext "github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap"
)

// HandlersManager demonstrates WebSocket with struct tags
type HandlersManager struct {
	// Regular HTTP endpoints
	Home   *HomeHandler   `url:"/"`
	Status *StatusHandler `url:"/status"`
	
	// WebSocket endpoint with hijack tag
	WS     *WSHandler     `url:"/ws" hijack:"ws"`
	
	// API for sending messages
	API    *APIHandler    `url:"/api"`
}

// HomeHandler serves the WebSocket client page
type HomeHandler struct{}

func (h *HomeHandler) GET(c gortexContext.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Gortex WebSocket Example</title>
</head>
<body>
    <h1>WebSocket Chat</h1>
    <div id="messages" style="height:300px;overflow:auto;border:1px solid #ccc;margin:10px 0;padding:10px;"></div>
    <input type="text" id="messageInput" placeholder="Type a message..." style="width:70%;">
    <button onclick="sendMessage()">Send</button>
    <button onclick="connect()">Connect</button>
    <button onclick="disconnect()">Disconnect</button>
    
    <script>
        let ws;
        
        function connect() {
            ws = new WebSocket('ws://localhost:8082/ws');
            
            ws.onopen = function() {
                appendMessage('Connected to server');
            };
            
            ws.onmessage = function(event) {
                appendMessage('Server: ' + event.data);
            };
            
            ws.onclose = function() {
                appendMessage('Disconnected from server');
            };
            
            ws.onerror = function(error) {
                appendMessage('Error: ' + error);
            };
        }
        
        function disconnect() {
            if (ws) {
                ws.close();
            }
        }
        
        function sendMessage() {
            const input = document.getElementById('messageInput');
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(input.value);
                appendMessage('You: ' + input.value);
                input.value = '';
            } else {
                alert('Not connected!');
            }
        }
        
        function appendMessage(msg) {
            const messages = document.getElementById('messages');
            messages.innerHTML += '<div>' + new Date().toLocaleTimeString() + ' - ' + msg + '</div>';
            messages.scrollTop = messages.scrollHeight;
        }
        
        // Connect automatically
        connect();
    </script>
</body>
</html>`
	
	return c.HTML(200, html)
}

// StatusHandler shows WebSocket hub status
type StatusHandler struct {
	hub *hub.Hub
}

func (h *StatusHandler) GET(c gortexContext.Context) error {
	metrics := h.hub.GetMetrics()
	sentRate, receivedRate := h.hub.GetMessageRate()
	return c.JSON(200, map[string]interface{}{
		"connections": metrics.CurrentConnections,
		"total":       metrics.TotalConnections,
		"messages": map[string]interface{}{
			"sent":     metrics.MessagesSent,
			"received": metrics.MessagesReceived,
			"rate": map[string]float64{
				"sent":     sentRate,
				"received": receivedRate,
			},
		},
	})
}

// WSHandler handles WebSocket connections
type WSHandler struct {
	hub      *hub.Hub
	upgrader websocket.Upgrader
	logger   *zap.Logger
}

// HandleConnection upgrades HTTP to WebSocket (marked with hijack:"ws")
func (h *WSHandler) HandleConnection(c gortexContext.Context) error {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	
	// Create client with unique ID
	clientID := "client-" + time.Now().Format("20060102150405")
	client := hub.NewClient(h.hub, conn, clientID, h.logger)
	
	// Register with hub
	h.hub.RegisterClient(client)
	
	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()
	
	// Send welcome message
	client.Send(&hub.Message{
		Type: "chat",
		Data: map[string]any{
			"message": "Welcome to Gortex WebSocket!",
		},
	})
	
	return nil
}

// APIHandler provides HTTP API to interact with WebSocket
type APIHandler struct {
	hub *hub.Hub
}

// Broadcast sends message to all connected clients
func (h *APIHandler) Broadcast(c gortexContext.Context) error {
	var req struct {
		Message string `json:"message"`
	}
	
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request"})
	}
	
	// Broadcast to all clients
	h.hub.Broadcast(&hub.Message{
		Type: "broadcast",
		Data: map[string]any{
			"message": req.Message,
		},
	})
	
	// Get current metrics for client count
	metrics := h.hub.GetMetrics()
	
	return c.JSON(200, map[string]interface{}{
		"success": true,
		"clients": metrics.CurrentConnections,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create WebSocket hub
	wsHub := hub.NewHub(logger)
	go wsHub.Run()

	// Create handlers
	handlers := &HandlersManager{
		Home:   &HomeHandler{},
		Status: &StatusHandler{hub: wsHub},
		WS: &WSHandler{
			hub:    wsHub,
			logger: logger,
			upgrader: websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true // Allow all origins for demo
				},
			},
		},
		API: &APIHandler{hub: wsHub},
	}

	// Configure application
	cfg := &app.Config{}
	cfg.Server.Address = ":8082"
	cfg.Logger.Level = "debug"

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Register shutdown hook for WebSocket cleanup
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Shutting down WebSocket hub...")
		return wsHub.ShutdownWithTimeout(5 * time.Second)
	})

	logger.Info("Starting Gortex WebSocket Example", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Routes:",
		zap.String("home", "GET / (WebSocket client)"),
		zap.String("websocket", "GET /ws (WebSocket endpoint)"),
		zap.String("status", "GET /status (Hub metrics)"),
		zap.String("broadcast", "POST /api/broadcast (Send to all)"),
	)

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}