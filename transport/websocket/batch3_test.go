package websocket

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// --- Fix 3: hub shutdown no longer freezes the event loop ---------------------

// TestHub_ShutdownWithTimeout_RunningHubEmpty is the regression test for the
// 500ms sleep that froze Run()'s event loop during shutdown. On a running but
// empty hub, ShutdownWithTimeout with a short timeout used to always time out
// (the loop slept 500ms while the timeout was shorter). It must now complete
// well within the timeout because an empty hub skips the grace entirely.
func TestHub_ShutdownWithTimeout_RunningHubEmpty(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	go hub.Run()

	// Let the loop start servicing.
	require.Equal(t, 0, hub.GetConnectedClients())

	start := time.Now()
	err := hub.ShutdownWithTimeout(200 * time.Millisecond)
	elapsed := time.Since(start)

	require.NoError(t, err, "empty running hub must shut down within the timeout")
	assert.Less(t, elapsed, 200*time.Millisecond, "empty hub shutdown must be near-instant, not blocked on a grace sleep")
}

// TestHub_GetMetricsDuringShutdownGrace verifies the event loop keeps serving
// requests while clients receive their close frames: with a client connected
// (so the grace window runs), GetMetrics and RegisterClient issued during
// shutdown must be answered rather than hang.
func TestHub_GetMetricsDuringShutdownGrace(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	go hub.Run()

	// A client with an unbuffered (cap-1, then filled) send channel keeps the
	// grace window busy. We use a buffered channel and register it so the hub
	// has a client and therefore runs the grace loop on shutdown.
	client := &Client{ID: "c1", UserID: "u1", send: make(chan *Message, 256)}
	hub.RegisterClient(client)

	// Trigger shutdown asynchronously so we can poke the hub mid-grace.
	shutdownDone := make(chan struct{})
	go func() {
		hub.Shutdown()
		close(shutdownDone)
	}()

	// These calls must return (be serviced or fall through the shutdown escape)
	// rather than block for the whole grace window.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = hub.GetMetrics()
		_ = hub.GetConnectedClients()
		// RegisterClient during shutdown must not hang; it is refused/acked
		// or returns via the shutdown escape.
		hub.RegisterClient(&Client{ID: "c2", UserID: "u2", send: make(chan *Message, 1)})
	}()

	select {
	case <-done:
		// Good: concurrent callers were answered during the grace window.
	case <-time.After(2 * time.Second):
		t.Fatal("hub calls during shutdown grace hung — event loop is frozen")
	}

	<-shutdownDone
}

// TestHub_ShutdownDeliversCloseToClient verifies the grace window preserves the
// existing semantic that a connected client receives the close frame before its
// send channel is closed.
func TestHub_ShutdownDeliversCloseToClient(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	go hub.Run()

	client := &Client{ID: "c1", UserID: "u1", send: make(chan *Message, 256)}
	hub.RegisterClient(client)

	// Drain the welcome message first.
	select {
	case <-client.send:
	case <-time.After(time.Second):
		t.Fatal("did not receive welcome message")
	}

	go hub.Shutdown()

	// The next message must be the graceful close, delivered before the
	// channel is closed.
	select {
	case msg, ok := <-client.send:
		require.True(t, ok, "close frame must arrive before the channel is closed")
		assert.Equal(t, "close", msg.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("client never received the close frame")
	}
}

// --- Fix 4: private target resolved before authorization ----------------------

// recordingAuthorizer captures the last message it was asked to authorize.
type recordingAuthorizer struct {
	last *Message
	veto error
}

func (r *recordingAuthorizer) authorize(_ *Client, m *Message) error {
	// Copy so later mutation by the caller cannot change what we recorded.
	cp := *m
	r.last = &cp
	return r.veto
}

// TestResolveTarget_PopulatesPrivateTarget is a focused unit test for the helper
// that now runs before authorization.
func TestResolveTarget_PopulatesPrivateTarget(t *testing.T) {
	msg := &Message{Type: "private", Data: map[string]any{"target": "user-42"}}
	resolveTarget(msg)
	assert.Equal(t, "user-42", msg.Target)

	// Non-private messages are untouched.
	other := &Message{Type: "chat", Data: map[string]any{"target": "user-42"}}
	resolveTarget(other)
	assert.Empty(t, other.Target, "only private messages resolve a target")

	// Private without a string target leaves Target empty.
	noTarget := &Message{Type: "private", Data: map[string]any{"target": 123}}
	resolveTarget(noTarget)
	assert.Empty(t, noTarget.Target)
}

// TestAuthorizer_SeesResolvedPrivateTarget is the regression test for the
// ordering bug: the Authorizer used to be consulted before Data["target"] was
// copied into Target, so it could never vet the real recipient. With the fix,
// the recorded message the Authorizer saw must already carry Target.
func TestAuthorizer_SeesResolvedPrivateTarget(t *testing.T) {
	rec := &recordingAuthorizer{}
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{
		AllowedMessageTypes: []string{"private"},
		Authorizer:          rec.authorize,
	})

	// Mirror the ReadPump ordering: resolve target, then authorize.
	msg := &Message{Type: "private", Data: map[string]any{"target": "victim"}}
	resolveTarget(msg)
	require.NoError(t, hub.checkInbound(&Client{ID: "attacker"}, msg))

	require.NotNil(t, rec.last, "authorizer must have been consulted")
	assert.Equal(t, "victim", rec.last.Target, "authorizer must see the resolved private target")
}

// TestAuthorizer_VetoesPrivateToThirdParty verifies an Authorizer that only
// allows clients to message themselves blocks a private message aimed at a
// third party — proving private targeting can now be enforced.
func TestAuthorizer_VetoesPrivateToThirdParty(t *testing.T) {
	denied := errors.New("cannot target other users")
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{
		AllowedMessageTypes: []string{"private"},
		Authorizer: func(c *Client, m *Message) error {
			if m.Type == "private" && m.Target != c.UserID {
				return denied
			}
			return nil
		},
	})

	attacker := &Client{ID: "a1", UserID: "attacker"}

	// Targeting a third party is rejected.
	toThirdParty := &Message{Type: "private", Data: map[string]any{"target": "victim"}}
	resolveTarget(toThirdParty)
	require.ErrorIs(t, hub.checkInbound(attacker, toThirdParty), denied)

	// Targeting self is allowed.
	toSelf := &Message{Type: "private", Data: map[string]any{"target": "attacker"}}
	resolveTarget(toSelf)
	require.NoError(t, hub.checkInbound(attacker, toSelf))
}

// --- Fix 5: dropped-broadcast and forced-disconnect counters ------------------

// TestHubMetrics_DroppedBroadcasts verifies the dropped-broadcast counter is
// surfaced via GetMetrics after the broadcast channel is saturated. The hub is
// not started, so nothing drains the 256-deep buffer; the 257th Broadcast (and
// beyond) is dropped and counted.
func TestHubMetrics_DroppedBroadcasts(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	// Do NOT start Run(): the broadcast buffer never drains.

	for i := 0; i < cap(hub.broadcast)+10; i++ {
		hub.Broadcast(&Message{Type: "chat"})
	}

	// Read the counter directly (GetMetrics needs the running loop). The
	// counter is the value GetMetrics exposes via snapshotMetrics.
	assert.GreaterOrEqual(t, hub.droppedBroadcasts.Load(), int64(1), "saturating the broadcast channel must increment the dropped counter")
	assert.Equal(t, hub.droppedBroadcasts.Load(), hub.snapshotMetrics().DroppedBroadcasts, "snapshotMetrics must expose the dropped count")
}

// TestHubMetrics_ForcedDisconnects verifies the forced-disconnect counter is
// incremented (and exposed) when a targeted message cannot be delivered because
// the recipient's send buffer is full.
func TestHubMetrics_ForcedDisconnects(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	go hub.Run()
	defer hub.Shutdown()

	// A client whose send buffer has capacity 1 and is already full forces the
	// hub to evict it on the next delivery attempt.
	client := &Client{ID: "slow", UserID: "slow-user", send: make(chan *Message, 1)}
	hub.RegisterClient(client)
	// Fill the buffer: the welcome message already occupies the single slot, so
	// the buffer is full. Any targeted delivery now hits the default branch.

	// broadcastSync waits for the hub to process the message, so the counter is
	// settled before we read it.
	hub.broadcastSync(&Message{Type: "private", Target: "slow-user", Data: map[string]any{"x": 1}})

	metrics := hub.GetMetrics()
	assert.GreaterOrEqual(t, metrics.ForcedDisconnects, int64(1), "a full-buffer delivery must count a forced disconnect")
}
