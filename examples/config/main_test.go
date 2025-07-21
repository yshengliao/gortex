package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/config"
	"go.uber.org/zap"
)

// TestConfigExample tests the configuration loading example
func TestConfigExample(t *testing.T) {
	// Create a temporary config file for testing
	configContent := `
server:
  address: ":8080"
  recovery: true
  gzip: false
  cors: true

logger:
  level: "debug"
  output: "stdout"

jwt:
  secret_key: "test-secret-key"
  access_token_ttl: "1h"
  refresh_token_ttl: "168h"
  issuer: "test-config-example"

database:
  user: "test_user"
  password: "test_password"
`
	
	// Write config to temp file
	tmpFile := "test_config.yaml"
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)
	defer os.Remove(tmpFile)

	t.Run("Load Config from YAML", func(t *testing.T) {
		loader := config.NewSimpleLoader().
			WithYAMLFile(tmpFile).
			WithEnvPrefix("TEST_STMP_")

		cfg := &config.Config{}
		err := loader.Load(cfg)
		require.NoError(t, err)

		assert.Equal(t, ":8080", cfg.Server.Address)
		assert.True(t, cfg.Server.Recovery)
		assert.False(t, cfg.Server.GZip)
		assert.True(t, cfg.Server.CORS)
		assert.Equal(t, "debug", cfg.Logger.Level)
		assert.Equal(t, "test-secret-key", cfg.JWT.SecretKey)
		assert.Equal(t, "test-config-example", cfg.JWT.Issuer)
	})

	t.Run("Load Config with Environment Override", func(t *testing.T) {
		// Set environment variables
		os.Setenv("TEST_STMP_SERVER_ADDRESS", ":9090")
		os.Setenv("TEST_STMP_LOGGER_LEVEL", "info")
		defer os.Unsetenv("TEST_STMP_SERVER_ADDRESS")
		defer os.Unsetenv("TEST_STMP_LOGGER_LEVEL")

		loader := config.NewSimpleLoader().
			WithYAMLFile(tmpFile).
			WithEnvPrefix("TEST_STMP_")

		cfg := &config.Config{}
		err := loader.Load(cfg)
		require.NoError(t, err)

		// Environment variables should override YAML values
		assert.Equal(t, ":9090", cfg.Server.Address)
		assert.Equal(t, "info", cfg.Logger.Level)
		// Other values should remain from YAML
		assert.Equal(t, "test-secret-key", cfg.JWT.SecretKey)
	})

	t.Run("Logger Creation Based on Config", func(t *testing.T) {
		testCases := []struct {
			level       string
			expectDebug bool
		}{
			{"debug", true},
			{"info", false},
			{"warn", false},
		}

		for _, tc := range testCases {
			t.Run(tc.level, func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Logger.Level = tc.level

				var logger *zap.Logger
				var err error
				
				if cfg.Logger.Level == "debug" {
					logger, err = zap.NewDevelopment()
				} else {
					logger, err = zap.NewProduction()
				}
				require.NoError(t, err)
				defer logger.Sync()

				// Check if logger is created successfully
				assert.NotNil(t, logger)
			})
		}
	})

	t.Run("Missing Config File", func(t *testing.T) {
		// Set required environment variables
		os.Setenv("TEST_STMP_JWT_SECRET_KEY", "test-secret")
		os.Setenv("TEST_STMP_DATABASE_USER", "test_user")
		os.Setenv("TEST_STMP_DATABASE_PASSWORD", "test_password")
		defer os.Unsetenv("TEST_STMP_JWT_SECRET_KEY")
		defer os.Unsetenv("TEST_STMP_DATABASE_USER")
		defer os.Unsetenv("TEST_STMP_DATABASE_PASSWORD")
		
		loader := config.NewSimpleLoader().
			WithYAMLFile("non_existent.yaml").
			WithEnvPrefix("TEST_STMP_")

		cfg := &config.Config{}
		err := loader.Load(cfg)
		// Should handle missing file gracefully when env vars are set
		assert.NoError(t, err)
		assert.Equal(t, "test-secret", cfg.JWT.SecretKey)
	})
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	t.Run("Valid Configuration", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Server.Address = ":8080"
		cfg.Logger.Level = "info"
		cfg.JWT.SecretKey = "secret-key"
		cfg.JWT.Issuer = "test-issuer"
		
		// Normally we would validate the config here
		// For now, just check that required fields are set
		assert.NotEmpty(t, cfg.Server.Address)
		assert.NotEmpty(t, cfg.JWT.SecretKey)
	})
}

// BenchmarkConfigLoading benchmarks configuration loading
func BenchmarkConfigLoading(b *testing.B) {
	// Create a temporary config file
	configContent := `
server:
  address: ":8080"
logger:
  level: "info"
jwt:
  secret_key: "benchmark-secret"
  issuer: "benchmark-issuer"
`
	tmpFile := "bench_config.yaml"
	os.WriteFile(tmpFile, []byte(configContent), 0644)
	defer os.Remove(tmpFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader := config.NewSimpleLoader().
			WithYAMLFile(tmpFile).
			WithEnvPrefix("BENCH_")
		
		cfg := &config.Config{}
		loader.Load(cfg)
	}
}