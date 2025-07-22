// Package services provides service interfaces for framework abstractions
package services

import (
	"context"
)

// Service is the base interface for all services
type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// HealthChecker defines the interface for health checking services
type HealthChecker interface {
	Check(ctx context.Context) error
}

// MetricsCollector defines the interface for metrics collection services
type MetricsCollector interface {
	Collect(ctx context.Context) (map[string]any, error)
}

// ConfigProvider defines the interface for configuration services
type ConfigProvider interface {
	Get(key string) any
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	Reload(ctx context.Context) error
}

// Logger defines the interface for logging services
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Fatal(msg string, fields ...any)
}

// EventBus defines the interface for event publishing/subscribing
type EventBus interface {
	Service
	Publish(ctx context.Context, topic string, event any) error
	Subscribe(topic string, handler func(event any)) error
	Unsubscribe(topic string) error
}

// CacheService defines the interface for caching services
type CacheService interface {
	Service
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}