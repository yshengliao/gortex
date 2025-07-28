package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/observability/metrics"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// HandlersManager demonstrates metrics collection
type HandlersManager struct {
	Home     *HomeHandler     `url:"/"`
	Products *ProductHandler  `url:"/products"`
	Orders   *OrderHandler    `url:"/orders"`
	Users    *UserHandler     `url:"/users"`
	Metrics  *MetricsHandler  `url:"/metrics"`
	Debug    *DebugHandler    `url:"/_debug"`
}

// ApplicationMetrics contains all application metrics
type ApplicationMetrics struct {
	collector metrics.Collector
	
	// HTTP metrics
	httpDuration   prometheus.HistogramVec
	httpRequests   prometheus.CounterVec
	httpInFlight   prometheus.Gauge
	
	// Business metrics
	orderTotal     prometheus.CounterVec
	orderValue     prometheus.HistogramVec
	productViews   prometheus.CounterVec
	userSignups    prometheus.CounterVec
	activeUsers    prometheus.GaugeVec
	
	// System metrics
	cacheHits      prometheus.CounterVec
	cacheMisses    prometheus.CounterVec
	dbQueries      prometheus.HistogramVec
	dbConnections  prometheus.GaugeVec
	
	// Custom business metrics
	conversionRate prometheus.GaugeVec
	cartAbandoned  prometheus.CounterVec
	searchQueries  prometheus.CounterVec
}

// NewApplicationMetrics creates metrics collectors
func NewApplicationMetrics() *ApplicationMetrics {
	// Use sharded collector for high-traffic scenarios
	collector := metrics.NewShardedCollector(
		metrics.WithShardCount(16),
		metrics.WithMaxCardinality(20000),
	)
	
	m := &ApplicationMetrics{
		collector: collector,
	}
	
	// HTTP metrics
	m.httpDuration = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status"},
	)
	
	m.httpRequests = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	
	m.httpInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
	
	// Business metrics
	m.orderTotal = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_total",
			Help: "Total number of orders",
		},
		[]string{"status", "payment_method"},
	)
	
	m.orderValue = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "order_value_dollars",
			Help:    "Order value distribution",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		},
		[]string{"category", "region"},
	)
	
	m.productViews = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "product_views_total",
			Help: "Total product views",
		},
		[]string{"product_id", "category"},
	)
	
	m.userSignups = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_signups_total",
			Help: "Total user signups",
		},
		[]string{"source", "plan"},
	)
	
	m.activeUsers = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_users",
			Help: "Number of active users",
		},
		[]string{"tier", "region"},
	)
	
	// System metrics
	m.cacheHits = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Number of cache hits",
		},
		[]string{"cache_name"},
	)
	
	m.cacheMisses = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Number of cache misses",
		},
		[]string{"cache_name"},
	)
	
	m.dbQueries = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"query_type", "table"},
	)
	
	m.dbConnections = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections",
			Help: "Number of database connections",
		},
		[]string{"state"},
	)
	
	// Custom business metrics
	m.conversionRate = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conversion_rate_percent",
			Help: "Conversion rate percentage",
		},
		[]string{"funnel_stage", "segment"},
	)
	
	m.cartAbandoned = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cart_abandoned_total",
			Help: "Number of abandoned carts",
		},
		[]string{"reason"},
	)
	
	m.searchQueries = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "search_queries_total",
			Help: "Number of search queries",
		},
		[]string{"type", "has_results"},
	)
	
	// Register all metrics
	prometheus.MustRegister(
		&m.httpDuration, &m.httpRequests, m.httpInFlight,
		&m.orderTotal, &m.orderValue, &m.productViews,
		&m.userSignups, &m.activeUsers,
		&m.cacheHits, &m.cacheMisses,
		&m.dbQueries, &m.dbConnections,
		&m.conversionRate, &m.cartAbandoned, &m.searchQueries,
	)
	
	return m
}

// MetricsMiddleware records HTTP metrics
func (m *ApplicationMetrics) MetricsMiddleware() middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c httpctx.Context) error {
			start := time.Now()
			
			// Track in-flight requests
			m.httpInFlight.Inc()
			defer m.httpInFlight.Dec()
			
			// Get route pattern (not actual path to avoid high cardinality)
			endpoint := c.Path()
			method := c.Request().Method
			
			// Process request
			err := next(c)
			
			// Record metrics
			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", c.Response().Status)
			
			m.httpDuration.WithLabelValues(method, endpoint, status).Observe(duration)
			m.httpRequests.WithLabelValues(method, endpoint, status).Inc()
			
			return err
		}
	}
}

// HomeHandler serves the dashboard
type HomeHandler struct {
	metrics *ApplicationMetrics
}

func (h *HomeHandler) GET(c httpctx.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Metrics Dashboard Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .feature { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .endpoint { background: #e8f4f8; padding: 10px; margin: 5px 0; }
        code { background: #e0e0e0; padding: 2px 5px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Gortex Metrics Dashboard Example</h1>
        
        <div class="feature">
            <h2>📊 Metrics Endpoints</h2>
            <div class="endpoint">
                <strong>Prometheus Metrics:</strong> <a href="/metrics">/metrics</a>
            </div>
            <div class="endpoint">
                <strong>Debug Metrics:</strong> <a href="/_debug/metrics">/_debug/metrics</a>
            </div>
        </div>
        
        <div class="feature">
            <h2>🎯 Test Endpoints</h2>
            <div class="endpoint">
                <strong>Product Catalog:</strong> GET <a href="/products">/products</a>
            </div>
            <div class="endpoint">
                <strong>Create Order:</strong> POST /orders
                <pre><code>{
  "user_id": "user_123",
  "items": [{"product_id": "prod_1", "quantity": 2}],
  "payment_method": "credit_card"
}</code></pre>
            </div>
            <div class="endpoint">
                <strong>User Activity:</strong> GET <a href="/users/activity">/users/activity</a>
            </div>
        </div>
        
        <div class="feature">
            <h2>📈 Grafana Dashboards</h2>
            <p>Import the dashboards from <code>grafana-dashboards/</code> directory:</p>
            <ul>
                <li><strong>HTTP Overview:</strong> Request rate, latency, errors</li>
                <li><strong>Business Metrics:</strong> Orders, revenue, conversion</li>
                <li><strong>System Health:</strong> Cache, database, resources</li>
            </ul>
        </div>
        
        <div class="feature">
            <h2>🚀 Features Demonstrated</h2>
            <ul>
                <li>Prometheus integration with custom collectors</li>
                <li>HTTP request/response metrics</li>
                <li>Business KPI tracking</li>
                <li>Cache and database performance metrics</li>
                <li>Custom metric cardinality management</li>
                <li>Real-time metric updates</li>
            </ul>
        </div>
        
        <div class="feature">
            <h2>🔧 Quick Test</h2>
            <button onclick="testEndpoints()">Run Test Requests</button>
            <div id="test-results"></div>
        </div>
    </div>
    
    <script>
    async function testEndpoints() {
        const results = document.getElementById('test-results');
        results.innerHTML = '<p>Running tests...</p>';
        
        try {
            // Test product view
            await fetch('/products');
            
            // Test order creation
            await fetch('/orders', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    user_id: 'test_user',
                    items: [{product_id: 'prod_1', quantity: 1}],
                    payment_method: 'credit_card'
                })
            });
            
            // Test user activity
            await fetch('/users/activity');
            
            results.innerHTML = '<p style="color: green;">✓ Test requests sent! Check <a href="/metrics">/metrics</a> for updated values.</p>';
        } catch (error) {
            results.innerHTML = '<p style="color: red;">✗ Error: ' + error.message + '</p>';
        }
    }
    </script>
</body>
</html>`
	
	return c.HTML(200, html)
}

// ProductHandler handles product operations
type ProductHandler struct {
	metrics *ApplicationMetrics
}

func (h *ProductHandler) GET(c httpctx.Context) error {
	// Simulate product listing
	products := []map[string]interface{}{
		{"id": "prod_1", "name": "Laptop", "price": 999.99, "category": "electronics"},
		{"id": "prod_2", "name": "Mouse", "price": 29.99, "category": "electronics"},
		{"id": "prod_3", "name": "Book", "price": 19.99, "category": "books"},
	}
	
	// Record product views
	for _, p := range products {
		h.metrics.productViews.WithLabelValues(
			p["id"].(string),
			p["category"].(string),
		).Inc()
	}
	
	// Simulate cache check
	cacheHit := rand.Float32() > 0.3
	if cacheHit {
		h.metrics.cacheHits.WithLabelValues("product_cache").Inc()
	} else {
		h.metrics.cacheMisses.WithLabelValues("product_cache").Inc()
		
		// Simulate database query
		queryStart := time.Now()
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
		h.metrics.dbQueries.WithLabelValues("select", "products").Observe(time.Since(queryStart).Seconds())
	}
	
	return c.JSON(200, map[string]interface{}{
		"products": products,
		"cached": cacheHit,
	})
}

// OrderHandler handles order operations
type OrderHandler struct {
	metrics *ApplicationMetrics
}

type OrderRequest struct {
	UserID        string `json:"user_id"`
	Items         []OrderItem `json:"items"`
	PaymentMethod string `json:"payment_method"`
}

type OrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

func (h *OrderHandler) POST(c httpctx.Context) error {
	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request"})
	}
	
	// Calculate order value
	orderValue := rand.Float64() * 500 + 50
	region := []string{"us-east", "us-west", "eu-west", "asia"}[rand.Intn(4)]
	category := []string{"electronics", "books", "clothing", "food"}[rand.Intn(4)]
	
	// Simulate order processing
	status := "completed"
	if rand.Float32() < 0.1 {
		status = "failed"
		h.metrics.cartAbandoned.WithLabelValues("payment_failed").Inc()
	}
	
	// Record metrics
	h.metrics.orderTotal.WithLabelValues(status, req.PaymentMethod).Inc()
	h.metrics.orderValue.WithLabelValues(category, region).Observe(orderValue)
	
	// Simulate database operations
	queryStart := time.Now()
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	h.metrics.dbQueries.WithLabelValues("insert", "orders").Observe(time.Since(queryStart).Seconds())
	
	// Update conversion rate
	h.updateConversionMetrics()
	
	return c.JSON(201, map[string]interface{}{
		"order_id": fmt.Sprintf("order_%d", time.Now().UnixNano()),
		"status":   status,
		"total":    orderValue,
		"region":   region,
	})
}

func (h *OrderHandler) updateConversionMetrics() {
	// Simulate conversion funnel metrics
	h.metrics.conversionRate.WithLabelValues("cart_to_checkout", "all").Set(rand.Float64() * 30 + 50)
	h.metrics.conversionRate.WithLabelValues("checkout_to_purchase", "all").Set(rand.Float64() * 20 + 60)
	h.metrics.conversionRate.WithLabelValues("cart_to_checkout", "premium").Set(rand.Float64() * 20 + 70)
	h.metrics.conversionRate.WithLabelValues("checkout_to_purchase", "premium").Set(rand.Float64() * 10 + 80)
}

// UserHandler handles user operations
type UserHandler struct {
	metrics *ApplicationMetrics
}

func (h *UserHandler) Activity(c httpctx.Context) error {
	// Simulate active users by tier and region
	tiers := []string{"free", "basic", "premium"}
	regions := []string{"us-east", "us-west", "eu-west", "asia"}
	
	for _, tier := range tiers {
		for _, region := range regions {
			activeCount := rand.Float64() * 1000
			h.metrics.activeUsers.WithLabelValues(tier, region).Set(activeCount)
		}
	}
	
	// Record search activity
	searchType := []string{"product", "user", "order"}[rand.Intn(3)]
	hasResults := []string{"true", "false"}[rand.Intn(2)]
	h.metrics.searchQueries.WithLabelValues(searchType, hasResults).Inc()
	
	// Simulate database connection pool
	h.metrics.dbConnections.WithLabelValues("active").Set(float64(rand.Intn(20) + 5))
	h.metrics.dbConnections.WithLabelValues("idle").Set(float64(rand.Intn(10) + 10))
	
	return c.JSON(200, map[string]interface{}{
		"active_users": map[string]interface{}{
			"total": rand.Intn(5000) + 1000,
			"new_today": rand.Intn(100) + 20,
		},
		"activity": map[string]interface{}{
			"page_views": rand.Intn(10000) + 5000,
			"searches": rand.Intn(1000) + 200,
		},
	})
}

// MetricsHandler serves Prometheus metrics
type MetricsHandler struct{}

func (h *MetricsHandler) GET(c httpctx.Context) error {
	// Serve Prometheus metrics
	promhttp.Handler().ServeHTTP(c.Response(), c.Request())
	return nil
}

// DebugHandler provides debug information
type DebugHandler struct {
	metrics *ApplicationMetrics
}

func (h *DebugHandler) Metrics(c httpctx.Context) error {
	// Get cardinality info from collector
	info := h.metrics.collector.GetCardinalityInfo()
	stats := h.metrics.collector.GetEvictionStats()
	
	return c.JSON(200, map[string]interface{}{
		"cardinality": map[string]interface{}{
			"current": info.CurrentCardinality,
			"max":     info.MaxCardinality,
			"usage":   fmt.Sprintf("%.2f%%", float64(info.CurrentCardinality)/float64(info.MaxCardinality)*100),
		},
		"evictions": map[string]interface{}{
			"total_evictions":     stats.TotalEvictions,
			"total_observations": stats.TotalEvictedObservations,
		},
		"metrics": map[string]interface{}{
			"http_requests":  "Counter tracking all HTTP requests",
			"http_duration":  "Histogram tracking request latency",
			"order_value":    "Histogram tracking order values",
			"active_users":   "Gauge showing current active users",
			"cache_hit_rate": "Calculated from cache_hits / (cache_hits + cache_misses)",
		},
	})
}

// SimulateBackgroundActivity generates continuous metrics
func simulateBackgroundActivity(metrics *ApplicationMetrics, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// Simulate user signups
		if rand.Float32() < 0.3 {
			source := []string{"organic", "social", "paid", "referral"}[rand.Intn(4)]
			plan := []string{"free", "basic", "premium"}[rand.Intn(3)]
			metrics.userSignups.WithLabelValues(source, plan).Inc()
		}
		
		// Update active users
		for _, tier := range []string{"free", "basic", "premium"} {
			for _, region := range []string{"us-east", "us-west", "eu-west", "asia"} {
				current := rand.Float64() * 1000 + float64(rand.Intn(500))
				metrics.activeUsers.WithLabelValues(tier, region).Set(current)
			}
		}
		
		// Simulate cart abandonment
		if rand.Float32() < 0.2 {
			reason := []string{"price", "shipping", "checkout_error", "other"}[rand.Intn(4)]
			metrics.cartAbandoned.WithLabelValues(reason).Inc()
		}
		
		logger.Debug("Background metrics updated")
	}
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize metrics
	appMetrics := NewApplicationMetrics()

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Server.Address = ":8084"
	cfg.Logger.Level = "info"

	// Create handlers
	handlers := &HandlersManager{
		Home:     &HomeHandler{metrics: appMetrics},
		Products: &ProductHandler{metrics: appMetrics},
		Orders:   &OrderHandler{metrics: appMetrics},
		Users:    &UserHandler{metrics: appMetrics},
		Metrics:  &MetricsHandler{},
		Debug:    &DebugHandler{metrics: appMetrics},
	}

	// Create app
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}

	// Add metrics middleware globally
	application.Use(appMetrics.MetricsMiddleware())

	// Start background activity simulation
	go simulateBackgroundActivity(appMetrics, logger)

	logger.Info("Starting Metrics Dashboard Example",
		zap.String("address", cfg.Server.Address))
	logger.Info("Available endpoints:",
		zap.String("dashboard", "http://localhost:8084/"),
		zap.String("prometheus_metrics", "http://localhost:8084/metrics"),
		zap.String("debug_metrics", "http://localhost:8084/_debug/metrics"),
	)
	logger.Info("Grafana dashboards available in grafana-dashboards/ directory")

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}