package hub

import (
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB
)

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	UserID string
	hub    *Hub
	conn   *websocket.Conn
	send   chan *Message
	logger *zap.Logger
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID string, logger *zap.Logger) *Client {
	return &Client{
		ID:     uuid.New().String(),
		UserID: userID,
		hub:    hub,
		conn:   conn,
		send:   make(chan *Message, 256),
		logger: logger,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var message Message
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error",
					zap.String("client_id", c.ID),
					zap.Error(err))
			}
			break
		}

		// Add client info to message
		message.ClientID = c.ID

		// Process message based on type
		switch message.Type {
		case "ping":
			// Respond with pong
			pongMsg := &Message{
				Type: "pong",
				Data: map[string]any{
					"timestamp": time.Now().Unix(),
				},
			}
			select {
			case c.send <- pongMsg:
			default:
				c.logger.Warn("Failed to send pong", zap.String("client_id", c.ID))
			}

		case "broadcast":
			// Broadcast to all clients
			c.hub.broadcast <- &message

		case "private":
			// Send to specific user
			if target, ok := message.Data["target"].(string); ok {
				message.Target = target
				c.hub.broadcast <- &message
			}

		default:
			// Handle other message types or broadcast
			c.hub.broadcast <- &message
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.logger.Info("Channel closed, sending close message", zap.String("client_id", c.ID))
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutting down"))
				return
			}

			// Handle special close message
			if message.Type == "close" {
				c.logger.Info("Received close message", zap.String("client_id", c.ID))
				
				// Extract close code and reason
				code := websocket.CloseGoingAway
				reason := "Server shutting down"
				
				if codeVal, ok := message.Data["code"].(float64); ok {
					code = int(codeVal)
				}
				if reasonVal, ok := message.Data["reason"].(string); ok {
					reason = reasonVal
				}
				
				// Send proper WebSocket close message
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				c.logger.Error("WebSocket write error",
					zap.String("client_id", c.ID),
					zap.Error(err))
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send sends a message to this client
func (c *Client) Send(message *Message) bool {
	select {
	case c.send <- message:
		return true
	default:
		return false
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.conn.Close()
}