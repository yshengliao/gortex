package websocket

import (
	"sync"
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
)

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	UserID string
	hub    *Hub
	conn   *websocket.Conn
	send   chan *Message
	logger *zap.Logger

	closeOnce sync.Once // guards conn.Close() against the Read/Write pumps racing to close
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

// closeConn closes the underlying connection exactly once. Both ReadPump and
// WritePump defer it, and Close() calls it too; sync.Once makes the extra
// calls harmless instead of relying on the driver tolerating a double close.
func (c *Client) closeConn() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
	})
}

// trySend performs a non-blocking send on the client's send channel. The hub
// owns closing c.send (on unregister/shutdown), so a producer here can race
// with that close; sending on a closed channel panics even inside a select,
// so the recover turns that race into a dropped message rather than a crash.
func (c *Client) trySend(m *Message) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	select {
	case c.send <- m:
		return true
	default:
		return false
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.removeClient(c)
		c.closeConn()
	}()

	c.conn.SetReadLimit(c.hub.maxMessageBytes())
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
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

		// Ping is always allowed — it's internal to the keepalive and
		// doesn't need to pass the inbound gate.
		if message.Type == "ping" {
			pongMsg := &Message{
				Type: "pong",
				Data: map[string]any{
					"timestamp": time.Now().Unix(),
				},
			}
			if !c.trySend(pongMsg) {
				c.logger.Warn("Failed to send pong", zap.String("client_id", c.ID))
			}
			continue
		}

		if err := c.hub.checkInbound(c, &message); err != nil {
			c.logger.Warn("Dropping inbound WebSocket message",
				zap.String("client_id", c.ID),
				zap.String("type", message.Type),
				zap.Error(err))
			continue
		}

		switch message.Type {
		case "private":
			if target, ok := message.Data["target"].(string); ok {
				message.Target = target
			}
			c.hub.broadcast <- broadcastOp{msg: &message}
		default:
			c.hub.broadcast <- broadcastOp{msg: &message}
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.closeConn()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.logger.Info("Channel closed, sending close message", zap.String("client_id", c.ID))
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutting down"))
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
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				c.logger.Error("WebSocket write error",
					zap.String("client_id", c.ID),
					zap.Error(err))
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send sends a message to this client. It is non-blocking and safe to call
// even if the hub has already closed the client's send channel — in that case
// it simply returns false instead of panicking.
func (c *Client) Send(message *Message) bool {
	return c.trySend(message)
}

// Close closes the client connection
func (c *Client) Close() {
	c.closeConn()
}
