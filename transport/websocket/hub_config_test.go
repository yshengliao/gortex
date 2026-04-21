package websocket

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestConfigMaxMessageBytesDefault(t *testing.T) {
	var cfg Config
	assert.Equal(t, DefaultMaxMessageBytes, cfg.maxMessageBytes())

	cfg.MaxMessageBytes = -1
	assert.Equal(t, DefaultMaxMessageBytes, cfg.maxMessageBytes())

	cfg.MaxMessageBytes = 128
	assert.Equal(t, int64(128), cfg.maxMessageBytes())
}

func TestHubCheckInboundAllowsWhenNoGates(t *testing.T) {
	hub := NewHub(zaptest.NewLogger(t))
	err := hub.checkInbound(nil, &Message{Type: "chat"})
	require.NoError(t, err)
}

func TestHubCheckInboundRejectsUnlistedType(t *testing.T) {
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{
		AllowedMessageTypes: []string{"chat", "private"},
	})
	err := hub.checkInbound(nil, &Message{Type: "evil"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"evil"`)

	// Listed type still passes.
	require.NoError(t, hub.checkInbound(nil, &Message{Type: "chat"}))
}

func TestHubCheckInboundInvokesAuthorizer(t *testing.T) {
	sentinel := errors.New("nope")
	called := 0
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{
		Authorizer: func(c *Client, m *Message) error {
			called++
			if m.Type == "forbidden" {
				return sentinel
			}
			return nil
		},
	})

	require.NoError(t, hub.checkInbound(nil, &Message{Type: "ok"}))
	require.ErrorIs(t, hub.checkInbound(nil, &Message{Type: "forbidden"}), sentinel)
	assert.Equal(t, 2, called)
}

func TestHubCheckInboundWhitelistBeforeAuthorizer(t *testing.T) {
	authorizerCalled := false
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{
		AllowedMessageTypes: []string{"chat"},
		Authorizer: func(c *Client, m *Message) error {
			authorizerCalled = true
			return nil
		},
	})
	err := hub.checkInbound(nil, &Message{Type: "other"})
	require.Error(t, err)
	assert.False(t, authorizerCalled, "authorizer should not run for whitelist-rejected messages")
}

func TestHubMaxMessageBytesFollowsConfig(t *testing.T) {
	hub := NewHubWithConfig(zaptest.NewLogger(t), Config{MaxMessageBytes: 2048})
	assert.Equal(t, int64(2048), hub.maxMessageBytes())

	defaultHub := NewHub(zaptest.NewLogger(t))
	assert.Equal(t, DefaultMaxMessageBytes, defaultHub.maxMessageBytes())
}
