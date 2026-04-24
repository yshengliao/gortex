package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// simpleLoader is a zero-dependency configuration loader that supports:
//   - YAML files
//   - .env files (KEY=VALUE format)
//   - Environment variables
//   - Command line arguments (--key=value)
//
// Load precedence (highest → lowest): CLI args → env vars → .env file → YAML → defaults.
type simpleLoader struct {
	yamlFile       string
	dotEnvFile     string
	envPrefix      string
	useCommandArgs bool
}

// NewLoader creates a new configuration loader backed by simpleLoader.
// This is the recommended constructor; it requires no external dependencies.
func NewLoader() *simpleLoader {
	return &simpleLoader{
		envPrefix: "GORTEX_",
	}
}

// WithYAMLFile sets the YAML configuration file path.
func (l *simpleLoader) WithYAMLFile(path string) *simpleLoader {
	l.yamlFile = path
	return l
}

// WithDotEnvFile sets the .env file path.
func (l *simpleLoader) WithDotEnvFile(path string) *simpleLoader {
	l.dotEnvFile = path
	return l
}

// WithEnvPrefix sets the environment variable prefix.
func (l *simpleLoader) WithEnvPrefix(prefix string) *simpleLoader {
	l.envPrefix = prefix
	return l
}

// WithCommandArguments enables parsing command line arguments.
func (l *simpleLoader) WithCommandArguments() *simpleLoader {
	l.useCommandArgs = true
	return l
}

// Load loads configuration from various sources.
// Precedence (highest → lowest): CLI args → env vars → .env file → YAML → defaults.
func (l *simpleLoader) Load(cfg *Config) error {
	// Start with default configuration
	*cfg = *DefaultConfig()

	// Apply command line arguments as environment variables (highest priority)
	if l.useCommandArgs {
		l.applyCommandArgs()
	}

	// Load from YAML file if specified
	if l.yamlFile != "" {
		if err := l.loadFromYAML(cfg); err != nil {
			return fmt.Errorf("failed to load YAML config: %w", err)
		}
	}

	// Load .env file (sets env vars that don't already exist)
	if l.dotEnvFile != "" {
		if err := l.loadFromDotEnv(); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// Override with environment variables
	if err := l.loadFromEnv(cfg); err != nil {
		return fmt.Errorf("failed to load env config: %w", err)
	}

	// Validate configuration
	return cfg.Validate()
}

// loadFromYAML loads configuration from a YAML file.
func (l *simpleLoader) loadFromYAML(cfg *Config) error {
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

// loadFromDotEnv parses a .env file and sets environment variables.
// Lines are in KEY=VALUE format. Empty lines and lines starting with # are skipped.
// Existing environment variables are NOT overwritten (so real env vars take precedence).
func (l *simpleLoader) loadFromDotEnv() error {
	f, err := os.Open(l.dotEnvFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '='
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if the env var is not already set.
		// This ensures: real env vars > .env file values.
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// applyCommandArgs parses command line arguments in the form --name=value
// and sets them as environment variables using the configured prefix.
func (l *simpleLoader) applyCommandArgs() {
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

// loadFromEnv loads configuration from environment variables.
func (l *simpleLoader) loadFromEnv(cfg *Config) error {
	v := reflect.ValueOf(cfg).Elem()
	return l.loadStructFromEnv(v, l.envPrefix)
}

// loadStructFromEnv recursively loads struct fields from environment variables.
func (l *simpleLoader) loadStructFromEnv(v reflect.Value, prefix string) error {
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

// setFieldValue sets a reflect.Value from a string.
func (l *simpleLoader) setFieldValue(field reflect.Value, value string) error {
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

// LoadFromJSON loads configuration from JSON data.
func LoadFromJSON(data []byte, cfg *Config) error {
	return json.Unmarshal(data, cfg)
}
