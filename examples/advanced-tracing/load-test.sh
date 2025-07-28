#!/bin/bash

# Load testing script for advanced tracing example

set -e

# Configuration
TARGET_URL="${TARGET_URL:-http://localhost:8083}"
DURATION="${DURATION:-30s}"
RATE="${RATE:-10}"

echo "Load Testing Configuration:"
echo "- Target URL: $TARGET_URL"
echo "- Duration: $DURATION"
echo "- Rate: $RATE requests/second"
echo ""

# Check if vegeta is installed
if ! command -v vegeta &> /dev/null; then
    echo "Installing vegeta..."
    go install github.com/tsenart/vegeta@latest
fi

# Create test payloads
cat > /tmp/targets.txt << EOF
POST ${TARGET_URL}/orders
Content-Type: application/json
@/tmp/order1.json

POST ${TARGET_URL}/orders
Content-Type: application/json
@/tmp/order2.json

GET ${TARGET_URL}/inventory/item_001

GET ${TARGET_URL}/inventory/item_002

GET ${TARGET_URL}/analytics/sales

GET ${TARGET_URL}/health
EOF

# Create order payloads
cat > /tmp/order1.json << EOF
{
  "user_id": "load_test_user_1",
  "items": [
    {"item_id": "item_001", "name": "Laptop", "quantity": 1, "price": 999.99},
    {"item_id": "item_002", "name": "Mouse", "quantity": 2, "price": 29.99}
  ]
}
EOF

cat > /tmp/order2.json << EOF
{
  "user_id": "load_test_user_2",
  "items": [
    {"item_id": "item_003", "name": "Keyboard", "quantity": 1, "price": 79.99},
    {"item_id": "item_004", "name": "Monitor", "quantity": 1, "price": 299.99},
    {"item_id": "item_005", "name": "Headphones", "quantity": 3, "price": 149.99}
  ]
}
EOF

# Run load test
echo "Starting load test..."
vegeta attack -targets=/tmp/targets.txt -rate=$RATE -duration=$DURATION | \
    tee /tmp/results.bin | \
    vegeta report

# Generate detailed report
echo ""
echo "Generating detailed report..."
vegeta report -type=json /tmp/results.bin > /tmp/report.json
vegeta plot /tmp/results.bin > /tmp/plot.html

echo ""
echo "Load test complete!"
echo "- JSON report: /tmp/report.json"
echo "- HTML plot: /tmp/plot.html"
echo ""
echo "View traces in Jaeger UI: http://localhost:16686"

# Cleanup
rm -f /tmp/targets.txt /tmp/order1.json /tmp/order2.json