package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/config"
)

// TestBofryLoader_LoadDefaults tests that the BofryLoader correctly loads default configuration
func TestBofryLoader_LoadDefaults(t *testing.T) {
	// Set required fields via environment to pass validation
	os.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	os.Setenv("GORTEX_DATABASE_USER", "test-user")
	os.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")
	defer func() {
		os.Unsetenv("GORTEX_JWT_SECRET_KEY")
		os.Unsetenv("GORTEX_DATABASE_USER")
		os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	}()
	
	loader := config.NewBofryLoader().WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	
	err := loader.Load(cfg)
	require.NoError(t, err)
	
	// Verify defaults are loaded
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.True(t, cfg.Server.Recovery)
	assert.Equal(t, "info", cfg.Logger.Level)
	assert.Equal(t, "json", cfg.Logger.Encoding)
}

// TestBofryLoader_LoadFromYAML tests loading configuration from YAML file
func TestBofryLoader_LoadFromYAML(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `
server:
  port: "8888"
  address: ":8888"
  gzip: false
  read_timeout: 45s
logger:
  level: "warn"
  encoding: "console"
websocket:
  read_buffer_size: 2048
  write_buffer_size: 2048
  max_message_size: 1048576
jwt:
  secret_key: "yaml-secret-key"
  issuer: "yaml-issuer"
  access_token_ttl: 2h
database:
  host: "localhost"
  port: 5432
  user: "yaml-user"
  password: "yaml-password"
  database: "testdb"
`
	tmpFile, err := os.CreateTemp("", "bofry-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load configuration
	loader := config.NewBofryLoader().WithYAMLFile(tmpFile.Name())
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Verify YAML values were loaded
	assert.Equal(t, "8888", cfg.Server.Port)
	assert.Equal(t, ":8888", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, 45*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "warn", cfg.Logger.Level)
	assert.Equal(t, "console", cfg.Logger.Encoding)
	assert.Equal(t, 2048, cfg.WebSocket.ReadBufferSize)
	assert.Equal(t, 2048, cfg.WebSocket.WriteBufferSize)
	assert.Equal(t, int64(1048576), cfg.WebSocket.MaxMessageSize)
	assert.Equal(t, "yaml-secret-key", cfg.JWT.SecretKey)
	assert.Equal(t, "yaml-issuer", cfg.JWT.Issuer)
	assert.Equal(t, 2*time.Hour, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "yaml-user", cfg.Database.User)
	assert.Equal(t, "yaml-password", cfg.Database.Password)
	assert.Equal(t, "testdb", cfg.Database.Database)
}

// TestBofryLoader_LoadFromEnv tests loading configuration from environment variables
func TestBofryLoader_LoadFromEnv(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"GORTEX_SERVER_PORT":               "9090",
		"GORTEX_SERVER_ADDRESS":            ":9090",
		"GORTEX_SERVER_GZIP":               "false",
		"GORTEX_SERVER_READ_TIMEOUT":       "60s",
		"GORTEX_LOGGER_LEVEL":              "debug",
		"GORTEX_LOGGER_ENCODING":           "console",
		"GORTEX_WEBSOCKET_READ_BUFFER_SIZE": "4096",
		"GORTEX_JWT_SECRET_KEY":            "env-secret-key",
		"GORTEX_JWT_ISSUER":                "env-issuer",
		"GORTEX_JWT_ACCESS_TOKEN_TTL":      "30m",
		"GORTEX_DATABASE_HOST":             "db.example.com",
		"GORTEX_DATABASE_PORT":             "5433",
		"GORTEX_DATABASE_USER":             "env-user",
		"GORTEX_DATABASE_PASSWORD":         "env-password",
		"GORTEX_DATABASE_DATABASE":         "envdb",
	}

	// Set all env vars
	for k, v := range envVars {
		os.Setenv(k, v)
	}
	
	// Clean up after test
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	loader := config.NewBofryLoader().WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	// Verify environment variables were loaded
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "debug", cfg.Logger.Level)
	assert.Equal(t, "console", cfg.Logger.Encoding)
	assert.Equal(t, 4096, cfg.WebSocket.ReadBufferSize)
	assert.Equal(t, "env-secret-key", cfg.JWT.SecretKey)
	assert.Equal(t, "env-issuer", cfg.JWT.Issuer)
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "env-user", cfg.Database.User)
	assert.Equal(t, "env-password", cfg.Database.Password)
	assert.Equal(t, "envdb", cfg.Database.Database)
}

// TestBofryLoader_EnvOverridesYAML tests that environment variables override YAML values
func TestBofryLoader_EnvOverridesYAML(t *testing.T) {
	// Create YAML file
	yamlContent := `
server:
  port: "7777"
  address: ":7777"
logger:
  level: "error"
jwt:
  secret_key: "yaml-key"
  issuer: "yaml-issuer"
database:
  user: "yaml-user"
  password: "yaml-pass"
`
	tmpFile, err := os.CreateTemp("", "bofry-override-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Set environment variables to override some YAML values
	os.Setenv("GORTEX_SERVER_PORT", "9999")
	os.Setenv("GORTEX_JWT_SECRET_KEY", "env-override-key")
	defer func() {
		os.Unsetenv("GORTEX_SERVER_PORT")
		os.Unsetenv("GORTEX_JWT_SECRET_KEY")
	}()

	// Load configuration
	loader := config.NewBofryLoader().
		WithYAMLFile(tmpFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Environment variable should override YAML
	assert.Equal(t, "9999", cfg.Server.Port)
	assert.Equal(t, "env-override-key", cfg.JWT.SecretKey)
	// YAML value should be loaded for non-overridden fields
	assert.Equal(t, ":7777", cfg.Server.Address)
	assert.Equal(t, "error", cfg.Logger.Level)
	assert.Equal(t, "yaml-issuer", cfg.JWT.Issuer)
}

// TestBofryLoader_LoadFromDotEnv tests loading configuration from .env file
func TestBofryLoader_LoadFromDotEnv(t *testing.T) {
	// Create a temporary .env file
	envContent := `
GORTEX_SERVER_PORT=6666
GORTEX_SERVER_ADDRESS=:6666
GORTEX_LOGGER_LEVEL=trace
GORTEX_JWT_SECRET_KEY=dotenv-secret
GORTEX_DATABASE_USER=dotenv-user
GORTEX_DATABASE_PASSWORD=dotenv-pass
`
	tmpFile, err := os.CreateTemp("", ".env")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(envContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load configuration
	loader := config.NewBofryLoader().
		WithDotEnvFile(tmpFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Verify .env values were loaded
	assert.Equal(t, "6666", cfg.Server.Port)
	assert.Equal(t, ":6666", cfg.Server.Address)
	assert.Equal(t, "trace", cfg.Logger.Level)
	assert.Equal(t, "dotenv-secret", cfg.JWT.SecretKey)
	assert.Equal(t, "dotenv-user", cfg.Database.User)
	assert.Equal(t, "dotenv-pass", cfg.Database.Password)
	
	// Clean up environment variables set by LoadDotEnvFile
	os.Unsetenv("GORTEX_SERVER_PORT")
	os.Unsetenv("GORTEX_SERVER_ADDRESS")
	os.Unsetenv("GORTEX_LOGGER_LEVEL")
	os.Unsetenv("GORTEX_JWT_SECRET_KEY")
	os.Unsetenv("GORTEX_DATABASE_USER")
	os.Unsetenv("GORTEX_DATABASE_PASSWORD")
}

// TestBofryLoader_MultiSource tests loading from multiple sources with proper precedence
func TestBofryLoader_MultiSource(t *testing.T) {
	// Create YAML file with some values
	yamlContent := `
server:
  port: "1111"
  address: ":1111"
logger:
  level: "warn"
jwt:
  secret_key: "yaml-secret"
database:
  user: "yaml-user"
`
	yamlFile, err := os.CreateTemp("", "multi-*.yaml")
	require.NoError(t, err)
	defer os.Remove(yamlFile.Name())
	_, err = yamlFile.WriteString(yamlContent)
	require.NoError(t, err)
	yamlFile.Close()

	// Create .env file with overlapping and new values
	envContent := `
GORTEX_SERVER_PORT=2222
GORTEX_LOGGER_ENCODING=console
GORTEX_DATABASE_PASSWORD=dotenv-pass
`
	envFile, err := os.CreateTemp("", ".env")
	require.NoError(t, err)
	defer os.Remove(envFile.Name())
	_, err = envFile.WriteString(envContent)
	require.NoError(t, err)
	envFile.Close()

	// Set environment variable to test highest precedence
	os.Setenv("GORTEX_SERVER_PORT", "3333")
	defer os.Unsetenv("GORTEX_SERVER_PORT")

	// Load configuration from all sources
	loader := config.NewBofryLoader().
		WithYAMLFile(yamlFile.Name()).
		WithDotEnvFile(envFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// Verify precedence: env > .env > yaml > defaults
	assert.Equal(t, "3333", cfg.Server.Port) // from env var (highest priority)
	assert.Equal(t, ":1111", cfg.Server.Address) // from yaml (no override)
	assert.Equal(t, "warn", cfg.Logger.Level) // from yaml (no override)
	assert.Equal(t, "console", cfg.Logger.Encoding) // from .env
	assert.Equal(t, "yaml-secret", cfg.JWT.SecretKey) // from yaml (no override)
	assert.Equal(t, "yaml-user", cfg.Database.User) // from yaml (no override)
	assert.Equal(t, "dotenv-pass", cfg.Database.Password) // from .env
}

// TestBofryLoader_Validation tests that configuration validation is performed
func TestBofryLoader_Validation(t *testing.T) {
	// Create invalid configuration with empty required fields
	yamlContent := `
server:
  port: "8080"
jwt:
  secret_key: ""
database:
  user: ""
  password: ""
`
	tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load configuration should fail validation
	loader := config.NewBofryLoader().WithYAMLFile(tmpFile.Name())
	cfg := &config.Config{}
	err = loader.Load(cfg)
	
	// Should return validation error (one of the required fields is missing)
	require.Error(t, err)
	// The error could be about JWT secret key, database user, or database password
	assert.True(t, 
		strings.Contains(err.Error(), "JWT secret key is required") ||
		strings.Contains(err.Error(), "database user is required") ||
		strings.Contains(err.Error(), "database password is required"),
		"Expected validation error, got: %v", err)
}

// TestBofryLoader_BackwardCompatibility tests that the new loader maintains backward compatibility
func TestBofryLoader_BackwardCompatibility(t *testing.T) {
	// This test ensures that switching from SimpleLoader to BofryLoader
	// doesn't break existing configurations
	
	// Set up environment like the old tests
	os.Setenv("STMP_SERVER_PORT", "9090")
	os.Setenv("STMP_JWT_SECRET_KEY", "test-secret-key")
	os.Setenv("STMP_DATABASE_USER", "test-user")
	os.Setenv("STMP_DATABASE_PASSWORD", "test-pass")
	defer func() {
		os.Unsetenv("STMP_SERVER_PORT")
		os.Unsetenv("STMP_JWT_SECRET_KEY")
		os.Unsetenv("STMP_DATABASE_USER")
		os.Unsetenv("STMP_DATABASE_PASSWORD")
	}()

	// Old prefix should still work if configured
	loader := config.NewBofryLoader().WithEnvPrefix("STMP_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "test-secret-key", cfg.JWT.SecretKey)
}

// TestBofryLoader_MissingFiles tests graceful handling of missing files
func TestBofryLoader_MissingFiles(t *testing.T) {
	// Set required fields to pass validation
	os.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	os.Setenv("GORTEX_DATABASE_USER", "test-user")
	os.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")
	defer func() {
		os.Unsetenv("GORTEX_JWT_SECRET_KEY")
		os.Unsetenv("GORTEX_DATABASE_USER")
		os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	}()
	
	// Load with non-existent files should not error if files are optional
	loader := config.NewBofryLoader().
		WithYAMLFile("/non/existent/file.yaml").
		WithDotEnvFile("/non/existent/.env").
		WithEnvPrefix("GORTEX_")
	
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)
	
	// Should load defaults
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "info", cfg.Logger.Level)
}

// TestBofryLoader_BuilderPattern tests the builder pattern interface
func TestBofryLoader_BuilderPattern(t *testing.T) {
	// Set required fields to pass validation
	os.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	os.Setenv("GORTEX_DATABASE_USER", "test-user")
	os.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")
	defer func() {
		os.Unsetenv("GORTEX_JWT_SECRET_KEY")
		os.Unsetenv("GORTEX_DATABASE_USER")
		os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	}()
	
	// Test the fluent builder pattern works correctly
	cfg := &config.Config{}
	
	err := config.NewBofryLoader().
		WithYAMLFile("config.yaml").
		WithDotEnvFile(".env").
		WithEnvPrefix("GORTEX_").
		Load(cfg)
	
	require.NoError(t, err)
	
	// Should at least load defaults
	assert.NotNil(t, cfg)
	assert.Equal(t, "8080", cfg.Server.Port)
}