package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	// Test server defaults
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.True(t, cfg.Server.Recovery)

	// Test logger defaults
	assert.Equal(t, "info", cfg.Logger.Level)
	assert.Equal(t, "json", cfg.Logger.Encoding)

	// Test WebSocket defaults
	assert.Equal(t, 1024, cfg.WebSocket.ReadBufferSize)
	assert.Equal(t, int64(512*1024), cfg.WebSocket.MaxMessageSize)

	// Test JWT defaults
	assert.Equal(t, time.Hour, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, "gortex-server", cfg.JWT.Issuer)
}

func TestLoadFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"server": {
			"port": "8081",
			"address": ":8081"
		},
		"logger": {
			"level": "debug"
		},
		"jwt": {
			"secret_key": "json-secret"
		},
		"database": {
			"user": "json-user",
			"password": "json-pass"
		}
	}`)

	cfg := &config.Config{}
	err := config.LoadFromJSON(jsonData, cfg)
	require.NoError(t, err)

	assert.Equal(t, "8081", cfg.Server.Port)
	assert.Equal(t, ":8081", cfg.Server.Address)
	assert.Equal(t, "debug", cfg.Logger.Level)
}
