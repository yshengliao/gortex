package config_test

import (
	"strings"
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

func TestValidateRejectsShortJWTSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Database.User = "u"
	cfg.Database.Password = "p"

	// Empty secret is reported as "required".
	cfg.JWT.SecretKey = ""
	require.ErrorContains(t, cfg.Validate(), "required")

	// A non-empty but sub-32-byte secret is rejected for length — mirroring
	// auth.NewJWTService so the weakness surfaces at config time.
	cfg.JWT.SecretKey = strings.Repeat("a", 31)
	require.ErrorContains(t, cfg.Validate(), "at least 32 bytes")

	// Exactly 32 bytes is accepted.
	cfg.JWT.SecretKey = strings.Repeat("a", 32)
	require.NoError(t, cfg.Validate())
}
