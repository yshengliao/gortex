# Observability Setup Guide

## Overview

Observability is crucial for understanding the behavior of your Gortex applications in production. This guide covers setting up comprehensive monitoring with metrics, tracing, and logging, including integration with popular observability platforms like Prometheus, Grafana, and Jaeger.

## Table of Contents

1. [Metrics Collector Configuration](#metrics-collector-configuration)
2. [Tracing Strategy and Sampling](#tracing-strategy-and-sampling)
3. [Logging Configuration](#logging-configuration)
4. [Monitoring Dashboard Design](#monitoring-dashboard-design)
5. [Alert Rules Configuration](#alert-rules-configuration)
6. [Prometheus & Grafana Integration](#prometheus--grafana-integration)
7. [Performance Optimization](#performance-optimization)
8. [Troubleshooting](#troubleshooting)

## Metrics Collector Configuration

### Choosing the Right Collector

Gortex provides multiple collector implementations optimized for different scenarios:

```go
// Example 1: Configuring collectors for different use cases
package main

import (
    "github.com/yshengliao/gortex/observability/metrics"
    "github.com/yshengliao/gortex/core/app"
)

func setupMetrics(cfg *app.Config) metrics.Collector {
    // For low-traffic applications (< 1000 RPS)
    if cfg.Server.ExpectedTraffic == "low" {
        return metrics.NewImprovedCollector(
            metrics.WithMaxCardinality(5000),
            metrics.WithEvictionCallback(func(key string, count int64) {
                logger.Warn("Metric evicted", 
                    zap.String("key", key),
                    zap.Int64("count", count))
            }),
        )
    }
    
    // For high-traffic applications (> 10000 RPS)
    if cfg.Server.ExpectedTraffic == "high" {
        return metrics.NewShardedCollector(
            metrics.WithShardCount(16), // Increase shards for more cores
            metrics.WithMaxCardinality(50000),
        )
    }
    
    // For medium traffic with read-heavy workload
    return metrics.NewOptimizedCollector(
        metrics.WithMaxCardinality(20000),
    )
}
```

### Configuring Metric Collection

```go
// Example 2: Complete metrics setup
type ApplicationMetrics struct {
    collector    metrics.Collector
    httpDuration *metrics.HistogramVec
    activeUsers  *metrics.GaugeVec
    orderTotal   *metrics.CounterVec
    cacheHits    *metrics.CounterVec
}

func NewApplicationMetrics() *ApplicationMetrics {
    collector := metrics.NewImprovedCollector(
        metrics.WithMaxCardinality(10000),
    )
    
    return &ApplicationMetrics{
        collector: collector,
        
        // HTTP request duration by endpoint and status
        httpDuration: collector.NewHistogramVec(
            "http_request_duration_seconds",
            "HTTP request latency",
            []string{"method", "endpoint", "status"},
            metrics.DefBuckets,
        ),
        
        // Active users by tier
        activeUsers: collector.NewGaugeVec(
            "active_users_total",
            "Number of active users",
            []string{"tier"},
        ),
        
        // Order totals by payment method
        orderTotal: collector.NewCounterVec(
            "order_total_amount",
            "Total order amount",
            []string{"payment_method", "currency"},
        ),
        
        // Cache performance
        cacheHits: collector.NewCounterVec(
            "cache_hits_total",
            "Number of cache hits",
            []string{"cache_name", "operation"},
        ),
    }
}

// Middleware to collect HTTP metrics
func (m *ApplicationMetrics) HTTPMetricsMiddleware() middleware.MiddlewareFunc {
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(c httpctx.Context) error {
            start := time.Now()
            
            // Get route pattern (not the actual path to avoid high cardinality)
            endpoint := c.Path() // e.g., "/users/:id" not "/users/123"
            method := c.Request().Method
            
            // Process request
            err := next(c)
            
            // Record metrics
            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            
            m.httpDuration.WithLabelValues(method, endpoint, status).Observe(duration)
            
            return err
        }
    }
}
```

### Managing Cardinality

High cardinality can cause memory issues. Here's how to manage it:

```go
// Example 3: Cardinality management strategies
func (m *ApplicationMetrics) RecordUserAction(userID, action string) {
    // ❌ BAD: Using userID directly creates high cardinality
    // m.userActions.WithLabelValues(userID, action).Inc()
    
    // ✅ GOOD: Bucket users to limit cardinality
    userBucket := getUserBucket(userID) // e.g., "bucket_1", "bucket_2", etc.
    m.userActions.WithLabelValues(userBucket, action).Inc()
}

func getUserBucket(userID string) string {
    // Simple bucketing by hash
    hash := fnv32a(userID)
    bucket := hash % 100
    return fmt.Sprintf("bucket_%d", bucket)
}

// Monitor cardinality
func (m *ApplicationMetrics) MonitorCardinality() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        info := m.collector.GetCardinalityInfo()
        if info.CurrentCardinality > info.MaxCardinality*0.8 {
            logger.Warn("High metric cardinality",
                zap.Int("current", info.CurrentCardinality),
                zap.Int("max", info.MaxCardinality),
                zap.Float64("usage_percent", float64(info.CurrentCardinality)/float64(info.MaxCardinality)*100),
            )
        }
    }
}
```

## Tracing Strategy and Sampling

### Setting Up Tracing

```go
// Example 4: Comprehensive tracing setup
package main

import (
    "github.com/yshengliao/gortex/observability/tracing"
    "github.com/yshengliao/gortex/observability/otel"
)

func setupTracing(cfg *TracingConfig) (tracing.EnhancedTracer, error) {
    // Create base tracer
    tracer := tracing.NewSimpleTracer(cfg.ServiceName)
    
    // Configure sampling
    sampler := configureSampling(cfg)
    
    // Setup OpenTelemetry if enabled
    if cfg.OpenTelemetry.Enabled {
        otelTracer, err := otel.NewOTelTracerAdapter(
            otel.WithEndpoint(cfg.OpenTelemetry.Endpoint),
            otel.WithSampler(sampler),
            otel.WithResource(
                semconv.ServiceNameKey.String(cfg.ServiceName),
                semconv.ServiceVersionKey.String(cfg.ServiceVersion),
                semconv.DeploymentEnvironmentKey.String(cfg.Environment),
            ),
        )
        if err != nil {
            return nil, err
        }
        return otelTracer, nil
    }
    
    return tracer, nil
}

func configureSampling(cfg *TracingConfig) Sampler {
    switch cfg.SamplingStrategy {
    case "always":
        return AlwaysSample()
        
    case "never":
        return NeverSample()
        
    case "probabilistic":
        return ProbabilisticSampler(cfg.SamplingRate)
        
    case "adaptive":
        // Adaptive sampling based on traffic
        return AdaptiveSampler(
            WithTargetTPS(100),           // Target 100 traces per second
            WithMinSamplingRate(0.001),   // Minimum 0.1% sampling
            WithMaxSamplingRate(1.0),     // Maximum 100% sampling
        )
        
    case "priority":
        // Different sampling rates for different operations
        return PrioritySampler(map[string]float64{
            "/health":       0.0001,  // Health checks: 0.01%
            "/metrics":      0.0001,  // Metrics endpoint: 0.01%
            "/api/orders":   0.1,     // Orders: 10%
            "/api/payments": 1.0,     // Payments: 100%
            "default":       0.01,    // Everything else: 1%
        })
        
    default:
        return ProbabilisticSampler(0.01) // Default 1%
    }
}
```

### Implementing Custom Sampling

```go
// Example 5: Custom sampling strategies
type ErrorSampler struct {
    baseRate float64
}

func (s *ErrorSampler) ShouldSample(span *tracing.Span) bool {
    // Always sample errors
    if span.Status() >= tracing.ERROR {
        return true
    }
    
    // Sample slow requests
    if duration := span.Duration(); duration > 1*time.Second {
        return true
    }
    
    // Probabilistic sampling for others
    return rand.Float64() < s.baseRate
}

// Composite sampler combining multiple strategies
type CompositeSampler struct {
    samplers []Sampler
}

func (s *CompositeSampler) ShouldSample(span *tracing.Span) bool {
    for _, sampler := range s.samplers {
        if sampler.ShouldSample(span) {
            return true
        }
    }
    return false
}
```

### Trace Context Propagation

```go
// Example 6: Proper trace propagation
func (h *OrderHandler) ProcessOrder(c httpctx.Context) error {
    // Extract or create span
    span := c.Span()
    if span == nil {
        span = h.tracer.StartSpan("ProcessOrder")
        defer span.End()
    }
    
    // Add span attributes
    span.AddTags(map[string]interface{}{
        "order.id":     c.Param("id"),
        "user.id":      c.Header("X-User-ID"),
        "http.method":  c.Request().Method,
        "http.url":     c.Request().URL.String(),
    })
    
    // Create child span for database operation
    dbSpan := h.tracer.StartSpan("database.query",
        tracing.WithParent(span),
        tracing.WithTags(map[string]interface{}{
            "db.system": "postgresql",
            "db.operation": "SELECT",
        }),
    )
    
    order, err := h.db.GetOrder(c.Request().Context(), c.Param("id"))
    dbSpan.End()
    
    if err != nil {
        span.SetError(err)
        span.LogEvent(tracing.ERROR, "Database query failed", map[string]any{
            "error": err.Error(),
        })
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    
    // Log important events
    span.LogEvent(tracing.INFO, "Order retrieved", map[string]any{
        "order_status": order.Status,
        "order_total":  order.Total,
    })
    
    return c.JSON(200, order)
}
```

## Logging Configuration

### Structured Logging Setup

```go
// Example 7: Production logging configuration
func setupLogging(cfg *LogConfig) (*zap.Logger, error) {
    // Base configuration
    zapConfig := zap.Config{
        Level:            zap.NewAtomicLevelAt(parseLevel(cfg.Level)),
        Development:      cfg.Development,
        Encoding:         cfg.Encoding, // "json" for production
        OutputPaths:      cfg.OutputPaths,
        ErrorOutputPaths: cfg.ErrorOutputPaths,
        EncoderConfig: zapcore.EncoderConfig{
            TimeKey:        "timestamp",
            LevelKey:       "level",
            NameKey:        "logger",
            CallerKey:      "caller",
            FunctionKey:    zapcore.OmitKey,
            MessageKey:     "message",
            StacktraceKey:  "stacktrace",
            LineEnding:     zapcore.DefaultLineEnding,
            EncodeLevel:    zapcore.LowercaseLevelEncoder,
            EncodeTime:     zapcore.ISO8601TimeEncoder,
            EncodeDuration: zapcore.MillisDurationEncoder,
            EncodeCaller:   zapcore.ShortCallerEncoder,
        },
    }
    
    // Add sampling for high-volume logs
    if !cfg.Development {
        zapConfig.Sampling = &zap.SamplingConfig{
            Initial:    100,  // Log first 100 entries
            Thereafter: 10,   // Then log every 10th
            Hook: func(e zapcore.Entry, d zapcore.SamplingDecision) {
                if d == zapcore.LogDropped {
                    // Track dropped logs in metrics
                    droppedLogsCounter.Inc()
                }
            },
        }
    }
    
    logger, err := zapConfig.Build()
    if err != nil {
        return nil, err
    }
    
    // Add common fields
    logger = logger.With(
        zap.String("service", cfg.ServiceName),
        zap.String("version", cfg.ServiceVersion),
        zap.String("environment", cfg.Environment),
    )
    
    return logger, nil
}
```

### Correlating Logs with Traces

```go
// Example 8: Log-trace correlation
func EnrichLoggerMiddleware(logger *zap.Logger) middleware.MiddlewareFunc {
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(c httpctx.Context) error {
            // Extract trace information
            span := c.Span()
            fields := []zap.Field{
                zap.String("request_id", c.RequestID()),
            }
            
            if span != nil {
                fields = append(fields,
                    zap.String("trace_id", span.TraceID()),
                    zap.String("span_id", span.SpanID()),
                )
            }
            
            // Create request-scoped logger
            reqLogger := logger.With(fields...)
            c.Set("logger", reqLogger)
            
            // Log request start
            reqLogger.Info("Request started",
                zap.String("method", c.Request().Method),
                zap.String("path", c.Path()),
                zap.String("remote_ip", c.RealIP()),
            )
            
            // Process request
            start := time.Now()
            err := next(c)
            duration := time.Since(start)
            
            // Log request completion
            if err != nil {
                reqLogger.Error("Request failed",
                    zap.Error(err),
                    zap.Duration("duration", duration),
                    zap.Int("status", c.Response().Status),
                )
            } else {
                reqLogger.Info("Request completed",
                    zap.Duration("duration", duration),
                    zap.Int("status", c.Response().Status),
                )
            }
            
            return err
        }
    }
}

// Helper to get logger from context
func GetLogger(c httpctx.Context) *zap.Logger {
    if logger, ok := c.Get("logger").(*zap.Logger); ok {
        return logger
    }
    return zap.L() // Fallback to global logger
}
```

## Monitoring Dashboard Design

### Essential Metrics Dashboard

```yaml
# Example 9: Grafana dashboard configuration (grafana-dashboard.json)
{
  "dashboard": {
    "title": "Gortex Application Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "sum(rate(http_request_duration_seconds_count[5m])) by (method, endpoint)"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "sum(rate(http_request_duration_seconds_count{status=~\"5..\"}[5m])) / sum(rate(http_request_duration_seconds_count[5m]))"
        }]
      },
      {
        "title": "P95 Latency",
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (endpoint, le))"
        }]
      },
      {
        "title": "Active Goroutines",
        "targets": [{
          "expr": "go_goroutines{service=\"gortex-app\"}"
        }]
      },
      {
        "title": "Memory Usage",
        "targets": [{
          "expr": "go_memstats_alloc_bytes{service=\"gortex-app\"}"
        }]
      },
      {
        "title": "Cache Hit Rate",
        "targets": [{
          "expr": "sum(rate(cache_hits_total[5m])) by (cache_name) / sum(rate(cache_requests_total[5m])) by (cache_name)"
        }]
      }
    ]
  }
}
```

### Business Metrics Dashboard

```go
// Example 10: Custom business metrics
type BusinessMetrics struct {
    orderValue    *metrics.HistogramVec
    userActivity  *metrics.GaugeVec
    conversionRate *metrics.GaugeVec
    revenueTotal  *metrics.CounterVec
}

func (m *BusinessMetrics) RecordOrder(order Order) {
    // Record order value distribution
    m.orderValue.WithLabelValues(
        order.Category,
        order.PaymentMethod,
    ).Observe(order.Total)
    
    // Track revenue
    m.revenueTotal.WithLabelValues(
        order.Currency,
        order.Region,
    ).Add(order.Total)
}

func (m *BusinessMetrics) UpdateConversionRate(rate float64, segment string) {
    m.conversionRate.WithLabelValues(segment).Set(rate)
}

// Periodic calculation of business metrics
func (m *BusinessMetrics) CalculateMetrics(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Calculate and update conversion rates
            for _, segment := range []string{"new", "returning", "premium"} {
                rate := m.calculateConversionRate(segment)
                m.UpdateConversionRate(rate, segment)
            }
            
            // Update active user counts
            m.updateActiveUsers()
        }
    }
}
```

## Alert Rules Configuration

### Prometheus Alert Rules

```yaml
# Example 11: prometheus-alerts.yml
groups:
  - name: gortex_app
    interval: 30s
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: |
          sum(rate(http_request_duration_seconds_count{status=~"5.."}[5m])) by (service)
          /
          sum(rate(http_request_duration_seconds_count[5m])) by (service)
          > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }} for {{ $labels.service }}"
      
      # High latency
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95,
            sum(rate(http_request_duration_seconds_bucket[5m])) by (endpoint, le)
          ) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency on {{ $labels.endpoint }}"
          description: "95th percentile latency is {{ $value }}s"
      
      # Memory pressure
      - alert: HighMemoryUsage
        expr: |
          go_memstats_alloc_bytes / go_memstats_sys_bytes > 0.9
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is at {{ $value | humanizePercentage }}"
      
      # Goroutine leak
      - alert: GoroutineLeak
        expr: |
          rate(go_goroutines[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Possible goroutine leak"
          description: "Goroutines increasing at {{ $value }} per second"
      
      # Business alerts
      - alert: LowConversionRate
        expr: |
          conversion_rate < 0.01
        for: 15m
        labels:
          severity: business
          team: product
        annotations:
          summary: "Low conversion rate in {{ $labels.segment }}"
          description: "Conversion rate dropped to {{ $value | humanizePercentage }}"
```

### Implementing Alert Routing

```go
// Example 12: Alert notification handling
type AlertManager struct {
    notifiers []Notifier
    logger    *zap.Logger
}

type Alert struct {
    Name        string
    Severity    string
    Labels      map[string]string
    Annotations map[string]string
    StartsAt    time.Time
}

func (am *AlertManager) HandleAlert(alert Alert) {
    // Log alert
    am.logger.Warn("Alert triggered",
        zap.String("alert", alert.Name),
        zap.String("severity", alert.Severity),
        zap.Any("labels", alert.Labels),
    )
    
    // Route based on severity
    switch alert.Severity {
    case "critical":
        am.notifyAll(alert)
        am.pageOnCall(alert)
    case "warning":
        am.notifySlack(alert)
    case "business":
        am.notifyBusinessTeam(alert)
    }
    
    // Record alert metric
    alertsTriggered.WithLabelValues(
        alert.Name,
        alert.Severity,
    ).Inc()
}
```

## Prometheus & Grafana Integration

### Complete Integration Example

```go
// Example 13: Full Prometheus integration
package main

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/yshengliao/gortex/core/app"
)

func setupPrometheus(app *app.App, metrics *ApplicationMetrics) {
    // Register collectors
    prometheus.MustRegister(metrics.collector)
    
    // Add Prometheus endpoint
    app.Router().GET("/metrics", func(c httpctx.Context) error {
        // Add basic auth for security
        user, pass, ok := c.Request().BasicAuth()
        if !ok || user != "prometheus" || pass != getMetricsPassword() {
            c.Response().Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
            return c.NoContent(401)
        }
        
        // Serve metrics
        handler := promhttp.HandlerFor(
            prometheus.DefaultGatherer,
            promhttp.HandlerOpts{
                EnableOpenMetrics: true,
                Timeout:          10 * time.Second,
            },
        )
        handler.ServeHTTP(c.Response(), c.Request())
        return nil
    })
}

// Docker Compose setup for local development
const dockerComposeYAML = `
version: '3.8'

services:
  gortex-app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - METRICS_ENABLED=true
      - TRACING_ENABLED=true
      - JAEGER_ENDPOINT=http://jaeger:14268/api/traces

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - ./alerts.yml:/etc/prometheus/alerts.yml
    ports:
      - "9090:9090"
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "5775:5775/udp"
      - "6831:6831/udp"
      - "6832:6832/udp"
      - "5778:5778"
      - "16686:16686"
      - "14268:14268"
      - "9411:9411"

  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml
`
```

## Performance Optimization

### Metrics Collection Optimization

```go
// Example 14: Optimizing metrics collection
type OptimizedMetrics struct {
    // Pre-allocated label combinations
    commonLabels map[string][]string
    
    // Batch updates
    batchSize    int
    batchTimeout time.Duration
    updateQueue  chan metricUpdate
}

func (m *OptimizedMetrics) BatchRecord() {
    batch := make([]metricUpdate, 0, m.batchSize)
    timer := time.NewTimer(m.batchTimeout)
    
    for {
        select {
        case update := <-m.updateQueue:
            batch = append(batch, update)
            
            if len(batch) >= m.batchSize {
                m.flushBatch(batch)
                batch = batch[:0]
                timer.Reset(m.batchTimeout)
            }
            
        case <-timer.C:
            if len(batch) > 0 {
                m.flushBatch(batch)
                batch = batch[:0]
            }
            timer.Reset(m.batchTimeout)
        }
    }
}

// Reduce cardinality with label normalization
func normalizeLabels(labels map[string]string) map[string]string {
    normalized := make(map[string]string, len(labels))
    
    for k, v := range labels {
        switch k {
        case "status_code":
            // Group status codes
            normalized[k] = fmt.Sprintf("%dxx", v[0]-'0')
        case "user_agent":
            // Simplify user agents
            normalized[k] = extractBrowser(v)
        case "error":
            // Group similar errors
            normalized[k] = classifyError(v)
        default:
            normalized[k] = v
        }
    }
    
    return normalized
}
```

### Logging Performance

```go
// Example 15: High-performance logging
func setupPerformantLogging() *zap.Logger {
    // Use buffered writes
    ws := zapcore.AddSync(&zapcore.BufferedWriteSyncer{
        WS:            os.Stdout,
        Size:          256 * 1024, // 256KB buffer
        FlushInterval: 5 * time.Second,
    })
    
    // Configure encoder for performance
    encoderCfg := zap.NewProductionEncoderConfig()
    encoderCfg.TimeKey = "" // Disable time if not needed
    encoderCfg.EncodeTime = zapcore.EpochTimeEncoder // Faster than ISO8601
    
    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderCfg),
        ws,
        zap.InfoLevel,
    )
    
    // Add sampling for high-frequency logs
    sampled := zapcore.NewSamplerWithOptions(
        core,
        time.Second,     // Sample per second
        100,             // First 100
        10,              // Thereafter every 10th
    )
    
    return zap.New(sampled)
}

// Conditional logging based on level
func (h *Handler) debugLog(msg string, fields ...zap.Field) {
    if h.logger.Core().Enabled(zap.DebugLevel) {
        h.logger.Debug(msg, fields...)
    }
}
```

## Troubleshooting

### Common Issues and Solutions

1. **High Memory Usage from Metrics**
   ```go
   // Monitor and alert on cardinality
   func monitorMetricsHealth(collector metrics.Collector) {
       info := collector.GetCardinalityInfo()
       stats := collector.GetEvictionStats()
       
       logger.Info("Metrics health",
           zap.Int("cardinality", info.CurrentCardinality),
           zap.Int64("evictions", stats.TotalEvictions),
           zap.Int64("evicted_observations", stats.TotalEvictedObservations),
       )
   }
   ```

2. **Missing Traces**
   - Check sampling configuration
   - Verify trace context propagation
   - Ensure Jaeger agent is reachable

3. **Dashboard Loading Slowly**
   - Reduce query complexity
   - Add recording rules for complex queries
   - Increase Prometheus retention and downsampling

4. **Alert Fatigue**
   - Tune alert thresholds based on baseline
   - Implement alert suppression rules
   - Use alert routing to appropriate teams

### Debug Helpers

```go
// Example 16: Observability debugging
func DebugObservability(app *app.App) {
    // Endpoint to check metrics
    app.Router().GET("/_debug/metrics", func(c httpctx.Context) error {
        collector := app.Metrics()
        info := collector.GetCardinalityInfo()
        
        return c.JSON(200, map[string]interface{}{
            "cardinality": info,
            "evictions":   collector.GetEvictionStats(),
            "sample_metrics": collector.GetSampleMetrics(10),
        })
    })
    
    // Endpoint to check tracing
    app.Router().GET("/_debug/trace", func(c httpctx.Context) error {
        span := c.Span()
        if span == nil {
            return c.JSON(200, map[string]string{
                "status": "tracing disabled",
            })
        }
        
        return c.JSON(200, map[string]interface{}{
            "trace_id": span.TraceID(),
            "span_id":  span.SpanID(),
            "sampled":  span.IsSampled(),
        })
    })
}
```

## Summary

Setting up comprehensive observability requires:

1. **Metrics**: Choose appropriate collectors, manage cardinality, design meaningful dashboards
2. **Tracing**: Implement proper sampling, ensure context propagation, correlate with logs
3. **Logging**: Structure logs properly, optimize performance, correlate with traces
4. **Monitoring**: Design dashboards for both technical and business metrics
5. **Alerting**: Configure meaningful alerts with proper routing and suppression
6. **Integration**: Set up Prometheus, Grafana, and Jaeger with proper security

Remember to:
- Start with essential metrics and expand gradually
- Monitor the monitoring system itself
- Regularly review and tune alerts
- Keep cardinality under control
- Document your metrics and what they mean