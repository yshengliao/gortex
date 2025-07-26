package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/context"
	"go.uber.org/zap"
)

// HandlersManager demonstrates struct tag routing
// The framework automatically discovers routes from struct tags
type HandlersManager struct {
	// Basic routes using url tags
	Home    *HomeHandler    `url:"/"`
	Health  *HealthHandler  `url:"/health"`
	
	// Dynamic parameter routes
	User    *UserHandler    `url:"/users/:id"`
	
	// Wildcard routes for static files
	Static  *StaticHandler  `url:"/static/*"`
	
	// Nested groups for API versioning
	API     *APIGroup       `url:"/api"`
	
	// Advanced struct tags demo (commented out - uncomment to use)
	// Protected routes with middleware
	// Admin   *AdminGroup     `url:"/admin" middleware:"auth"`
	
	// Rate-limited public API
	// Public  *PublicAPI      `url:"/public" ratelimit:"100/min"`
}

// APIGroup demonstrates nested routing groups
type APIGroup struct {
	// Nested groups create hierarchical routes
	V1 *APIv1Group `url:"/v1"`
	V2 *APIv2Group `url:"/v2"`
}

// APIv1Group contains v1 API endpoints
type APIv1Group struct {
	// Routes become /api/v1/users/:id
	Users   *UserAPIHandler   `url:"/users/:id"`
	// Routes become /api/v1/products/:id
	Products *ProductHandler  `url:"/products/:id"`
}

// APIv2Group contains v2 API endpoints
type APIv2Group struct {
	// Routes become /api/v2/users/:id
	Users   *UserAPIHandlerV2 `url:"/users/:id"`
}

// HomeHandler handles the root route
type HomeHandler struct{}

func (h *HomeHandler) GET(c context.Context) error {
	// Using the new OK helper method
	return c.OK(map[string]string{
		"message": "Welcome to Gortex",
		"version": "v0.4.0",
	})
}

// HealthHandler demonstrates simple health check
type HealthHandler struct{}

func (h *HealthHandler) GET(c context.Context) error {
	// Using the OK helper
	return c.OK(map[string]string{
		"status": "healthy",
	})
}

// UserHandler demonstrates dynamic parameters
type UserHandler struct{}

// GET /users/:id
func (h *UserHandler) GET(c context.Context) error {
	// Using the new helper method to get parameter as int
	id := c.ParamInt("id", 0)
	
	// Check for query parameters using helper methods
	page := c.QueryInt("page", 1)
	active := c.QueryBool("active", true)
	
	// Using the new OK helper method
	return c.OK(map[string]interface{}{
		"id":     id,
		"name":   fmt.Sprintf("User %d", id),
		"page":   page,
		"active": active,
	})
}

// POST /users/:id
func (h *UserHandler) POST(c context.Context) error {
	id := c.ParamInt("id", 0)
	
	// Using the new Created helper method
	return c.Created(map[string]interface{}{
		"message": "User created",
		"id":      id,
	})
}

// Profile creates a sub-route: POST /users/:id/profile
func (h *UserHandler) Profile(c context.Context) error {
	id := c.Param("id")
	return c.JSON(200, map[string]string{
		"userId":  id,
		"profile": "User profile data",
	})
}

// StaticHandler demonstrates wildcard routes
type StaticHandler struct{}

func (h *StaticHandler) GET(c context.Context) error {
	filepath := c.Param("*")
	return c.JSON(200, map[string]string{
		"file": filepath,
		"type": "static file",
	})
}

// UserAPIHandler for API v1
type UserAPIHandler struct{}

func (h *UserAPIHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{
		"version": "v1",
		"user":    c.Param("id"),
	})
}

// ProductHandler for API v1
type ProductHandler struct{}

func (h *ProductHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{
		"version": "v1",
		"product": c.Param("id"),
	})
}

// UserAPIHandlerV2 for API v2
type UserAPIHandlerV2 struct{}

func (h *UserAPIHandlerV2) GET(c context.Context) error {
	return c.JSON(200, map[string]string{
		"version": "v2",
		"user":    c.Param("id"),
		"features": "enhanced",
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create handlers structure
	// With auto-initialization, you don't need to manually initialize each handler!
	// The framework will automatically initialize all handlers for you.
	
	// Option 1: Let the framework auto-initialize everything
	handlers := &HandlersManager{} // All handlers will be auto-initialized!
	
	// Option 2: Mix manual and auto initialization
	// handlers := &HandlersManager{
	//     Home: &HomeHandler{}, // Manually initialized
	//     // Other handlers will be auto-initialized
	// }

	// Simple configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug"

	// Create and run application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithDevelopmentMode(), // Enable development mode with debug endpoints
		app.WithRoutesLogger(), // Enable automatic route logging
		app.WithHandlers(handlers),
	)
	if err != nil {
		log.Fatal(err)
	}

	logger.Info("Starting Gortex server", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Routes automatically discovered from struct tags!")

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}

// ============================================
// Advanced Struct Tags Examples (Gortex v0.3.0+)
// ============================================

// Example 1: Dependency Injection with inject tag
type DatabaseService struct {
	// Your database connection
}

type UserServiceHandler struct {
	DB *DatabaseService `inject:""` // Will be automatically injected from DI container
}

func (h *UserServiceHandler) GET(c context.Context) error {
	// h.DB is automatically injected
	return c.OK("User service with injected DB")
}

// Example 2: Middleware composition
type ProtectedHandlers struct {
	// Single middleware
	Profile *ProfileHandler `url:"/profile" middleware:"auth"`
	
	// Multiple middleware (executed in order)
	Admin *AdminHandler `url:"/admin" middleware:"auth,rbac,audit"`
}

type ProfileHandler struct{}
type AdminHandler struct{}

// Example 3: Rate limiting
type PublicAPI struct {
	Search *SearchHandler `url:"/search" ratelimit:"10/sec"`     // 10 requests per second
	Upload *UploadHandler `url:"/upload" ratelimit:"100/min"`    // 100 requests per minute
	Report *ReportHandler `url:"/report" ratelimit:"1000/hour"`  // 1000 requests per hour
}

type SearchHandler struct{}
type UploadHandler struct{}
type ReportHandler struct{}

// Example 4: Combined tags
type AdvancedAPI struct {
	// Protected endpoint with rate limiting
	Premium *PremiumHandler `url:"/premium" middleware:"auth" ratelimit:"50/min"`
	
	// Nested group with inherited middleware
	V3 *APIv3Group `url:"/v3" middleware:"auth,requestid"`
}

type PremiumHandler struct{}
type APIv3Group struct{}

// To use these advanced features:
// 1. Register services in DI container:
//    ctx := app.NewContext()
//    app.Register(ctx, &DatabaseService{})
//
// 2. Register middleware:
//    middlewareRegistry := make(map[string]middleware.MiddlewareFunc)
//    middlewareRegistry["auth"] = authMiddleware
//    app.Register(ctx, middlewareRegistry)
//
// 3. Use the handlers:
//    handlers := &AdvancedHandlers{}
//    app.RegisterRoutesFromStruct(router, handlers, ctx)