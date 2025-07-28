package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SimpleLoader is a temporary loader implementation
// This will be replaced with Bofry/config in production
type SimpleLoader struct {
	yamlFile  string
	envPrefix string
}

// NewSimpleLoader creates a new simple configuration loader
func NewSimpleLoader() *SimpleLoader {
	return &SimpleLoader{
		envPrefix: "GORTEX_",
	}
}

// WithYAMLFile sets the YAML configuration file path
func (l *SimpleLoader) WithYAMLFile(path string) *SimpleLoader {
	l.yamlFile = path
	return l
}

// WithEnvPrefix sets the environment variable prefix
func (l *SimpleLoader) WithEnvPrefix(prefix string) *SimpleLoader {
	l.envPrefix = prefix
	return l
}

// Load loads configuration from various sources
func (l *SimpleLoader) Load(cfg *Config) error {
	// Start with default configuration
	*cfg = *DefaultConfig()

	// Load from YAML file if specified
	if l.yamlFile != "" {
		if err := l.loadFromYAML(cfg); err != nil {
			return fmt.Errorf("failed to load YAML config: %w", err)
		}
	}

	// Override with environment variables
	if err := l.loadFromEnv(cfg); err != nil {
		return fmt.Errorf("failed to load env config: %w", err)
	}

	// Validate configuration
	return cfg.Validate()
}

// loadFromYAML loads configuration from a YAML file
func (l *SimpleLoader) loadFromYAML(cfg *Config) error {
	data, err := os.ReadFile(l.yamlFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, skip
			return nil
		}
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

// loadFromEnv loads configuration from environment variables
func (l *SimpleLoader) loadFromEnv(cfg *Config) error {
	v := reflect.ValueOf(cfg).Elem()
	return l.loadStructFromEnv(v, l.envPrefix)
}

// loadStructFromEnv recursively loads struct fields from environment variables
func (l *SimpleLoader) loadStructFromEnv(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Parse env tag
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]

		// Build full environment variable name
		fullEnvName := prefix + envName

		// Check if it's a struct
		if field.Kind() == reflect.Struct && fieldType.Type != reflect.TypeOf(time.Duration(0)) && fieldType.Type != reflect.TypeOf(time.Time{}) {
			// Recursively load nested struct
			if err := l.loadStructFromEnv(field, fullEnvName+"_"); err != nil {
				return err
			}
			continue
		}

		// Get environment variable value
		envValue := os.Getenv(fullEnvName)
		if envValue == "" {
			continue
		}

		// Set field value
		if err := l.setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s from env %s: %w", fieldType.Name, fullEnvName, err)
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string
func (l *SimpleLoader) setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			// Parse duration
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(d))
		} else {
			// Parse int
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Slice:
		// Handle string slices
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			field.Set(reflect.ValueOf(values))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// LoadFromJSON loads configuration from JSON data
func LoadFromJSON(data []byte, cfg *Config) error {
	return json.Unmarshal(data, cfg)
}

// Migration Guide to Bofry/config:
//
// When ready to migrate to Bofry/config, replace the usage like this:
//
// Before (current implementation):
//   loader := config.NewSimpleLoader().
//       WithYAMLFile("config.yaml").
//       WithEnvPrefix("GORTEX_")
//   cfg := &config.Config{}
//   err := loader.Load(cfg)
//
// After (with Bofry/config):
//   import "github.com/Bofry/config"
//
//   cfg := &Config{}
//   err := config.NewConfigurationService(cfg).
//       LoadYamlFile("config.yaml").
//       LoadEnvironmentVariables("GORTEX").
//       LoadDotEnv(".env").
//       LoadCommandArguments()
//
// The Config struct tags are already compatible with Bofry/config
