// Package hub provides WebSocket connection management
package websocket

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// DefaultMaxMessageBytes is the default per-message read cap applied by the
// hub unless Config.MaxMessageBytes overrides it. Matches the size most
// chat-style JSON payloads comfortably fit into whilst keeping the
// per-connection memory footprint bounded against oversize-message DoS.
const DefaultMaxMessageBytes int64 = 64 << 10 // 64 KiB

// MessageAuthorizer decides whether a client is allowed to publish a
// particular message. Returning a non-nil error causes the hub to drop the
// message and log a warning; returning nil lets the message flow as normal.
type MessageAuthorizer func(client *Client, msg *Message) error

// ErrMessageUnauthorized is the canonical error to return from a
// MessageAuthorizer when a client's message should be rejected.
var ErrMessageUnauthorized = errors.New("websocket: message unauthorized")

// Config tunes the hub's runtime behaviour. The zero value is valid and
// applies the defaults.
type Config struct {
	// MaxMessageBytes caps the size of any single WebSocket frame read
	// from a client. Values <= 0 fall back to DefaultMaxMessageBytes.
	MaxMessageBytes int64

	// AllowedMessageTypes, when non-empty, gates inbound messages to the
	// listed Type values. Unknown types are dropped and logged.
	AllowedMessageTypes []string

	// Authorizer, when non-nil, is consulted for every inbound message.
	// Returning an error drops the message.
	Authorizer MessageAuthorizer
}

// allowedTypeSet returns a lookup set for the configured whitelist, or nil
// if every type should be allowed.
func (c *Config) allowedTypeSet() map[string]struct{} {
	if c == nil || len(c.AllowedMessageTypes) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(c.AllowedMessageTypes))
	for _, t := range c.AllowedMessageTypes {
		set[t] = struct{}{}
	}
	return set
}

// maxMessageBytes resolves the effective per-frame cap.
func (c *Config) maxMessageBytes() int64 {
	if c == nil || c.MaxMessageBytes <= 0 {
		return DefaultMaxMessageBytes
	}
	return c.MaxMessageBytes
}

// Message represents a WebSocket message
type Message struct {
	Type     string                 `json:"type"`
	Data     map[string]any `json:"data,omitempty"`
	Target   string                 `json:"target,omitempty"`   // For targeted messages
	ClientID string                 `json:"client_id,omitempty"` // Sender's client ID
}

// clientRequest represents a request to get client information
type clientRequest struct {
	response chan int
}

// metricsRequest represents a request to get hub metrics
type metricsRequest struct {
	response chan *Metrics
}

// registerRequest carries a client to be registered plus an ack channel that
// is closed once the hub has recorded the registration. Acking removes the
// need for callers (and tests) to sleep-and-pray after RegisterClient.
type registerRequest struct {
	client *Client
	done   chan struct{}
}

// unregisterRequest mirrors registerRequest for removals.
type unregisterRequest struct {
	client *Client
	done   chan struct{}
}

// broadcastOp is what flows through the hub's broadcast channel. The done
// field is optional: production callers leave it nil and the hub fires and
// forgets, whilst tests can supply an ack channel to wait for the hub to
// finish processing the message before checking metrics.
type broadcastOp struct {
	msg  *Message
	done chan struct{}
}

// Metrics contains WebSocket hub metrics for development monitoring
type Metrics struct {
	CurrentConnections int                    `json:"current_connections"`
	TotalConnections   int64                  `json:"total_connections"`
	MessagesSent       int64                  `json:"messages_sent"`
	MessagesReceived   int64                  `json:"messages_received"`
	MessageTypes       map[string]int64       `json:"message_types"`
	LastMessageTime    time.Time              `json:"last_message_time"`
	Uptime             time.Duration          `json:"uptime"`
}

// messageRateTracker tracks message rates over time
type messageRateTracker struct {
	sent     int64
	received int64
	lastTime time.Time
}

// Hub maintains active WebSocket connections
type Hub struct {
	clients      map[*Client]bool
	broadcast    chan broadcastOp
	register     chan registerRequest
	unregister   chan unregisterRequest
	clientCount  chan clientRequest
	metricsReq   chan metricsRequest
	logger       *zap.Logger
	shutdown     chan struct{}
	shutdownDone chan struct{}

	config           Config
	allowedTypesCache map[string]struct{}
	
	// Metrics fields
	totalConnections atomic.Int64
	messagesSent     atomic.Int64
	messagesReceived atomic.Int64
	messageTypes     map[string]int64
	lastMessageTime  time.Time
	startTime        time.Time
}

// NewHub creates a new WebSocket hub with default configuration.
func NewHub(logger *zap.Logger) *Hub {
	return NewHubWithConfig(logger, Config{})
}

// NewHubWithConfig creates a hub with the supplied configuration. Zero-value
// fields fall back to defaults (see DefaultMaxMessageBytes).
func NewHubWithConfig(logger *zap.Logger, cfg Config) *Hub {
	return &Hub{
		clients:           make(map[*Client]bool),
		broadcast:         make(chan broadcastOp, 256),
		register:          make(chan registerRequest),
		unregister:        make(chan unregisterRequest),
		clientCount:       make(chan clientRequest),
		metricsReq:        make(chan metricsRequest),
		logger:            logger,
		shutdown:          make(chan struct{}),
		shutdownDone:      make(chan struct{}),
		messageTypes:      make(map[string]int64),
		startTime:         time.Now(),
		config:            cfg,
		allowedTypesCache: cfg.allowedTypeSet(),
	}
}

// Run starts the hub's main loop - all state mutations happen here
func (h *Hub) Run() {
	defer close(h.shutdownDone)
	
	for {
		select {
		case req := <-h.register:
			h.registerClient(req.client)
			close(req.done)

		case req := <-h.unregister:
			h.unregisterClient(req.client)
			close(req.done)

		case op := <-h.broadcast:
			h.broadcastMessage(op.msg)
			if op.done != nil {
				close(op.done)
			}
			
		case req := <-h.clientCount:
			req.response <- len(h.clients)
			
		case req := <-h.metricsReq:
			metrics := &Metrics{
				CurrentConnections: len(h.clients),
				TotalConnections:   h.totalConnections.Load(),
				MessagesSent:       h.messagesSent.Load(),
				MessagesReceived:   h.messagesReceived.Load(),
				MessageTypes:       make(map[string]int64),
				LastMessageTime:    h.lastMessageTime,
				Uptime:             time.Since(h.startTime),
			}
			// Copy message types to avoid race conditions
			for k, v := range h.messageTypes {
				metrics.MessageTypes[k] = v
			}
			req.response <- metrics

		case <-h.shutdown:
			// Send graceful close message to all clients
			h.logger.Info("Closing all client connections", zap.Int("count", len(h.clients)))
			
			closeMsg := &Message{
				Type: "close",
				Data: map[string]any{
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
	h.totalConnections.Add(1)

	h.logger.Info("Client connected",
		zap.String("client_id", client.ID),
		zap.String("user_id", client.UserID))

	// Send welcome message
	welcomeMsg := &Message{
		Type: "welcome",
		Data: map[string]any{
			"client_id": client.ID,
			"message":   "Connected to server",
		},
	}
	
	select {
	case client.send <- welcomeMsg:
		h.messagesSent.Add(1)
		h.messageTypes["welcome"]++
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
	// Track message metrics
	h.messagesReceived.Add(1)
	h.lastMessageTime = time.Now()
	if message.Type != "" {
		h.messageTypes[message.Type]++
	}
	
	if message.Target != "" {
		// Targeted message
		for client := range h.clients {
			if client.ID == message.Target || client.UserID == message.Target {
				select {
				case client.send <- message:
					h.messagesSent.Add(1)
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
				h.messagesSent.Add(1)
			default:
				h.logger.Warn("Client send channel full, closing",
					zap.String("client_id", client.ID))
				go h.removeClient(client)
			}
		}
	}
}

// removeClient safely removes a client. It is fire-and-forget: callers do
// not block on the hub acknowledging the removal, so it is safe to invoke
// from inside the hub's own goroutine (see broadcastMessage).
func (h *Hub) removeClient(client *Client) {
	req := unregisterRequest{client: client, done: make(chan struct{})}
	select {
	case h.unregister <- req:
	case <-h.shutdown:
	}
}

// checkInbound validates an inbound client message against the configured
// type whitelist and authorizer. It returns an error describing why the
// message should be dropped, or nil if the message is allowed.
func (h *Hub) checkInbound(client *Client, msg *Message) error {
	if h.allowedTypesCache != nil {
		if _, ok := h.allowedTypesCache[msg.Type]; !ok {
			return fmt.Errorf("websocket: message type %q not allowed", msg.Type)
		}
	}
	if h.config.Authorizer != nil {
		if err := h.config.Authorizer(client, msg); err != nil {
			return err
		}
	}
	return nil
}

// maxMessageBytes returns the effective per-frame read cap for the hub's
// connected clients.
func (h *Hub) maxMessageBytes() int64 {
	return h.config.maxMessageBytes()
}

// Broadcast sends a message to all connected clients. The call is
// fire-and-forget: if the broadcast channel is full the message is dropped
// and a warning is logged — slow consumers must never block producers.
func (h *Hub) Broadcast(message *Message) {
	select {
	case h.broadcast <- broadcastOp{msg: message}:
	default:
		h.logger.Warn("Broadcast channel full")
	}
}

// broadcastSync sends a message and blocks until the hub has processed it.
// Intended for tests that need to observe the post-broadcast metrics
// state; production code should use Broadcast instead.
func (h *Hub) broadcastSync(message *Message) {
	op := broadcastOp{msg: message, done: make(chan struct{})}
	select {
	case h.broadcast <- op:
	case <-h.shutdown:
		return
	}
	select {
	case <-op.done:
	case <-h.shutdown:
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

// RegisterClient registers a new client to the hub. It blocks until the hub
// has recorded the client (and sent its welcome message) or until the hub is
// shut down, so tests can observe the post-registration state without
// time-based waits.
func (h *Hub) RegisterClient(client *Client) {
	req := registerRequest{client: client, done: make(chan struct{})}
	select {
	case h.register <- req:
	case <-h.shutdown:
		return
	}
	select {
	case <-req.done:
	case <-h.shutdown:
	}
}

// UnregisterClient removes a client from the hub and blocks until the hub
// has processed the removal. Use this from tests and shutdown paths; the
// internal removeClient helper stays fire-and-forget for use inside the hub
// goroutine.
func (h *Hub) UnregisterClient(client *Client) {
	req := unregisterRequest{client: client, done: make(chan struct{})}
	select {
	case h.unregister <- req:
	case <-h.shutdown:
		return
	}
	select {
	case <-req.done:
	case <-h.shutdown:
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
		Data: map[string]any{
			"message": "Server is shutting down",
			"time":    time.Now().Unix(),
		},
	}
	
	// Try to broadcast shutdown message
	select {
	case h.broadcast <- broadcastOp{msg: shutdownMsg}:
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

// GetMetrics returns current hub metrics for development monitoring
func (h *Hub) GetMetrics() *Metrics {
	req := metricsRequest{
		response: make(chan *Metrics),
	}
	
	select {
	case h.metricsReq <- req:
		return <-req.response
	case <-h.shutdown:
		// Hub is shutting down, return empty metrics
		return &Metrics{
			Uptime: time.Since(h.startTime),
		}
	}
}

// GetMessageRate calculates messages per second over the last minute
func (h *Hub) GetMessageRate() (sent float64, received float64) {
	metrics := h.GetMetrics()
	if metrics.Uptime.Seconds() > 0 {
		sent = float64(metrics.MessagesSent) / metrics.Uptime.Seconds()
		received = float64(metrics.MessagesReceived) / metrics.Uptime.Seconds()
	}
	return
}