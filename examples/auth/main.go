package main

import (
	"log"
	"net/http"
	"time"

	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/pkg/auth"
	"github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/pkg/errors"
	"go.uber.org/zap"
)

// HandlersManager demonstrates routing with authentication
type HandlersManager struct {
	// Public endpoints
	Home  *HomeHandler  `url:"/"`
	Auth  *AuthHandler  `url:"/auth"`
	
	// Protected endpoints with middleware
	User  *UserHandler  `url:"/user" middleware:"auth"`
	Admin *AdminGroup   `url:"/admin" middleware:"auth,admin"`
}

// AdminGroup demonstrates protected nested routes
type AdminGroup struct {
	Dashboard *DashboardHandler `url:"/dashboard"`
	Users     *UsersHandler     `url:"/users/:id"`
}

// HomeHandler - public endpoint
type HomeHandler struct{}

func (h *HomeHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Welcome to Gortex Auth Example",
		"login":   "POST /auth/login",
		"profile": "GET /user/profile (requires auth)",
	})
}

// AuthHandler handles authentication
type AuthHandler struct {
	jwt *auth.JWTService
}

// Login endpoint: POST /auth/login
func (h *AuthHandler) Login(c context.Context) error {
	var req struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	
	if err := c.Bind(&req); err != nil {
		return errors.ValidationError(c, "Invalid request", nil)
	}
	
	// Simple authentication (in real app, check database)
	if req.Username != "admin" || req.Password != "secret" {
		return errors.UnauthorizedError(c, "Invalid credentials")
	}
	
	// Generate JWT token
	token, err := h.jwt.GenerateAccessToken(
		"user-123",    // UserID
		req.Username,  // Username
		"admin@example.com", // Email
		"admin",       // Role
	)
	if err != nil {
		err := errors.New(errors.CodeInternalServerError, "Failed to generate token")
		return err.Send(c, http.StatusInternalServerError)
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
func (h *UserHandler) Profile(c context.Context) error {
	// Get user claims from JWT (set by auth middleware)
	claims := auth.GetClaims(c)
	if claims != nil {
		return c.JSON(200, map[string]interface{}{
			"message": "This is your profile",
			"user": map[string]string{
				"id":       claims.UserID,
				"username": claims.Username,
				"email":    claims.Email,
				"role":     claims.Role,
			},
		})
	}
	
	return c.JSON(200, map[string]interface{}{
		"message": "This is a protected endpoint",
		"note": "JWT claims would be available here in production",
	})
}

// DashboardHandler - admin only
type DashboardHandler struct{}

func (h *DashboardHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Admin Dashboard",
		"access":  "admin only",
	})
}

// UsersHandler - admin only
type UsersHandler struct{}

func (h *UsersHandler) GET(c context.Context) error {
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

	// Configure application
	cfg := &app.Config{}
	cfg.Server.Address = ":8081"
	cfg.Logger.Level = "debug"

	// Create application first
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}
	
	// Get application context and register middleware
	ctx := application.Context()
	
	// Create middleware registry
	middlewareRegistry := make(map[string]middleware.MiddlewareFunc)
	middlewareRegistry["auth"] = auth.Middleware(jwtService)
	middlewareRegistry["admin"] = AdminMiddleware()
	
	// Register middleware registry in context
	app.Register(ctx, middlewareRegistry)
	
	// Now register handlers with middleware support
	if err := app.RegisterRoutes(application, handlers); err != nil {
		log.Fatal("Failed to register routes:", err)
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

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}

// AdminMiddleware checks if user has admin role
func AdminMiddleware() middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			// Check if user has admin role
			claims := auth.GetClaims(c)
			if claims == nil || claims.Role != "admin" {
				return errors.ForbiddenError(c, "Admin access required")
			}
			return next(c)
		}
	}
}