#!/bin/bash

# Load testing script for metrics dashboard example

set -e

# Configuration
TARGET_URL="${TARGET_URL:-http://localhost:8084}"
DURATION="${DURATION:-60s}"
RATE="${RATE:-20}"

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
GET ${TARGET_URL}/

GET ${TARGET_URL}/products

POST ${TARGET_URL}/orders
Content-Type: application/json
@/tmp/order1.json

POST ${TARGET_URL}/orders
Content-Type: application/json
@/tmp/order2.json

GET ${TARGET_URL}/users/activity

GET ${TARGET_URL}/metrics

GET ${TARGET_URL}/_debug/metrics
EOF

# Create order payloads with varying sizes
cat > /tmp/order1.json << EOF
{
  "user_id": "user_123",
  "items": [
    {"product_id": "prod_1", "quantity": 2},
    {"product_id": "prod_2", "quantity": 1}
  ],
  "payment_method": "credit_card"
}
EOF

cat > /tmp/order2.json << EOF
{
  "user_id": "user_456",
  "items": [
    {"product_id": "prod_3", "quantity": 5},
    {"product_id": "prod_1", "quantity": 3},
    {"product_id": "prod_2", "quantity": 10}
  ],
  "payment_method": "paypal"
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
echo "View metrics in:"
echo "- Prometheus: http://localhost:9090"
echo "- Grafana: http://localhost:3000 (admin/admin)"

# Cleanup
rm -f /tmp/targets.txt /tmp/order1.json /tmp/order2.json