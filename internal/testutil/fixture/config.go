package fixture

import (
	"time"
	"github.com/yshengliao/gortex/config"
)

// TestConfig returns a test configuration with sensible defaults
func TestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:            "8080",
			Address:         ":8080",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			GZip:            true,
			CORS:            true,
			Recovery:        true,
		},
		Logger: config.LoggerConfig{
			Level:    "debug",
			Encoding: "json",
		},
		WebSocket: config.WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			MaxMessageSize:  512,
			PongWait:        60 * time.Second,
			PingPeriod:      54 * time.Second,
		},
	}
}

// MinimalConfig returns a minimal test configuration
func MinimalConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Address: ":0", // Random port
		},
		Logger: config.LoggerConfig{
			Level: "error", // Reduce noise in tests
		},
	}
}

// ConfigWithTimeout returns a config with custom timeouts for testing
func ConfigWithTimeout(timeout time.Duration) *config.Config {
	cfg := TestConfig()
	cfg.Server.ReadTimeout = timeout
	cfg.Server.WriteTimeout = timeout
	cfg.Server.ShutdownTimeout = timeout
	return cfg
}