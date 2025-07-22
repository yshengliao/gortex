package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/auth"
)

// HandlersManager defines all handlers with declarative routing
type HandlersManager struct {
	Auth    *AuthHandler    `url:"/auth"`
	Profile *ProfileHandler `url:"/profile"`
}

// AuthHandler demonstrates JWT authentication
type AuthHandler struct {
	JWTService *auth.JWTService
}

// POST /auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	// Simple authentication example
	accessToken, _ := h.JWTService.GenerateAccessToken("user-123", "admin", "admin@example.com", "admin")
	refreshToken, _ := h.JWTService.GenerateRefreshToken("user-123")
	
	return c.JSON(200, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// ProfileHandler demonstrates protected endpoints
type ProfileHandler struct{}

// GET /profile (requires authentication)
func (h *ProfileHandler) GET(c echo.Context) error {
	claims := auth.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(401, "Unauthorized")
	}
	
	return c.JSON(200, map[string]any{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create JWT service
	jwtService := auth.NewJWTService("secret-key", time.Hour, 24*time.Hour*7, "gortex-auth")

	// Create handlers
	handlers := &HandlersManager{
		Auth:    &AuthHandler{JWTService: jwtService},
		Profile: &ProfileHandler{},
	}

	// Create application with functional options
	cfg := &app.Config{}
	cfg.Server.Address = ":8081"
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Apply authentication middleware to profile routes
	e := application.Echo()
	profileGroup := e.Group("/profile")
	profileGroup.Use(auth.Middleware(jwtService))

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting auth server on :8081")
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}
}