package websocket

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// TestHub_DoubleShutdown verifies that calling Shutdown twice does not panic.
func TestHub_DoubleShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(logger)

	go hub.Run()
	time.Sleep(20 * time.Millisecond)

	// First shutdown — should succeed.
	hub.Shutdown()

	// Second shutdown — must NOT panic (close on closed channel).
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("second Shutdown() panicked: %v", r)
		}
	}()
	hub.Shutdown()
}

// TestHub_DoubleShutdownWithTimeout verifies that ShutdownWithTimeout called
// twice does not panic.
func TestHub_DoubleShutdownWithTimeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(logger)

	go hub.Run()
	time.Sleep(20 * time.Millisecond)

	err := hub.ShutdownWithTimeout(1 * time.Second)
	if err != nil {
		t.Fatalf("first ShutdownWithTimeout failed: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("second ShutdownWithTimeout() panicked: %v", r)
		}
	}()
	// Second call — the hub is already shut down; should not panic.
	_ = hub.ShutdownWithTimeout(500 * time.Millisecond)
}
