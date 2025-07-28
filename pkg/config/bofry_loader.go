package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bofryconfig "github.com/Bofry/config"
)

// BofryLoader is a configuration loader using Bofry/config library
// This provides enhanced configuration management with support for:
// - YAML files
// - Environment variables
// - .env files
// - Command line arguments
type BofryLoader struct {
	yamlFile       string
	dotEnvFile     string
	envPrefix      string
	useCommandArgs bool
}

// NewBofryLoader creates a new Bofry configuration loader
func NewBofryLoader() *BofryLoader {
	return &BofryLoader{
		envPrefix: "GORTEX_", // Default to GORTEX_ instead of STMP_ for new implementation
	}
}

// WithCommandArguments enables parsing command line arguments
func (l *BofryLoader) WithCommandArguments() *BofryLoader {
	l.useCommandArgs = true
	return l
}

// WithYAMLFile sets the YAML configuration file path
func (l *BofryLoader) WithYAMLFile(path string) *BofryLoader {
	l.yamlFile = path
	return l
}

// WithDotEnvFile sets the .env file path
func (l *BofryLoader) WithDotEnvFile(path string) *BofryLoader {
	l.dotEnvFile = path
	return l
}

// WithEnvPrefix sets the environment variable prefix
func (l *BofryLoader) WithEnvPrefix(prefix string) *BofryLoader {
	l.envPrefix = prefix
	return l
}

// Load loads configuration from various sources
func (l *BofryLoader) Load(cfg *Config) error {
	// Start with default configuration
	*cfg = *DefaultConfig()

	// Apply command line arguments as environment variables if enabled
	if l.useCommandArgs {
		l.applyCommandArgs()
	}

	// Bofry/config panics on errors, so we need to recover
	var loadErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					loadErr = err
				} else {
					loadErr = fmt.Errorf("configuration loading panic: %v", r)
				}
			}
		}()

		// Create Bofry configuration service
		configService := bofryconfig.NewConfigurationService(cfg)

		// Load YAML file if specified
		if l.yamlFile != "" {
			// Check if file exists first to avoid panic on missing file
			if _, err := os.Stat(l.yamlFile); err == nil {
				configService.LoadYamlFile(l.yamlFile)
			} else if !os.IsNotExist(err) {
				loadErr = fmt.Errorf("failed to check YAML file: %w", err)
				return
			}
			// If file doesn't exist, skip silently (like SimpleLoader)
		}

		// Load .env file if specified
		if l.dotEnvFile != "" {
			// Check if file exists first to avoid panic on missing file
			if _, err := os.Stat(l.dotEnvFile); err == nil {
				configService.LoadDotEnvFile(l.dotEnvFile)
			} else if !os.IsNotExist(err) {
				loadErr = fmt.Errorf("failed to check .env file: %w", err)
				return
			}
			// If file doesn't exist, skip silently
		}

		// Load environment variables
		// Note: Bofry/config's environment variable loading works differently than SimpleLoader
		// It doesn't automatically handle nested structs the same way.
		// For backward compatibility, we'll also use SimpleLoader's approach for env vars
		envPrefix := l.envPrefix
		if len(envPrefix) > 0 && envPrefix[len(envPrefix)-1] == '_' {
			envPrefix = envPrefix[:len(envPrefix)-1]
		}

		// First try Bofry's native env loading
		configService.LoadEnvironmentVariables(envPrefix)
	}()

	if loadErr != nil {
		return loadErr
	}

	// For backward compatibility with SimpleLoader's env handling,
	// we also manually load environment variables using the same logic
	if err := l.loadEnvCompat(cfg); err != nil {
		return fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Validate configuration
	return cfg.Validate()
}

// loadEnvCompat provides backward compatibility with SimpleLoader's environment variable handling
func (l *BofryLoader) loadEnvCompat(cfg *Config) error {
	// Use SimpleLoader's logic for environment variables to maintain compatibility
	loader := &SimpleLoader{envPrefix: l.envPrefix}
	return loader.loadFromEnv(cfg)
}

// applyCommandArgs parses command line arguments in the form --name=value
// and sets them as environment variables using the configured prefix.
func (l *BofryLoader) applyCommandArgs() {
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		kv := strings.SplitN(arg[2:], "=", 2)
		if len(kv) != 2 {
			continue
		}
		name := strings.ToUpper(strings.ReplaceAll(kv[0], "-", "_"))
		envName := l.envPrefix + name
		os.Setenv(envName, kv[1])
	}
}

// ConfigBuilder provides a fluent builder pattern for configuration
// This is to match the interface mentioned in the documentation
type ConfigBuilder struct {
	loader *BofryLoader
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		loader: NewBofryLoader(),
	}
}

// LoadYamlFile adds a YAML file source
func (b *ConfigBuilder) LoadYamlFile(path string) *ConfigBuilder {
	b.loader.WithYAMLFile(path)
	return b
}

// LoadEnvironmentVariables sets the environment variable prefix
func (b *ConfigBuilder) LoadEnvironmentVariables(prefix string) *ConfigBuilder {
	b.loader.WithEnvPrefix(prefix)
	return b
}

// LoadDotEnv adds a .env file source
func (b *ConfigBuilder) LoadDotEnv(path string) *ConfigBuilder {
	b.loader.WithDotEnvFile(path)
	return b
}

// LoadCommandArguments enables command line argument parsing
func (b *ConfigBuilder) LoadCommandArguments() *ConfigBuilder {
	b.loader.WithCommandArguments()
	return b
}

// Validate returns the builder for chaining (validation happens during Build)
func (b *ConfigBuilder) Validate() *ConfigBuilder {
	return b
}

// Build loads the configuration and returns it
func (b *ConfigBuilder) Build() (*Config, error) {
	cfg := &Config{}
	err := b.loader.Load(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustBuild loads the configuration and panics on error
func (b *ConfigBuilder) MustBuild() *Config {
	cfg, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build configuration: %v", err))
	}
	return cfg
}

// Migration helper to ease transition from SimpleLoader to BofryLoader
// This provides a compatible interface with the old SimpleLoader

// NewSimpleLoaderCompat creates a BofryLoader with SimpleLoader-compatible defaults
func NewSimpleLoaderCompat() *BofryLoader {
	return &BofryLoader{
		envPrefix: "STMP_", // Keep old prefix for compatibility
	}
}

// LoadFromDotEnv is a helper to load configuration from a .env file
func LoadFromDotEnv(path string, cfg *Config) error {
	loader := NewBofryLoader().WithDotEnvFile(path)
	return loader.Load(cfg)
}

// LoadWithBofry loads configuration using the new Bofry-based system
// This is a convenience function for common use cases
func LoadWithBofry(yamlFile string, envPrefix string, cfg *Config) error {
	// Look for .env file in the same directory as the YAML file
	dotEnvFile := ""
	if yamlFile != "" {
		dir := filepath.Dir(yamlFile)
		possibleDotEnv := filepath.Join(dir, ".env")
		if _, err := os.Stat(possibleDotEnv); err == nil {
			dotEnvFile = possibleDotEnv
		}
	}

	loader := NewBofryLoader().
		WithYAMLFile(yamlFile).
		WithDotEnvFile(dotEnvFile).
		WithEnvPrefix(envPrefix)

	return loader.Load(cfg)
}
