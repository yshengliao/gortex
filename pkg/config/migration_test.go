package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/config"
)

// Migration tests: these verify that simpleLoader (via NewLoader) provides
// identical behaviour to the former BofryLoader. When all tests in this file
// pass, it is safe to remove the Bofry/config dependency.

func TestSimpleLoader_LoadDefaults(t *testing.T) {
	t.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	t.Setenv("GORTEX_DATABASE_USER", "test-user")
	t.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")
	// Clear any leaked env vars from other test files
	t.Setenv("GORTEX_LOGGER_ENCODING", "")
	t.Setenv("GORTEX_LOGGER_LEVEL", "")
	t.Setenv("GORTEX_SERVER_PORT", "")
	t.Setenv("GORTEX_SERVER_ADDRESS", "")

	loader := config.NewLoader().WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.True(t, cfg.Server.Recovery)
	assert.Equal(t, "info", cfg.Logger.Level)
	assert.Equal(t, "json", cfg.Logger.Encoding)
}

func TestSimpleLoader_LoadFromYAML(t *testing.T) {
	yamlContent := `
server:
  port: "8888"
  address: ":8888"
  gzip: false
  read_timeout: 45s
  shutdown_timeout: 7s
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
	tmpFile, err := os.CreateTemp(t.TempDir(), "simple-config-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	loader := config.NewLoader().WithYAMLFile(tmpFile.Name())
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "8888", cfg.Server.Port)
	assert.Equal(t, ":8888", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, 45*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 7*time.Second, cfg.Server.ShutdownTimeout)
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

func TestSimpleLoader_LoadFromEnv(t *testing.T) {
	envVars := map[string]string{
		"GORTEX_SERVER_PORT":                "9090",
		"GORTEX_SERVER_ADDRESS":             ":9090",
		"GORTEX_SERVER_GZIP":                "false",
		"GORTEX_SERVER_READ_TIMEOUT":        "60s",
		"GORTEX_SERVER_SHUTDOWN_TIMEOUT":     "70s",
		"GORTEX_LOGGER_LEVEL":               "debug",
		"GORTEX_LOGGER_ENCODING":            "console",
		"GORTEX_WEBSOCKET_READ_BUFFER_SIZE": "4096",
		"GORTEX_JWT_SECRET_KEY":             "env-secret-key",
		"GORTEX_JWT_ISSUER":                 "env-issuer",
		"GORTEX_JWT_ACCESS_TOKEN_TTL":       "30m",
		"GORTEX_DATABASE_HOST":              "db.example.com",
		"GORTEX_DATABASE_PORT":              "5433",
		"GORTEX_DATABASE_USER":              "env-user",
		"GORTEX_DATABASE_PASSWORD":          "env-password",
		"GORTEX_DATABASE_DATABASE":          "envdb",
	}
	for k, v := range envVars {
		t.Setenv(k, v)
	}

	loader := config.NewLoader().WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.False(t, cfg.Server.GZip)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 70*time.Second, cfg.Server.ShutdownTimeout)
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

func TestSimpleLoader_EnvOverridesYAML(t *testing.T) {
	yamlContent := `
server:
  port: "7777"
  address: ":7777"
  shutdown_timeout: 40s
logger:
  level: "error"
jwt:
  secret_key: "yaml-key"
  issuer: "yaml-issuer"
database:
  user: "yaml-user"
  password: "yaml-pass"
`
	tmpFile, err := os.CreateTemp(t.TempDir(), "simple-override-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	t.Setenv("GORTEX_SERVER_PORT", "9999")
	t.Setenv("GORTEX_SERVER_SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("GORTEX_JWT_SECRET_KEY", "env-override-key")

	loader := config.NewLoader().
		WithYAMLFile(tmpFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// env overrides YAML
	assert.Equal(t, "9999", cfg.Server.Port)
	assert.Equal(t, 5*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "env-override-key", cfg.JWT.SecretKey)
	// YAML values preserved where no env override
	assert.Equal(t, ":7777", cfg.Server.Address)
	assert.Equal(t, "error", cfg.Logger.Level)
	assert.Equal(t, "yaml-issuer", cfg.JWT.Issuer)
}

func TestSimpleLoader_DotEnvFile(t *testing.T) {
	envContent := `
GORTEX_SERVER_PORT=6666
GORTEX_SERVER_ADDRESS=:6666
GORTEX_LOGGER_LEVEL=trace
GORTEX_SERVER_SHUTDOWN_TIMEOUT=8s
GORTEX_JWT_SECRET_KEY=dotenv-secret
GORTEX_DATABASE_USER=dotenv-user
GORTEX_DATABASE_PASSWORD=dotenv-pass
`
	tmpFile, err := os.CreateTemp(t.TempDir(), ".env")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(envContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Ensure these keys don't exist yet, so .env loading can set them.
	// Register cleanup to remove them after the test.
	dotEnvKeys := []string{
		"GORTEX_SERVER_PORT", "GORTEX_SERVER_ADDRESS", "GORTEX_LOGGER_LEVEL",
		"GORTEX_SERVER_SHUTDOWN_TIMEOUT", "GORTEX_JWT_SECRET_KEY",
		"GORTEX_DATABASE_USER", "GORTEX_DATABASE_PASSWORD",
	}
	for _, key := range dotEnvKeys {
		os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for _, key := range dotEnvKeys {
			os.Unsetenv(key)
		}
	})

	loader := config.NewLoader().
		WithDotEnvFile(tmpFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "6666", cfg.Server.Port)
	assert.Equal(t, ":6666", cfg.Server.Address)
	assert.Equal(t, "trace", cfg.Logger.Level)
	assert.Equal(t, 8*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "dotenv-secret", cfg.JWT.SecretKey)
	assert.Equal(t, "dotenv-user", cfg.Database.User)
	assert.Equal(t, "dotenv-pass", cfg.Database.Password)
}

func TestSimpleLoader_MissingFiles(t *testing.T) {
	t.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	t.Setenv("GORTEX_DATABASE_USER", "test-user")
	t.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")

	loader := config.NewLoader().
		WithYAMLFile("/non/existent/file.yaml").
		WithDotEnvFile("/non/existent/.env").
		WithEnvPrefix("GORTEX_")

	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "info", cfg.Logger.Level)
}

func TestSimpleLoader_ValidationFails(t *testing.T) {
	// No required env vars set → should fail validation
	loader := config.NewLoader()
	cfg := &config.Config{}
	err := loader.Load(cfg)

	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "JWT secret key is required") ||
			strings.Contains(err.Error(), "database user is required") ||
			strings.Contains(err.Error(), "database password is required"),
		"Expected validation error, got: %v", err)
}

func TestSimpleLoader_CustomEnvPrefix(t *testing.T) {
	t.Setenv("MYAPP_SERVER_PORT", "5555")
	t.Setenv("MYAPP_SERVER_SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("MYAPP_JWT_SECRET_KEY", "myapp-secret")
	t.Setenv("MYAPP_DATABASE_USER", "myapp-user")
	t.Setenv("MYAPP_DATABASE_PASSWORD", "myapp-pass")

	loader := config.NewLoader().WithEnvPrefix("MYAPP_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "5555", cfg.Server.Port)
	assert.Equal(t, 25*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "myapp-secret", cfg.JWT.SecretKey)
}

func TestSimpleLoader_CommandArgsOverride(t *testing.T) {
	yamlContent := `
server:
  port: "1111"
jwt:
  secret_key: "yaml-key"
database:
  user: "yaml-user"
`
	yamlFile, err := os.CreateTemp(t.TempDir(), "flags-*.yaml")
	require.NoError(t, err)
	_, err = yamlFile.WriteString(yamlContent)
	require.NoError(t, err)
	yamlFile.Close()

	envContent := `
GORTEX_SERVER_PORT=2222
GORTEX_DATABASE_PASSWORD=dotenv-pass
`
	envFile, err := os.CreateTemp(t.TempDir(), ".env")
	require.NoError(t, err)
	_, err = envFile.WriteString(envContent)
	require.NoError(t, err)
	envFile.Close()

	// Pre-register keys that will be set by .env loading
	os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	t.Cleanup(func() { os.Unsetenv("GORTEX_DATABASE_PASSWORD") })

	// Real env var: should be overridden by CLI arg (4444 > 3333)
	t.Setenv("GORTEX_SERVER_PORT", "3333")

	origArgs := os.Args
	os.Args = append([]string{origArgs[0]}, "--server-port=4444")
	t.Cleanup(func() { os.Args = origArgs })

	loader := config.NewLoader().
		WithYAMLFile(yamlFile.Name()).
		WithDotEnvFile(envFile.Name()).
		WithEnvPrefix("GORTEX_").
		WithCommandArguments()
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "4444", cfg.Server.Port)
	assert.Equal(t, "yaml-key", cfg.JWT.SecretKey)
	assert.Equal(t, "yaml-user", cfg.Database.User)
	assert.Equal(t, "dotenv-pass", cfg.Database.Password)
}

func TestSimpleLoader_MultiSource(t *testing.T) {
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
	yamlFile, err := os.CreateTemp(t.TempDir(), "multi-*.yaml")
	require.NoError(t, err)
	_, err = yamlFile.WriteString(yamlContent)
	require.NoError(t, err)
	yamlFile.Close()

	envContent := `
GORTEX_SERVER_PORT=2222
GORTEX_LOGGER_ENCODING=console
GORTEX_DATABASE_PASSWORD=dotenv-pass
`
	envFile, err := os.CreateTemp(t.TempDir(), ".env")
	require.NoError(t, err)
	_, err = envFile.WriteString(envContent)
	require.NoError(t, err)
	envFile.Close()

	// Pre-register .env-only keys for cleanup
	for _, key := range []string{"GORTEX_LOGGER_ENCODING", "GORTEX_DATABASE_PASSWORD"} {
		os.Unsetenv(key)
	}
	t.Cleanup(func() {
		os.Unsetenv("GORTEX_LOGGER_ENCODING")
		os.Unsetenv("GORTEX_DATABASE_PASSWORD")
	})

	// Real env var: should win over .env value (3333 > 2222)
	t.Setenv("GORTEX_SERVER_PORT", "3333")

	loader := config.NewLoader().
		WithYAMLFile(yamlFile.Name()).
		WithDotEnvFile(envFile.Name()).
		WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err = loader.Load(cfg)
	require.NoError(t, err)

	// env var > .env > yaml > defaults
	assert.Equal(t, "3333", cfg.Server.Port)              // env var (highest)
	assert.Equal(t, ":1111", cfg.Server.Address)           // yaml
	assert.Equal(t, "warn", cfg.Logger.Level)              // yaml
	assert.Equal(t, "console", cfg.Logger.Encoding)        // .env
	assert.Equal(t, "yaml-secret", cfg.JWT.SecretKey)      // yaml
	assert.Equal(t, "yaml-user", cfg.Database.User)        // yaml
	assert.Equal(t, "dotenv-pass", cfg.Database.Password)  // .env
}
