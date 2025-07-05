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
	"github.com/yshengliao/gortex/validation"
)

// Request/Response structures

type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Handlers

type HandlersManager struct {
	Auth    *AuthHandler    `url:"/auth"`
	Profile *ProfileHandler `url:"/profile"`
}

type AuthHandler struct {
	JWTService *auth.JWTService
	Logger     *zap.Logger
}

// POST /auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := validation.BindAndValidate(c, &req); err != nil {
		return err
	}

	// Simple authentication (in production, check against database)
	if req.Username != "admin" || req.Password != "password" {
		return echo.NewHTTPError(401, "Invalid credentials")
	}

	// Generate tokens
	accessToken, err := h.JWTService.GenerateAccessToken(
		"user-123",
		req.Username,
		"admin@example.com",
		"admin",
	)
	if err != nil {
		h.Logger.Error("Failed to generate access token", zap.Error(err))
		return echo.NewHTTPError(500, "Failed to generate token")
	}

	refreshToken, err := h.JWTService.GenerateRefreshToken("user-123")
	if err != nil {
		h.Logger.Error("Failed to generate refresh token", zap.Error(err))
		return echo.NewHTTPError(500, "Failed to generate token")
	}

	return c.JSON(200, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	})
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := validation.BindAndValidate(c, &req); err != nil {
		return err
	}

	// Refresh the token
	newAccessToken, err := h.JWTService.RefreshAccessToken(req.RefreshToken, func(userID string) (string, string, string, error) {
		// In production, fetch user details from database
		return "admin", "admin@example.com", "admin", nil
	})

	if err != nil {
		return echo.NewHTTPError(401, "Invalid refresh token")
	}

	return c.JSON(200, map[string]interface{}{
		"access_token": newAccessToken,
		"expires_in":   3600,
	})
}

type ProfileHandler struct {
	Logger *zap.Logger
}

// GET /profile (requires authentication)
func (h *ProfileHandler) GET(c echo.Context) error {
	claims := auth.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(401, "Unauthorized")
	}

	return c.JSON(200, map[string]interface{}{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"email":    claims.Email,
		"role":     claims.Role,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Server.Recovery = true

	// Create JWT service
	jwtService := auth.NewJWTService(
		"your-secret-key",
		time.Hour,           // Access token TTL
		24*time.Hour*7,      // Refresh token TTL
		"stmp-auth-example",
	)

	// Create validator
	validator := validation.NewValidator()

	// Create handlers
	handlers := &HandlersManager{
		Auth: &AuthHandler{
			JWTService: jwtService,
			Logger:     logger,
		},
		Profile: &ProfileHandler{
			Logger: logger,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Set custom validator
	application.Echo().Validator = validator

	// Register services
	app.Register(application.Context(), logger)
	app.Register(application.Context(), jwtService)

	// Apply authentication middleware to profile routes
	e := application.Echo()
	profileGroup := e.Group("/profile")
	profileGroup.Use(auth.Middleware(jwtService))

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting authentication example server", zap.String("address", cfg.Server.Address))
		logger.Info("Try: curl -X POST http://localhost:8080/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"password\"}'")
		
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt
	<-ctx.Done()

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}
}