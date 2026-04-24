package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/config"
)

// Smoke tests for deprecated BofryLoader API.
// Full loader behaviour is covered by migration_test.go (TestSimpleLoader_*).
// These tests verify only that the deprecated aliases still wire correctly.

func TestBofryLoader_DeprecatedAlias(t *testing.T) {
	t.Setenv("GORTEX_JWT_SECRET_KEY", "test-secret")
	t.Setenv("GORTEX_DATABASE_USER", "test-user")
	t.Setenv("GORTEX_DATABASE_PASSWORD", "test-pass")

	loader := config.NewBofryLoader().WithEnvPrefix("GORTEX_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "info", cfg.Logger.Level)
}

func TestConfigBuilder_BuilderPattern(t *testing.T) {
	yamlContent := `
server:
  port: "9999"
jwt:
  secret_key: "builder-secret"
database:
  user: "builder-user"
  password: "builder-pass"
`
	tmpFile, err := os.CreateTemp(t.TempDir(), "builder-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	cfg, err := config.NewConfigBuilder().
		LoadYamlFile(tmpFile.Name()).
		LoadEnvironmentVariables("GORTEX").
		Validate().
		Build()
	require.NoError(t, err)

	assert.Equal(t, "9999", cfg.Server.Port)
	assert.Equal(t, "builder-secret", cfg.JWT.SecretKey)
}

func TestConfigBuilder_MustBuild_Panics(t *testing.T) {
	// No required env vars → MustBuild should panic
	assert.Panics(t, func() {
		config.NewConfigBuilder().MustBuild()
	})
}

func TestBofryLoader_CustomPrefix(t *testing.T) {
	t.Setenv("MYPREFIX_SERVER_PORT", "7777")
	t.Setenv("MYPREFIX_SERVER_SHUTDOWN_TIMEOUT", "15s")
	t.Setenv("MYPREFIX_JWT_SECRET_KEY", "prefix-secret")
	t.Setenv("MYPREFIX_DATABASE_USER", "prefix-user")
	t.Setenv("MYPREFIX_DATABASE_PASSWORD", "prefix-pass")

	loader := config.NewBofryLoader().WithEnvPrefix("MYPREFIX_")
	cfg := &config.Config{}
	err := loader.Load(cfg)
	require.NoError(t, err)

	assert.Equal(t, "7777", cfg.Server.Port)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "prefix-secret", cfg.JWT.SecretKey)
}
