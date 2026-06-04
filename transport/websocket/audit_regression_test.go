package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_SendAfterChannelClosed_NoPanic is a regression test for the
// "send on closed channel" race: the hub owns closing client.send (on
// unregister/shutdown), so a producer — Client.Send or the ReadPump pong path
// — can race with that close. Sending on a closed channel panics even inside a
// select, so the producer must turn it into a dropped message, not a crash.
func TestClient_SendAfterChannelClosed_NoPanic(t *testing.T) {
	c := &Client{send: make(chan *Message, 1)}
	close(c.send)

	var ok bool
	require.NotPanics(t, func() {
		ok = c.Send(&Message{Type: "chat"})
	})
	assert.False(t, ok, "send on a closed channel must report failure, not panic")
}

// TestClient_ForwardToHub_ReturnsOnShutdown is a regression test for the
// ReadPump goroutine leak: the broadcast send used to be a bare blocking send
// with no shutdown escape. h.broadcast is buffered and Hub.Run stops draining it
// after shutdown, so a ReadPump reaching the send at/after shutdown would block
// forever (closing the conn cannot wake a goroutine parked on a channel send).
func TestClient_ForwardToHub_ReturnsOnShutdown(t *testing.T) {
	// An unbuffered, undrained broadcast channel plus a closed shutdown channel
	// mimics a hub whose Run loop has already returned.
	hub := &Hub{
		broadcast: make(chan broadcastOp),
		shutdown:  make(chan struct{}),
	}
	close(hub.shutdown)
	c := &Client{hub: hub}

	done := make(chan bool, 1)
	go func() { done <- c.forwardToHub(&Message{Type: "chat"}) }()

	select {
	case ok := <-done:
		assert.False(t, ok, "forwardToHub must report shutdown rather than block")
	case <-time.After(2 * time.Second):
		t.Fatal("forwardToHub blocked forever after hub shutdown (goroutine leak)")
	}
}

// TestClient_ForwardToHub_DeliversWhenDrained verifies the happy path still
// hands the message to the hub when the broadcast channel has capacity.
func TestClient_ForwardToHub_DeliversWhenDrained(t *testing.T) {
	hub := &Hub{
		broadcast: make(chan broadcastOp, 1),
		shutdown:  make(chan struct{}),
	}
	c := &Client{hub: hub}

	require.True(t, c.forwardToHub(&Message{Type: "chat"}), "should deliver when broadcast has capacity")
	op := <-hub.broadcast
	assert.Equal(t, "chat", op.msg.Type)
}
