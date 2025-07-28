package main

import (
	"log"

	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/core/app/doc"
	"github.com/yshengliao/gortex/core/app/doc/swagger"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// Handlers demonstrates API documentation generation
type Handlers struct {
	Users    *UserHandler    `url:"/users" api:"group=User Management,version=v1,desc=User-related operations,tags=users|admin"`
	Products *ProductHandler `url:"/products" api:"group=Product Catalog,version=v1,desc=Product management,tags=products"`
}

// UserHandler handles user-related operations
type UserHandler struct{}

// GET retrieves all users
func (h *UserHandler) GET(c httpctx.Context) error {
	users := []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com"},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
	}
	return c.JSON(200, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// POST creates a new user
func (h *UserHandler) POST(c httpctx.Context) error {
	var user struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	
	if err := c.Bind(&user); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request body"})
	}
	
	return c.JSON(201, map[string]interface{}{
		"id":    3,
		"name":  user.Name,
		"email": user.Email,
	})
}

// Profile retrieves user profile (custom method)
func (h *UserHandler) Profile(c httpctx.Context) error {
	userID := c.Param("id")
	return c.JSON(200, map[string]interface{}{
		"id":       userID,
		"name":     "John Doe",
		"email":    "john@example.com",
		"joinDate": "2024-01-15",
	})
}

// ProductHandler handles product-related operations
type ProductHandler struct{}

// GET retrieves all products
func (h *ProductHandler) GET(c httpctx.Context) error {
	products := []map[string]interface{}{
		{"id": 1, "name": "Laptop", "price": 999.99, "category": "Electronics"},
		{"id": 2, "name": "Coffee Mug", "price": 12.99, "category": "Kitchen"},
	}
	return c.JSON(200, map[string]interface{}{
		"products": products,
		"total":    len(products),
	})
}

// Search searches for products (custom method)
func (h *ProductHandler) Search(c httpctx.Context) error {
	query := c.QueryParam("q")
	category := c.QueryParam("category")
	
	return c.JSON(200, map[string]interface{}{
		"query":    query,
		"category": category,
		"results":  []map[string]interface{}{},
	})
}

func main() {
	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &config.Config{}
	cfg.Server.Address = ":8083"
	cfg.Logger.Level = "debug"

	// Create documentation configuration
	docConfig := &doc.DocConfig{
		Title:       "Gortex API Documentation Example",
		Version:     "1.0.0",
		Description: "This example demonstrates automatic API documentation generation in Gortex",
		Servers: []doc.ServerInfo{
			{
				URL:         "http://localhost:8083",
				Description: "Local development server",
			},
		},
		Contact: &doc.ContactInfo{
			Name:  "Gortex Team",
			Email: "support@gortex.example.com",
			URL:   "https://github.com/yshengliao/gortex",
		},
		License: &doc.LicenseInfo{
			Name: "MIT",
			URL:  "https://opensource.org/licenses/MIT",
		},
	}

	// Create Swagger documentation provider
	docProvider := swagger.NewSwaggerProvider(docConfig)

	// Create handlers
	handlers := &Handlers{
		Users:    &UserHandler{},
		Products: &ProductHandler{},
	}

	// Create app with documentation
	gortexApp, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithDocProvider(docProvider),
		app.WithHandlers(handlers),
		app.WithRoutesLogger(),
		app.WithDevelopmentMode(),
	)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	logger.Info("Starting API documentation example",
		zap.String("address", cfg.Server.Address),
		zap.String("docs", "http://localhost:8083/docs"),
		zap.String("swagger_json", "http://localhost:8083/docs/swagger.json"),
	)

	// Print instructions
	log.Println("\n🚀 API Documentation Example Started!")
	log.Println("📄 View API documentation at: http://localhost:8083/docs")
	log.Println("📋 View Swagger JSON at: http://localhost:8083/docs/swagger.json")
	log.Println("🔍 View all routes at: http://localhost:8083/_routes")
	log.Println("\nTest endpoints:")
	log.Println("  GET  http://localhost:8083/users")
	log.Println("  POST http://localhost:8083/users")
	log.Println("  POST http://localhost:8083/users/profile")
	log.Println("  GET  http://localhost:8083/products")
	log.Println("  POST http://localhost:8083/products/search")

	// Run the app
	if err := gortexApp.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}