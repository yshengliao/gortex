package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// BofryLoader is a configuration loader.
//
// Deprecated: Use NewLoader instead. BofryLoader now delegates entirely to
// simpleLoader and no longer depends on github.com/Bofry/config.
type BofryLoader = simpleLoader

// NewBofryLoader creates a new configuration loader.
//
// Deprecated: Use NewLoader instead.
func NewBofryLoader() *simpleLoader {
	return NewLoader()
}

// ConfigBuilder provides a fluent builder pattern for configuration.
type ConfigBuilder struct {
	loader *simpleLoader
}

// NewConfigBuilder creates a new configuration builder.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		loader: NewLoader(),
	}
}

// LoadYamlFile adds a YAML file source.
func (b *ConfigBuilder) LoadYamlFile(path string) *ConfigBuilder {
	b.loader.WithYAMLFile(path)
	return b
}

// LoadEnvironmentVariables sets the environment variable prefix.
func (b *ConfigBuilder) LoadEnvironmentVariables(prefix string) *ConfigBuilder {
	b.loader.WithEnvPrefix(prefix)
	return b
}

// LoadDotEnv adds a .env file source.
func (b *ConfigBuilder) LoadDotEnv(path string) *ConfigBuilder {
	b.loader.WithDotEnvFile(path)
	return b
}

// LoadCommandArguments enables command line argument parsing.
func (b *ConfigBuilder) LoadCommandArguments() *ConfigBuilder {
	b.loader.WithCommandArguments()
	return b
}

// Validate returns the builder for chaining (validation happens during Build).
func (b *ConfigBuilder) Validate() *ConfigBuilder {
	return b
}

// Build loads the configuration and returns it.
func (b *ConfigBuilder) Build() (*Config, error) {
	cfg := &Config{}
	err := b.loader.Load(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustBuild loads the configuration and panics on error.
func (b *ConfigBuilder) MustBuild() *Config {
	cfg, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build configuration: %v", err))
	}
	return cfg
}

// LoadFromDotEnv is a helper to load configuration from a .env file.
func LoadFromDotEnv(path string, cfg *Config) error {
	loader := NewLoader().WithDotEnvFile(path)
	return loader.Load(cfg)
}

// LoadWithBofry loads configuration using the Bofry-compatible system.
//
// Deprecated: Use NewLoader directly.
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

	loader := NewLoader().
		WithYAMLFile(yamlFile).
		WithDotEnvFile(dotEnvFile).
		WithEnvPrefix(envPrefix)

	return loader.Load(cfg)
}
