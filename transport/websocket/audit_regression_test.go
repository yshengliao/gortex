package websocket

import (
	"testing"

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
