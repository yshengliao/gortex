// Package hub provides WebSocket connection management
package hub

import (
	"fmt"
	"time"
	
	"go.uber.org/zap"
)

// Message represents a WebSocket message
type Message struct {
	Type     string                 `json:"type"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Target   string                 `json:"target,omitempty"`   // For targeted messages
	ClientID string                 `json:"client_id,omitempty"` // Sender's client ID
}

// clientRequest represents a request to get client information
type clientRequest struct {
	response chan int
}

// Hub maintains active WebSocket connections
type Hub struct {
	clients      map[*Client]bool
	broadcast    chan *Message
	register     chan *Client    // register is now unexported
	unregister   chan *Client
	clientCount  chan clientRequest
	logger       *zap.Logger
	shutdown     chan struct{}
	shutdownDone chan struct{}
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		broadcast:    make(chan *Message, 256),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		clientCount:  make(chan clientRequest),
		logger:       logger,
		shutdown:     make(chan struct{}),
		shutdownDone: make(chan struct{}),
	}
}

// Run starts the hub's main loop - all state mutations happen here
func (h *Hub) Run() {
	defer close(h.shutdownDone)
	
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
			
		case req := <-h.clientCount:
			req.response <- len(h.clients)

		case <-h.shutdown:
			// Send graceful close message to all clients
			h.logger.Info("Closing all client connections", zap.Int("count", len(h.clients)))
			
			closeMsg := &Message{
				Type: "close",
				Data: map[string]interface{}{
					"code":    1001, // Going Away
					"reason":  "Server is shutting down",
					"message": "Please reconnect later",
				},
			}
			
			// Send close message to all clients
			for client := range h.clients {
				select {
				case client.send <- closeMsg:
					// Give client time to process the close message
				default:
					// Channel is full, client will be forcefully closed
				}
			}
			
			// Give clients a moment to receive close messages
			time.Sleep(500 * time.Millisecond)
			
			// Now close all client connections
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			
			h.logger.Info("Hub shutdown complete")
			return
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.clients[client] = true

	h.logger.Info("Client connected",
		zap.String("client_id", client.ID),
		zap.String("user_id", client.UserID))

	// Send welcome message
	welcomeMsg := &Message{
		Type: "welcome",
		Data: map[string]interface{}{
			"client_id": client.ID,
			"message":   "Connected to server",
		},
	}
	
	select {
	case client.send <- welcomeMsg:
	default:
		h.logger.Warn("Failed to send welcome message", zap.String("client_id", client.ID))
	}
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		h.logger.Info("Client disconnected",
			zap.String("client_id", client.ID),
			zap.String("user_id", client.UserID))
	}
}

// broadcastMessage sends a message to all or specific clients
func (h *Hub) broadcastMessage(message *Message) {
	if message.Target != "" {
		// Targeted message
		for client := range h.clients {
			if client.ID == message.Target || client.UserID == message.Target {
				select {
				case client.send <- message:
				default:
					h.logger.Warn("Client send channel full, closing",
						zap.String("client_id", client.ID))
					go h.removeClient(client)
				}
			}
		}
	} else {
		// Broadcast to all clients
		for client := range h.clients {
			select {
			case client.send <- message:
			default:
				h.logger.Warn("Client send channel full, closing",
					zap.String("client_id", client.ID))
				go h.removeClient(client)
			}
		}
	}
}

// removeClient safely removes a client
func (h *Hub) removeClient(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.shutdown:
		// Hub is shutting down, ignore
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message *Message) {
	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("Broadcast channel full")
	}
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID string, message *Message) {
	message.Target = userID
	h.Broadcast(message)
}

// GetConnectedClients returns the number of connected clients
func (h *Hub) GetConnectedClients() int {
	req := clientRequest{
		response: make(chan int),
	}
	
	select {
	case h.clientCount <- req:
		return <-req.response
	case <-h.shutdown:
		return 0
	}
}

// RegisterClient registers a new client to the hub
func (h *Hub) RegisterClient(client *Client) {
	select {
	case h.register <- client:
	default:
		h.logger.Warn("Register channel full")
	}
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	h.logger.Info("Hub shutdown initiated")
	close(h.shutdown)
	<-h.shutdownDone
}

// ShutdownWithTimeout gracefully shuts down the hub with a timeout
func (h *Hub) ShutdownWithTimeout(timeout time.Duration) error {
	h.logger.Info("Hub shutdown initiated", zap.Duration("timeout", timeout))
	
	// Send shutdown notification to all clients
	shutdownMsg := &Message{
		Type: "server_shutdown",
		Data: map[string]interface{}{
			"message": "Server is shutting down",
			"time":    time.Now().Unix(),
		},
	}
	
	// Try to broadcast shutdown message
	select {
	case h.broadcast <- shutdownMsg:
		// Give clients a moment to receive the message
		time.Sleep(100 * time.Millisecond)
	default:
		h.logger.Warn("Could not broadcast shutdown message")
	}
	
	// Start shutdown
	close(h.shutdown)
	
	// Wait for shutdown with timeout
	select {
	case <-h.shutdownDone:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("hub shutdown timed out after %v", timeout)
	}
}