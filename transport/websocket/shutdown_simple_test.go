package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestHubShutdownBasic(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(logger)
	
	// Start hub
	go hub.Run()
	time.Sleep(50 * time.Millisecond)
	
	// Verify hub is running
	assert.Equal(t, 0, hub.GetConnectedClients())
	
	// Shutdown hub
	err := hub.ShutdownWithTimeout(1 * time.Second)
	assert.NoError(t, err)
}

func TestHubShutdownTimeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(logger)
	
	// Don't start the hub - this simulates a stuck hub
	start := time.Now()
	err := hub.ShutdownWithTimeout(100 * time.Millisecond)
	duration := time.Since(start)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, duration, 300*time.Millisecond)
}