package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/observability/otel"
	"github.com/yshengliao/gortex/observability/tracing"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// HandlersManager demonstrates advanced tracing features
type HandlersManager struct {
	Home      *HomeHandler      `url:"/"`
	Orders    *OrderHandler     `url:"/orders"`
	Inventory *InventoryHandler `url:"/inventory"`
	Analytics *AnalyticsHandler `url:"/analytics"`
	Health    *HealthHandler    `url:"/health"`
}

// Services encapsulates all backend services
type Services struct {
	db     *sql.DB
	redis  *redis.Client
	tracer tracing.EnhancedTracer
	logger *zap.Logger
}

// HomeHandler serves the home page
type HomeHandler struct {
	services *Services
}

func (h *HomeHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]interface{}{
		"message": "Advanced Tracing Example",
		"endpoints": map[string]string{
			"create_order":    "POST /orders",
			"check_inventory": "GET /inventory/:item_id",
			"analytics":       "GET /analytics/sales",
			"health":          "GET /health",
		},
		"features": []string{
			"8-level severity tracing",
			"Distributed tracing across services",
			"Database and cache tracing",
			"Error tracking and diagnosis",
			"Custom span attributes",
		},
	})
}

// OrderHandler handles order operations
type OrderHandler struct {
	services *Services
}

// Order represents an order
type Order struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Items     []Item    `json:"items"`
	Total     float64   `json:"total"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Item represents an order item
type Item struct {
	ItemID   string  `json:"item_id"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// CreateOrder demonstrates all 8 severity levels
func (h *OrderHandler) POST(c httpctx.Context) error {
	span := c.Span()
	if span == nil {
		span = h.services.tracer.StartSpan("CreateOrder")
		defer span.End()
	}

	// DEBUG level - detailed debugging information
	span.LogEvent(tracing.DEBUG, "Starting order creation", map[string]any{
		"request_id": c.Request().Header.Get("X-Request-ID"),
		"user_agent": c.Request().UserAgent(),
	})

	// Parse request
	var req struct {
		UserID string `json:"user_id"`
		Items  []Item `json:"items"`
	}
	if err := c.Bind(&req); err != nil {
		span.SetError(err)
		span.LogEvent(tracing.ERROR, "Failed to parse request", map[string]any{
			"error": err.Error(),
		})
		return c.JSON(400, map[string]string{"error": "Invalid request"})
	}

	// INFO level - general information
	span.LogEvent(tracing.INFO, "Order request received", map[string]any{
		"user_id":    req.UserID,
		"item_count": len(req.Items),
	})

	// Validate inventory (with child span)
	inventorySpan := h.services.tracer.StartSpan("CheckInventory",
		tracing.WithParent(span),
		tracing.WithTags(map[string]interface{}{
			"items_count": len(req.Items),
		}),
	)

	availabilityErrors := h.checkInventory(c.Request().Context(), req.Items)
	inventorySpan.End()

	if len(availabilityErrors) > 0 {
		// WARN level - warning conditions
		span.LogEvent(tracing.WARN, "Some items not available", map[string]any{
			"unavailable_items": availabilityErrors,
		})
		return c.JSON(409, map[string]interface{}{
			"error":             "Items not available",
			"unavailable_items": availabilityErrors,
		})
	}

	// NOTICE level - normal but significant events
	span.LogEvent(tracing.NOTICE, "Inventory check passed", map[string]any{
		"timestamp": time.Now().Unix(),
	})

	// Calculate total with database span
	dbSpan := h.services.tracer.StartSpan("CalculateOrderTotal",
		tracing.WithParent(span),
		tracing.WithTags(map[string]interface{}{
			"db.system": "postgresql",
			"db.operation": "calculate_total",
		}),
	)

	total, err := h.calculateTotal(c.Request().Context(), req.Items)
	dbSpan.End()

	if err != nil {
		// ERROR level - error conditions
		span.SetError(err)
		span.LogEvent(tracing.ERROR, "Failed to calculate total", map[string]any{
			"error": err.Error(),
			"items": req.Items,
		})
		return c.JSON(500, map[string]string{"error": "Failed to calculate total"})
	}

	// Create order in database
	order := &Order{
		ID:        fmt.Sprintf("order_%d", time.Now().UnixNano()),
		UserID:    req.UserID,
		Items:     req.Items,
		Total:     total,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// Simulate critical condition
	if total > 10000 {
		// CRITICAL level - critical conditions
		span.LogEvent(tracing.CRITICAL, "High value order detected", map[string]any{
			"order_id": order.ID,
			"total":    total,
			"user_id":  req.UserID,
		})
		
		// Trigger additional validation
		if err := h.performHighValueValidation(c.Request().Context(), order); err != nil {
			// ALERT level - action must be taken immediately
			span.LogEvent(tracing.ALERT, "High value order validation failed", map[string]any{
				"order_id": order.ID,
				"error":    err.Error(),
			})
			return c.JSON(403, map[string]string{"error": "Additional validation required"})
		}
	}

	// Save to database
	if err := h.saveOrder(c.Request().Context(), order); err != nil {
		// EMERGENCY level - system is unusable
		span.LogEvent(tracing.EMERGENCY, "Database write failed - system critical", map[string]any{
			"error":     err.Error(),
			"order_id":  order.ID,
			"action":    "manual_intervention_required",
		})
		return c.JSON(503, map[string]string{"error": "System unavailable"})
	}

	// Cache the order
	cacheSpan := h.services.tracer.StartSpan("CacheOrder",
		tracing.WithParent(span),
		tracing.WithTags(map[string]interface{}{
			"cache.system": "redis",
			"cache.operation": "set",
		}),
	)
	
	h.cacheOrder(c.Request().Context(), order)
	cacheSpan.End()

	// Success
	span.LogEvent(tracing.INFO, "Order created successfully", map[string]any{
		"order_id": order.ID,
		"total":    order.Total,
	})

	return c.JSON(201, order)
}

func (h *OrderHandler) checkInventory(ctx context.Context, items []Item) []string {
	var unavailable []string
	
	for _, item := range items {
		// Simulate inventory check with Redis
		key := fmt.Sprintf("inventory:%s", item.ItemID)
		stock, err := h.services.redis.Get(ctx, key).Int()
		
		if err != nil || stock < item.Quantity {
			unavailable = append(unavailable, item.ItemID)
		}
	}
	
	return unavailable
}

func (h *OrderHandler) calculateTotal(ctx context.Context, items []Item) (float64, error) {
	var total float64
	
	// Simulate database query
	query := `SELECT SUM(price * $1) FROM items WHERE item_id = ANY($2)`
	
	for _, item := range items {
		// Simulate price lookup
		var price float64
		err := h.services.db.QueryRowContext(ctx, 
			"SELECT price FROM items WHERE item_id = $1", 
			item.ItemID,
		).Scan(&price)
		
		if err != nil {
			return 0, err
		}
		
		total += price * float64(item.Quantity)
	}
	
	// Add tax
	total *= 1.08
	
	return total, nil
}

func (h *OrderHandler) performHighValueValidation(ctx context.Context, order *Order) error {
	// Simulate additional validation
	time.Sleep(100 * time.Millisecond)
	
	if rand.Float32() < 0.1 {
		return fmt.Errorf("fraud detection triggered")
	}
	
	return nil
}

func (h *OrderHandler) saveOrder(ctx context.Context, order *Order) error {
	// Simulate database save
	_, err := h.services.db.ExecContext(ctx, `
		INSERT INTO orders (id, user_id, total, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, order.ID, order.UserID, order.Total, order.Status, order.CreatedAt)
	
	return err
}

func (h *OrderHandler) cacheOrder(ctx context.Context, order *Order) {
	key := fmt.Sprintf("order:%s", order.ID)
	h.services.redis.Set(ctx, key, order.Total, 5*time.Minute)
}

// InventoryHandler handles inventory checks
type InventoryHandler struct {
	services *Services
}

func (h *InventoryHandler) GET(c httpctx.Context) error {
	itemID := c.Param("item_id")
	
	span := c.Span()
	if span != nil {
		span.AddTags(map[string]interface{}{
			"item_id": itemID,
			"operation": "inventory_check",
		})
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("inventory:%s", itemID)
	stock, err := h.services.redis.Get(c.Request().Context(), cacheKey).Int()
	
	if err == redis.Nil {
		// Cache miss - check database
		if span != nil {
			span.LogEvent(tracing.DEBUG, "Cache miss", map[string]any{
				"cache_key": cacheKey,
			})
		}
		
		err = h.services.db.QueryRowContext(c.Request().Context(),
			"SELECT stock FROM inventory WHERE item_id = $1",
			itemID,
		).Scan(&stock)
		
		if err != nil {
			if span != nil {
				span.SetError(err)
			}
			return c.JSON(404, map[string]string{"error": "Item not found"})
		}
		
		// Update cache
		h.services.redis.Set(c.Request().Context(), cacheKey, stock, 1*time.Hour)
	}
	
	return c.JSON(200, map[string]interface{}{
		"item_id": itemID,
		"stock":   stock,
		"cached":  err != redis.Nil,
	})
}

// AnalyticsHandler demonstrates cross-service tracing
type AnalyticsHandler struct {
	services *Services
}

func (h *AnalyticsHandler) Sales(c httpctx.Context) error {
	span := c.Span()
	if span == nil {
		span = h.services.tracer.StartSpan("AnalyticsSales")
		defer span.End()
	}
	
	// Simulate multiple service calls
	results := make(chan map[string]interface{}, 3)
	errors := make(chan error, 3)
	
	// Parallel queries with separate spans
	go func() {
		childSpan := h.services.tracer.StartSpan("GetDailySales",
			tracing.WithParent(span))
		defer childSpan.End()
		
		sales, err := h.getDailySales(c.Request().Context())
		if err != nil {
			childSpan.SetError(err)
			errors <- err
			return
		}
		results <- map[string]interface{}{"daily_sales": sales}
	}()
	
	go func() {
		childSpan := h.services.tracer.StartSpan("GetTopProducts",
			tracing.WithParent(span))
		defer childSpan.End()
		
		products, err := h.getTopProducts(c.Request().Context())
		if err != nil {
			childSpan.SetError(err)
			errors <- err
			return
		}
		results <- map[string]interface{}{"top_products": products}
	}()
	
	go func() {
		childSpan := h.services.tracer.StartSpan("GetUserMetrics",
			tracing.WithParent(span))
		defer childSpan.End()
		
		metrics, err := h.getUserMetrics(c.Request().Context())
		if err != nil {
			childSpan.SetError(err)
			errors <- err
			return
		}
		results <- map[string]interface{}{"user_metrics": metrics}
	}()
	
	// Collect results
	analytics := make(map[string]interface{})
	for i := 0; i < 3; i++ {
		select {
		case result := <-results:
			for k, v := range result {
				analytics[k] = v
			}
		case err := <-errors:
			span.LogEvent(tracing.WARN, "Analytics query failed", map[string]any{
				"error": err.Error(),
			})
		case <-time.After(5 * time.Second):
			span.LogEvent(tracing.ERROR, "Analytics query timeout", nil)
		}
	}
	
	return c.JSON(200, analytics)
}

func (h *AnalyticsHandler) getDailySales(ctx context.Context) (float64, error) {
	var total float64
	err := h.services.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(total), 0) FROM orders WHERE created_at > NOW() - INTERVAL '24 hours'",
	).Scan(&total)
	return total, err
}

func (h *AnalyticsHandler) getTopProducts(ctx context.Context) ([]string, error) {
	// Simulate getting top products
	time.Sleep(50 * time.Millisecond)
	return []string{"product_1", "product_2", "product_3"}, nil
}

func (h *AnalyticsHandler) getUserMetrics(ctx context.Context) (map[string]int, error) {
	// Simulate user metrics
	time.Sleep(30 * time.Millisecond)
	return map[string]int{
		"active_users": 1234,
		"new_users":    56,
	}, nil
}

// HealthHandler provides health checks
type HealthHandler struct {
	services *Services
}

func (h *HealthHandler) GET(c httpctx.Context) error {
	span := c.Span()
	if span != nil {
		span.AddTags(map[string]interface{}{
			"check": "health",
		})
	}
	
	health := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Unix(),
	}
	
	// Check database
	if err := h.services.db.PingContext(c.Request().Context()); err != nil {
		health["database"] = "unhealthy"
		health["status"] = "degraded"
		if span != nil {
			span.LogEvent(tracing.WARN, "Database unhealthy", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		health["database"] = "healthy"
	}
	
	// Check Redis
	if err := h.services.redis.Ping(c.Request().Context()).Err(); err != nil {
		health["cache"] = "unhealthy"
		health["status"] = "degraded"
		if span != nil {
			span.LogEvent(tracing.WARN, "Cache unhealthy", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		health["cache"] = "healthy"
	}
	
	status := 200
	if health["status"] != "healthy" {
		status = 503
	}
	
	return c.JSON(status, health)
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Server.Address = ":8083"
	cfg.Logger.Level = "debug"

	// Initialize tracer (would use Jaeger in production)
	tracer := tracing.NewSimpleTracer("advanced-tracing-example")
	
	// In production, use OpenTelemetry adapter:
	// tracer, _ = otel.NewOTelTracerAdapter(
	//     otel.WithEndpoint("http://jaeger:14268/api/traces"),
	// )

	// Initialize database (PostgreSQL)
	db, err := sql.Open("postgres", "postgres://user:password@localhost/orders?sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	// Create services
	services := &Services{
		db:     db,
		redis:  rdb,
		tracer: tracer,
		logger: logger,
	}

	// Create handlers
	handlers := &HandlersManager{
		Home:      &HomeHandler{services: services},
		Orders:    &OrderHandler{services: services},
		Inventory: &InventoryHandler{services: services},
		Analytics: &AnalyticsHandler{services: services},
		Health:    &HealthHandler{services: services},
	}

	// Create app with tracing
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
		app.WithTracer(tracer),
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}

	logger.Info("Starting Advanced Tracing Example",
		zap.String("address", cfg.Server.Address))
	logger.Info("Endpoints:",
		zap.String("home", "GET /"),
		zap.String("create_order", "POST /orders"),
		zap.String("check_inventory", "GET /inventory/:item_id"),
		zap.String("analytics", "GET /analytics/sales"),
		zap.String("health", "GET /health"),
	)
	logger.Info("Tracing features demonstrated:",
		zap.String("severity_levels", "All 8 levels (DEBUG to EMERGENCY)"),
		zap.String("distributed", "Parent-child span relationships"),
		zap.String("external_systems", "Database and cache tracing"),
		zap.String("error_tracking", "Error propagation and logging"),
	)

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}