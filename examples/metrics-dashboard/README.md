# Metrics Dashboard Example

This example demonstrates comprehensive metrics collection and visualization using Gortex, Prometheus, and Grafana.

## Features Demonstrated

### 1. Metrics Collection
- **HTTP Metrics**: Request rate, latency, status codes, in-flight requests
- **Business Metrics**: Orders, revenue, conversion rates, user activity
- **System Metrics**: Cache performance, database connections, memory usage
- **Custom Metrics**: Conversion funnel, cart abandonment, search analytics

### 2. Advanced Collectors
- **Sharded Collector**: High-performance metrics collection with 16 shards
- **Cardinality Management**: Automatic eviction of high-cardinality metrics
- **Memory Optimization**: Efficient storage with configurable limits

### 3. Visualization
- **3 Pre-built Dashboards**: HTTP, Business, and System Health
- **Real-time Updates**: 5-second refresh rate
- **Alerting Rules**: 7 pre-configured alerts for common issues

### 4. Production Features
- **Docker Compose Setup**: Complete monitoring stack
- **Service Discovery**: Automatic metric scraping
- **Load Testing**: Integrated performance testing

## Quick Start

### Using Docker Compose (Recommended)

1. Start all services:
```bash
docker-compose up -d
```

2. Wait for services to be ready:
```bash
docker-compose ps
```

3. Access the services:
- **Application**: http://localhost:8084
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

4. Import Grafana dashboards:
   - Login to Grafana
   - Go to Dashboards → Import
   - Upload JSON files from `grafana-dashboards/`

### Manual Setup

1. Start Prometheus:
```bash
prometheus --config.file=prometheus.yml
```

2. Start Grafana:
```bash
grafana-server
```

3. Run the application:
```bash
go run main.go
```

## API Endpoints

### 1. Dashboard - GET /
Interactive dashboard with test controls and feature overview.

### 2. Products - GET /products
Simulates product catalog with cache hit/miss and database queries.

**Response:**
```json
{
  "products": [
    {"id": "prod_1", "name": "Laptop", "price": 999.99, "category": "electronics"},
    {"id": "prod_2", "name": "Mouse", "price": 29.99, "category": "electronics"}
  ],
  "cached": true
}
```

### 3. Create Order - POST /orders
Process order with business metrics tracking.

**Request:**
```json
{
  "user_id": "user_123",
  "items": [
    {"product_id": "prod_1", "quantity": 2}
  ],
  "payment_method": "credit_card"
}
```

**Response:**
```json
{
  "order_id": "order_1706276400000",
  "status": "completed",
  "total": 245.67,
  "region": "us-west"
}
```

### 4. User Activity - GET /users/activity
Returns active users and search metrics.

### 5. Prometheus Metrics - GET /metrics
Standard Prometheus metrics endpoint.

### 6. Debug Metrics - GET /_debug/metrics
Cardinality and eviction statistics.

**Response:**
```json
{
  "cardinality": {
    "current": 1523,
    "max": 20000,
    "usage": "7.62%"
  },
  "evictions": {
    "total_evictions": 0,
    "total_observations": 0
  },
  "metrics": {
    "http_requests": "Counter tracking all HTTP requests",
    "order_value": "Histogram tracking order values"
  }
}
```

## Metrics Reference

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_request_duration_seconds` | Histogram | method, endpoint, status | Request latency |
| `http_requests_total` | Counter | method, endpoint, status | Total requests |
| `http_requests_in_flight` | Gauge | - | Current active requests |

### Business Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `orders_total` | Counter | status, payment_method | Order count |
| `order_value_dollars` | Histogram | category, region | Order value distribution |
| `product_views_total` | Counter | product_id, category | Product view count |
| `user_signups_total` | Counter | source, plan | New user signups |
| `active_users` | Gauge | tier, region | Currently active users |
| `conversion_rate_percent` | Gauge | funnel_stage, segment | Conversion percentages |
| `cart_abandoned_total` | Counter | reason | Abandoned cart count |
| `search_queries_total` | Counter | type, has_results | Search query count |

### System Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cache_hits_total` | Counter | cache_name | Cache hit count |
| `cache_misses_total` | Counter | cache_name | Cache miss count |
| `database_query_duration_seconds` | Histogram | query_type, table | Query latency |
| `database_connections` | Gauge | state | Connection pool status |

## Grafana Dashboards

### 1. HTTP Metrics Dashboard
- Request rate by method
- Error rate gauge
- Latency percentiles (p95, p99)
- Top endpoints table
- Status code distribution

### 2. Business Metrics Dashboard
- Key business KPIs (orders, revenue, conversion)
- Orders per minute by status
- Order value distribution
- Active users by tier
- Payment method breakdown
- Conversion funnel visualization

### 3. System Health Dashboard
- Cache hit rate gauge
- Database connection pool
- Cache operations timeline
- Database query latency
- Memory usage tracking
- Cache performance table

## Load Testing

### Quick Test
Run a 60-second load test:
```bash
docker-compose --profile load-test up load-generator
```

### Custom Load Test
```bash
./load-test.sh
```

Or with custom parameters:
```bash
TARGET_URL=http://localhost:8084 DURATION=120s RATE=50 ./load-test.sh
```

### Load Test Scenarios
1. **Mixed workload**: Products, orders, user activity
2. **Burst traffic**: Sudden spike simulation
3. **Sustained load**: Continuous traffic for stability testing

## Alerting Rules

### Configured Alerts

1. **HighErrorRate**: Error rate > 5% for 2 minutes
2. **HighLatency**: p95 latency > 500ms for 5 minutes
3. **LowConversionRate**: Conversion < 50% for 10 minutes
4. **DatabaseConnectionPoolExhaustion**: > 90% connections used
5. **HighCacheMissRate**: Cache miss rate > 50%
6. **UnusuallyHighOrderValue**: Order value > $5000
7. **NoOrdersReceived**: No orders in 10 minutes

### Adding Custom Alerts

Edit `alerts.yml` to add new rules:
```yaml
- alert: CustomAlert
  expr: your_metric > threshold
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Alert description"
```

## Configuration

### Application Configuration

```go
// Sharded collector for high-traffic
collector := metrics.NewShardedCollector(
    metrics.WithShardCount(16),         // Number of shards
    metrics.WithMaxCardinality(20000),  // Max unique label combinations
)
```

### Prometheus Configuration

- **Scrape interval**: 15 seconds
- **Evaluation interval**: 15 seconds
- **Retention**: 15 days (default)

### Grafana Configuration

- **Default datasource**: Prometheus
- **Auto-provisioning**: Dashboards and datasources
- **Anonymous access**: Disabled

## Production Considerations

### 1. Cardinality Management

```go
// Monitor cardinality
info := collector.GetCardinalityInfo()
if info.CurrentCardinality > 0.8 * info.MaxCardinality {
    // Alert on high cardinality
}
```

### 2. Performance Tuning

- **Shard count**: Increase for higher throughput
- **Batch size**: Adjust for network efficiency
- **Scrape interval**: Balance freshness vs load

### 3. Storage Backend

```yaml
# Long-term storage with Thanos
thanos:
  enabled: true
  objectStorage:
    type: s3
    config:
      bucket: metrics-bucket
```

### 4. Security

- Enable HTTPS for all endpoints
- Use authentication for Grafana
- Restrict Prometheus access
- Don't expose debug endpoints publicly

## Troubleshooting

### Common Issues

1. **High memory usage**
   - Check cardinality: `/_debug/metrics`
   - Review label combinations
   - Enable metric eviction

2. **Missing metrics**
   - Verify scrape targets in Prometheus
   - Check application logs
   - Ensure metrics are registered

3. **Slow dashboards**
   - Optimize queries with recording rules
   - Reduce dashboard refresh rate
   - Use aggregation functions

4. **Alert fatigue**
   - Tune alert thresholds
   - Add alert routing
   - Implement alert silencing

### Debug Commands

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Verify metrics
curl http://localhost:8084/metrics | grep order_total

# Test specific endpoint
curl -X POST http://localhost:8084/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test","items":[{"product_id":"prod_1","quantity":1}]}'
```

## Advanced Usage

### Custom Metrics

```go
// Add custom metric
customMetric := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "my_custom_metric",
        Help: "Description",
        Buckets: prometheus.DefBuckets,
    },
    []string{"label1", "label2"},
)
prometheus.MustRegister(customMetric)
```

### Recording Rules

```yaml
# prometheus-rules.yml
groups:
  - name: aggregations
    interval: 30s
    rules:
      - record: instance:http_requests:rate5m
        expr: sum(rate(http_requests_total[5m])) by (instance)
```

### Multi-tenancy

```go
// Add tenant label
metrics.httpRequests.WithLabelValues(
    method, endpoint, status, tenantID,
).Inc()
```

## Monitoring Best Practices

1. **USE Method**: Utilization, Saturation, Errors
2. **RED Method**: Rate, Errors, Duration
3. **Four Golden Signals**: Latency, Traffic, Errors, Saturation
4. **Business KPIs**: Revenue, Users, Conversion, Retention

## Resources

- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Gortex Observability Guide](../../docs/best-practices/observability-setup.md)
- [Metrics Cardinality Guide](https://prometheus.io/docs/practices/naming/#labels)