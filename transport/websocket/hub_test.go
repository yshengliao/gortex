package websocket_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/transport/websocket"
)

func TestHub(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub in background
	go h.Run()
	
	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	t.Run("InitialState", func(t *testing.T) {
		assert.Equal(t, 0, h.GetConnectedClients())
	})

	t.Run("Broadcast", func(t *testing.T) {
		msg := &hub.Message{
			Type: "test",
			Data: map[string]interface{}{
				"content": "test message",
			},
		}
		
		// Should not panic even with no clients
		assert.NotPanics(t, func() {
			h.Broadcast(msg)
		})
	})

	t.Run("SendToUser", func(t *testing.T) {
		msg := &hub.Message{
			Type: "private",
			Data: map[string]interface{}{
				"content": "private message",
			},
		}
		
		// Should not panic even with no matching user
		assert.NotPanics(t, func() {
			h.SendToUser("user123", msg)
		})
		
		// Verify target is set
		assert.Equal(t, "user123", msg.Target)
	})

	t.Run("Shutdown", func(t *testing.T) {
		// Create a new hub for shutdown test
		h2 := hub.NewHub(logger)
		go h2.Run()
		time.Sleep(10 * time.Millisecond)
		
		// Shutdown should not panic
		assert.NotPanics(t, func() {
			h2.Shutdown()
		})
	})
}

// MockConn is a mock WebSocket connection for testing
type MockConn struct {
	writeMessages []interface{}
	readMessages  chan interface{}
	closed        bool
}

func NewMockConn() *MockConn {
	return &MockConn{
		writeMessages: []interface{}{},
		readMessages:  make(chan interface{}, 10),
	}
}

func (c *MockConn) WriteJSON(v interface{}) error {
	if c.closed {
		return nil
	}
	c.writeMessages = append(c.writeMessages, v)
	return nil
}

func (c *MockConn) ReadJSON(v interface{}) error {
	if c.closed {
		return nil
	}
	msg := <-c.readMessages
	// In a real implementation, you'd unmarshal msg into v
	_ = msg
	return nil
}

func (c *MockConn) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (c *MockConn) SetReadLimit(limit int64) {}
func (c *MockConn) SetReadDeadline(t time.Time) error { return nil }
func (c *MockConn) SetWriteDeadline(t time.Time) error { return nil }
func (c *MockConn) SetPongHandler(h func(string) error) {}
func (c *MockConn) Close() error {
	c.closed = true
	close(c.readMessages)
	return nil
}