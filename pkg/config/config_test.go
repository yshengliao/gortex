package config_test

import (
	"os"
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

func TestSimpleLoader_LoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("GORTEX_SERVER_PORT", "9090")
	os.Setenv("GORTEX_SERVER_ADDRESS", ":9090")
	os.Setenv("GORTEX_SERVER_GZIP", "false")
	os.Setenv("GORTEX_SERVER_SHUTDOWN_TIMEOUT", "20s")
	os.Setenv("GORTEX_LOGGER_LEVEL", "debug")
	os.Setenv("GORTEX_WEBSOCKET_READ_BUFFER_SIZE", "2048")
	os.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret-key")
	os.Setenv("GORTEX_JWT_ISSUER", "test-issuer")
	os.Setenv("GORTEX_DATABASE_USER", "test-user")
	os.Setenv("GORTEX_DATABASE_PASSWORD", "test-password")

	defer func() {
		// Clean up
		os.Unsetenv("GORTEX_SERVER_PORT")
		os.Unsetenv("GORTEX_SERVER_ADDRESS")
		os.Unsetenv("GORTEX_SERVER_GZIP")
		os.Unsetenv("GORTEX_SERVER_SHUTDOWN_TIMEOUT")
		os.Unsetenv("GORTEX_LOGGER_LEVEL")
		os.Unsetenv("GORTEX_WEBSOCKET_READ_BUFFER_SIZE")
		os.Unsetenv("GORTEX_JWT_SECRET_KEY")
		os.Unsetenv("GORTEX_JWT_ISSUER")
		os.Unsetenv("GORTEX_DATABASE_USER")
		os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	}()

	loader := config.NewSimpleLoader()
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	// Verify environment variables were loaded
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, 20*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "debug", cfg.Logger.Level)
	assert.Equal(t, 2048, cfg.WebSocket.ReadBufferSize)
	assert.Equal(t, "test-secret-key", cfg.JWT.SecretKey)
	assert.Equal(t, "test-issuer", cfg.JWT.Issuer)
}

func TestSimpleLoader_WithYAML(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `
server:
  port: "8888"
  address: ":8888"
  gzip: false
  shutdown_timeout: 5s
logger:
  level: "warn"
websocket:
  read_buffer_size: 2048
jwt:
  secret_key: "yaml-secret"
  issuer: "yaml-issuer"
database:
  user: "yaml-user"
  password: "yaml-password"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load configuration
	loader := config.NewSimpleLoader().WithYAMLFile(tmpFile.Name())
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Verify YAML values were loaded
	assert.Equal(t, "8888", cfg.Server.Port)
	assert.Equal(t, ":8888", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, "warn", cfg.Logger.Level)
	assert.Equal(t, 2048, cfg.WebSocket.ReadBufferSize)
	assert.Equal(t, "yaml-secret", cfg.JWT.SecretKey)
	assert.Equal(t, "yaml-issuer", cfg.JWT.Issuer)
	assert.Equal(t, 5*time.Second, cfg.Server.ShutdownTimeout)
}

func TestSimpleLoader_EnvOverridesYAML(t *testing.T) {
	// Create YAML file
	yamlContent := `
server:
  port: "7777"
  shutdown_timeout: 30s
logger:
  level: "error"
jwt:
  secret_key: "test-key"
database:
  user: "test-user"
  password: "test-pass"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Set environment variable to override YAML
	os.Setenv("GORTEX_SERVER_PORT", "9999")
	os.Setenv("GORTEX_SERVER_SHUTDOWN_TIMEOUT", "15s")
	defer func() {
		os.Unsetenv("GORTEX_SERVER_PORT")
		os.Unsetenv("GORTEX_SERVER_SHUTDOWN_TIMEOUT")
	}()

	// Load configuration
	loader := config.NewSimpleLoader().WithYAMLFile(tmpFile.Name())
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Environment variable should override YAML
	assert.Equal(t, "9999", cfg.Server.Port)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
	// YAML value should be loaded for non-overridden fields
	assert.Equal(t, "error", cfg.Logger.Level)
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

func TestSimpleLoader_WithEnvPrefix(t *testing.T) {
	// Set environment variable with custom prefix
	os.Setenv("MYAPP_SERVER_PORT", "5555")
	os.Setenv("MYAPP_SERVER_SHUTDOWN_TIMEOUT", "25s")
	os.Setenv("MYAPP_JWT_SECRET_KEY", "myapp-secret")
	os.Setenv("MYAPP_DATABASE_USER", "myapp-user")
	os.Setenv("MYAPP_DATABASE_PASSWORD", "myapp-pass")
	defer func() {
		os.Unsetenv("MYAPP_SERVER_PORT")
		os.Unsetenv("MYAPP_SERVER_SHUTDOWN_TIMEOUT")
		os.Unsetenv("MYAPP_JWT_SECRET_KEY")
		os.Unsetenv("MYAPP_DATABASE_USER")
		os.Unsetenv("MYAPP_DATABASE_PASSWORD")
	}()

	loader := config.NewSimpleLoader().WithEnvPrefix("MYAPP_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "5555", cfg.Server.Port)
	assert.Equal(t, 25*time.Second, cfg.Server.ShutdownTimeout)
}
