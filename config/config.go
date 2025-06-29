// Package config provides configuration management with future Bofry/config compatibility
package config

import (
	"fmt"
	"time"
)

// Config represents the application configuration structure
// This structure is designed to be compatible with Bofry/config when migrated
type Config struct {
	Server    ServerConfig    `yaml:"server" env:"SERVER"`
	Logger    LoggerConfig    `yaml:"logger" env:"LOGGER"`
	WebSocket WebSocketConfig `yaml:"websocket" env:"WEBSOCKET"`
	JWT       JWTConfig       `yaml:"jwt" env:"JWT"`
	Database  DatabaseConfig  `yaml:"database" env:"DATABASE"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string        `yaml:"port" env:"PORT" default:"8080"`
	Address      string        `yaml:"address" env:"ADDRESS" default:":8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"READ_TIMEOUT" default:"30s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"WRITE_TIMEOUT" default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"IDLE_TIMEOUT" default:"120s"`
	GZip         bool          `yaml:"gzip" env:"GZIP" default:"true"`
	CORS         bool          `yaml:"cors" env:"CORS" default:"true"`
	Recovery     bool          `yaml:"recovery" env:"RECOVERY" default:"true"`
}

// LoggerConfig holds logging configuration
type LoggerConfig struct {
	Level            string   `yaml:"level" env:"LEVEL" default:"info"`
	Encoding         string   `yaml:"encoding" env:"ENCODING" default:"json"`
	OutputPaths      []string `yaml:"output_paths" env:"OUTPUT_PATHS" default:"stdout"`
	ErrorOutputPaths []string `yaml:"error_output_paths" env:"ERROR_OUTPUT_PATHS" default:"stderr"`
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int           `yaml:"read_buffer_size" env:"READ_BUFFER_SIZE" default:"1024"`
	WriteBufferSize int           `yaml:"write_buffer_size" env:"WRITE_BUFFER_SIZE" default:"1024"`
	MaxMessageSize  int64         `yaml:"max_message_size" env:"MAX_MESSAGE_SIZE" default:"512"`
	PongWait        time.Duration `yaml:"pong_wait" env:"PONG_WAIT" default:"60s"`
	PingPeriod      time.Duration `yaml:"ping_period" env:"PING_PERIOD" default:"54s"`
}

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	SecretKey       string        `yaml:"secret_key" env:"SECRET_KEY,required" resource:".jwt-secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl" env:"ACCESS_TOKEN_TTL" default:"1h"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl" env:"REFRESH_TOKEN_TTL" default:"168h"` // 7 days
	Issuer          string        `yaml:"issuer" env:"ISSUER" default:"gortex-server"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver          string        `yaml:"driver" env:"DRIVER" default:"postgres"`
	Host            string        `yaml:"host" env:"HOST" default:"localhost"`
	Port            int           `yaml:"port" env:"PORT" default:"5432"`
	User            string        `yaml:"user" env:"USER,required"`
	Password        string        `yaml:"password" env:"PASSWORD,required" resource:".db-password"`
	Database        string        `yaml:"database" env:"DATABASE" default:"gortex"`
	SSLMode         string        `yaml:"ssl_mode" env:"SSL_MODE" default:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"MAX_IDLE_CONNS" default:"25"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"CONN_MAX_LIFETIME" default:"5m"`
}

// LoaderFunc is a function that loads configuration
// This allows for easy replacement with Bofry/config later
type LoaderFunc func(*Config) error

// Loader interface for configuration loading
// This will be implemented by Bofry/config when migrated
type Loader interface {
	Load(cfg *Config) error
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "8080",
			Address:      ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
			GZip:         true,
			CORS:         true,
			Recovery:     true,
		},
		Logger: LoggerConfig{
			Level:            "info",
			Encoding:         "json",
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			MaxMessageSize:  512 * 1024, // 512KB
			PongWait:        60 * time.Second,
			PingPeriod:      54 * time.Second,
		},
		JWT: JWTConfig{
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 7 * 24 * time.Hour,
			Issuer:          "gortex-server",
		},
		Database: DatabaseConfig{
			Driver:          "postgres",
			Host:            "localhost",
			Port:            5432,
			Database:        "gortex",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: 5 * time.Minute,
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Basic validation for required fields
	if c.JWT.SecretKey == "" {
		return fmt.Errorf("JWT secret key is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("database password is required")
	}
	return nil
}