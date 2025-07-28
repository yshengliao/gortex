# Advanced Tracing Example

This example demonstrates Gortex's advanced tracing capabilities, including all 8 severity levels, distributed tracing, and integration with external systems.

## Features Demonstrated

### 1. Eight Severity Levels
- **DEBUG**: Detailed debugging information
- **INFO**: General informational messages
- **NOTICE**: Normal but significant events
- **WARN**: Warning conditions
- **ERROR**: Error conditions
- **CRITICAL**: Critical conditions requiring attention
- **ALERT**: Action must be taken immediately
- **EMERGENCY**: System is unusable

### 2. Distributed Tracing
- Parent-child span relationships
- Cross-service trace propagation
- Parallel operation tracing
- Context propagation through the stack

### 3. External System Integration
- PostgreSQL database tracing
- Redis cache tracing
- Custom span attributes for each system
- Error tracking and propagation

### 4. Advanced Features
- Custom span attributes
- Event logging at different severity levels
- Error diagnosis and tracking
- Performance monitoring

## Quick Start

### Using Docker Compose

1. Start all services:
```bash
docker-compose up -d
```

2. Wait for services to be ready:
```bash
docker-compose ps
```

3. Access the application:
- Application: http://localhost:8083
- Jaeger UI: http://localhost:16686

### Manual Setup

1. Start PostgreSQL:
```bash
docker run -d --name postgres \
  -e POSTGRES_USER=gortex \
  -e POSTGRES_PASSWORD=gortex \
  -e POSTGRES_DB=gortex \
  -p 5432:5432 \
  postgres:15-alpine
```

2. Start Redis:
```bash
docker run -d --name redis \
  -p 6379:6379 \
  redis:7-alpine
```

3. Start Jaeger:
```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

4. Initialize the database:
```bash
psql -h localhost -U gortex -d gortex -f init.sql
```

5. Run the application:
```bash
go run main.go
```

## API Endpoints

### 1. Home - GET /
Shows available endpoints and features.

### 2. Create Order - POST /orders
Demonstrates all 8 severity levels of tracing.

**Request:**
```json
{
  "user_id": "user_123",
  "items": [
    {
      "item_id": "item_001",
      "name": "Laptop",
      "quantity": 1,
      "price": 999.99
    }
  ]
}
```

**Response:**
```json
{
  "id": "order_1234567890",
  "user_id": "user_123",
  "items": [...],
  "total": 1079.99,
  "status": "pending",
  "created_at": "2024-01-26T10:30:00Z"
}
```

### 3. Check Inventory - GET /inventory/:item_id
Shows cache and database tracing.

**Example:**
```bash
curl http://localhost:8083/inventory/item_001
```

### 4. Analytics - GET /analytics/sales
Demonstrates parallel operations with distributed tracing.

### 5. Health Check - GET /health
Shows system health with trace logging.

## Viewing Traces

1. Open Jaeger UI: http://localhost:16686
2. Select service: "advanced-tracing-example"
3. Click "Find Traces"
4. Click on any trace to see details

### Understanding Trace Details

- **Span Duration**: Time taken by each operation
- **Tags**: Custom attributes (db.system, cache.operation, etc.)
- **Logs**: Events at different severity levels
- **Errors**: Failed operations are marked in red

## Load Testing

Run the load test to generate trace data:

```bash
docker-compose --profile load-test up load-generator
```

Or manually:

```bash
# Install vegeta
go install github.com/tsenart/vegeta@latest

# Run load test
echo "POST http://localhost:8083/orders" | \
  vegeta attack -body order.json -duration=30s -rate=10 | \
  vegeta report
```

Sample `order.json`:
```json
{
  "user_id": "user_123",
  "items": [
    {"item_id": "item_001", "quantity": 1},
    {"item_id": "item_002", "quantity": 2}
  ]
}
```

## Configuration

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string
- `JAEGER_ENDPOINT`: Jaeger collector endpoint
- `LOG_LEVEL`: Logging level (debug, info, warn, error)

### Tracing Configuration

In production, use OpenTelemetry adapter:

```go
tracer, _ := otel.NewOTelTracerAdapter(
    otel.WithEndpoint(os.Getenv("JAEGER_ENDPOINT")),
    otel.WithServiceName("my-service"),
    otel.WithSampler(otel.ProbabilisticSampler(0.1)), // 10% sampling
)
```

## Severity Level Examples

The order creation endpoint demonstrates all severity levels:

1. **DEBUG**: Request details logging
2. **INFO**: Order received confirmation
3. **NOTICE**: Inventory check passed
4. **WARN**: Some items unavailable
5. **ERROR**: Calculation failures
6. **CRITICAL**: High-value order detection
7. **ALERT**: Validation failures requiring action
8. **EMERGENCY**: Database write failures

## Monitoring

### Key Metrics to Watch

1. **Trace Duration**: Overall request time
2. **Span Count**: Number of operations per request
3. **Error Rate**: Failed operations
4. **Cache Hit Rate**: Redis cache effectiveness
5. **Database Query Time**: PostgreSQL performance

### Sample Queries in Jaeger

1. Find slow requests:
   - Min Duration: 1s
   - Service: advanced-tracing-example

2. Find errors:
   - Tags: error=true
   - Service: advanced-tracing-example

3. Find high-value orders:
   - Tags: order.total>5000
   - Operation: CreateOrder

## Troubleshooting

### Common Issues

1. **Cannot connect to PostgreSQL**
   - Check if container is running: `docker ps`
   - Verify credentials in connection string
   - Check if database is initialized

2. **Redis connection refused**
   - Ensure Redis is running: `docker ps`
   - Check Redis logs: `docker logs redis`

3. **No traces in Jaeger**
   - Verify Jaeger is running: http://localhost:16686
   - Check application logs for errors
   - Ensure JAEGER_ENDPOINT is correct

4. **High memory usage**
   - Adjust sampling rate
   - Configure span limits
   - Monitor cardinality of tags

## Production Considerations

1. **Sampling Strategy**
   - Use adaptive sampling for high-traffic endpoints
   - Always sample errors and high-value transactions
   - Consider head-based vs tail-based sampling

2. **Performance Impact**
   - Tracing adds ~5-10% overhead
   - Use async span reporting
   - Batch span exports

3. **Data Retention**
   - Configure Jaeger storage backend (Cassandra/Elasticsearch)
   - Set appropriate retention policies
   - Archive important traces

4. **Security**
   - Don't trace sensitive data (passwords, tokens)
   - Use TLS for trace collection
   - Implement access controls for Jaeger UI