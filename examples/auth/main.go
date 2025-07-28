package main

import (
	"log"
	"time"

	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/pkg/auth"
	"github.com/yshengliao/gortex/pkg/errors"
	"go.uber.org/zap"
)

// HandlersManager is the root handler struct with struct tags
type HandlersManager struct {
	Home  *HomeHandler  `url:"/"`
	Auth  *AuthHandler  `url:"/auth"`
	User  *UserHandler  `url:"/user"`
	Admin *AdminGroup   `url:"/admin"`
}

// HomeHandler handles the home page
type HomeHandler struct{}

// GET /
func (h *HomeHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Welcome to Gortex Auth Example",
		"login":   "POST /auth/login with {username: 'admin', password: 'secret'}",
		"profile": "GET /user/profile (requires auth token)",
		"admin":   "GET /admin/dashboard (requires admin role)",
	})
}

// AuthHandler handles authentication
type AuthHandler struct {
	jwt *auth.JWTService
}

// Login endpoint: POST /auth/login
func (h *AuthHandler) Login(c httpctx.Context) error {
	var req struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request"})
	}
	
	// Simple authentication (in real app, check database)
	if req.Username != "admin" || req.Password != "secret" {
		return c.JSON(401, errors.NewFromCode(errors.CodeUnauthorized).WithDetail("reason", "Invalid credentials"))
	}
	
	// Generate JWT token
	token, err := h.jwt.GenerateAccessToken(
		"user-123",    // UserID
		req.Username,  // Username
		"admin@example.com", // Email
		"admin",       // Role
	)
	if err != nil {
		return c.JSON(500, errors.New(errors.CodeInternalServerError, "Failed to generate token"))
	}
	
	return c.JSON(200, map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       "user-123",
			"username": req.Username,
			"role":     "admin",
		},
	})
}

// UserHandler - requires authentication
type UserHandler struct{}

// Profile endpoint: GET /user/profile
func (h *UserHandler) Profile(c httpctx.Context) error {
	// Get JWT token from Authorization header
	token := c.Request().Header.Get("Authorization")
	if token == "" {
		return c.JSON(401, map[string]string{"error": "No authorization token"})
	}
	
	return c.JSON(200, map[string]interface{}{
		"message": "This is your profile",
		"user": map[string]string{
			"id":       "user-123",
			"username": "admin",
			"email":    "admin@example.com",
			"role":     "admin",
		},
	})
}

// AdminGroup contains admin-only handlers
type AdminGroup struct {
	Dashboard *DashboardHandler `url:"/dashboard"`
	Users     *UsersHandler     `url:"/users/:id"`
}

// DashboardHandler - admin only
type DashboardHandler struct{}

func (h *DashboardHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Admin Dashboard",
		"access":  "admin only",
	})
}

// UsersHandler - admin only
type UsersHandler struct{}

func (h *UsersHandler) GET(c httpctx.Context) error {
	id := c.Param("id")
	return c.JSON(200, map[string]string{
		"message": "Admin viewing user",
		"userId":  id,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create JWT service
	jwtService := auth.NewJWTService(
		"my-secret-key",     // Secret key
		24*time.Hour,        // Access token TTL
		7*24*time.Hour,      // Refresh token TTL
		"gortex-auth",       // Issuer
	)

	// Create handlers
	handlers := &HandlersManager{
		Home: &HomeHandler{},
		Auth: &AuthHandler{jwt: jwtService},
		User: &UserHandler{},
		Admin: &AdminGroup{
			Dashboard: &DashboardHandler{},
			Users:     &UsersHandler{},
		},
	}

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Server.Address = ":8081"
	cfg.Logger.Level = "debug"
	
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}

	logger.Info("Starting Gortex Auth Example", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Routes:",
		zap.String("public", "GET /, POST /auth/login"),
		zap.String("protected", "GET /user/profile (requires token)"),
		zap.String("admin", "GET /admin/dashboard, GET /admin/users/:id (requires admin role)"),
	)
	logger.Info("Test credentials:", 
		zap.String("username", "admin"),
		zap.String("password", "secret"),
	)

	if err := application.Run(); err != nil {
		logger.Fatal("Server error", zap.Error(err))
	}
}

