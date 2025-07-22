package main

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/yshengliao/gortex/pkg/errors"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

// UserHandler demonstrates the usage of the standardized error response system
type UserHandler struct {
	logger *zap.Logger
}

// User model
type User struct {
	ID       int     `json:"id"`
	Email    string  `json:"email"`
	Name     string  `json:"name"`
	Balance  float64 `json:"balance"`
	IsActive bool    `json:"is_active"`
}

// Mock database
var users = map[int]*User{
	1: {ID: 1, Email: "admin@example.com", Name: "Admin User", Balance: 1000, IsActive: true},
	2: {ID: 2, Email: "test@example.com", Name: "Test User", Balance: 500, IsActive: true},
	3: {ID: 3, Email: "inactive@example.com", Name: "Inactive User", Balance: 0, IsActive: false},
}

// GetUser demonstrates resource not found error
func (h *UserHandler) GetUser(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return errors.ValidationFieldError(c, "id", "Invalid user ID format")
	}

	user, exists := users[id]
	if !exists {
		return errors.NotFoundError(c, "User")
	}

	return response.Success(c, http.StatusOK, user)
}

// CreateUser demonstrates validation errors
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req struct {
		Email    string  `json:"email"`
		Name     string  `json:"name"`
		Password string  `json:"password"`
		Age      int     `json:"age"`
		Balance  float64 `json:"balance"`
	}

	if err := c.Bind(&req); err != nil {
		return errors.SendError(c, errors.CodeInvalidJSON, "Invalid request body", nil)
	}

	// Validation errors
	validationErrors := make(map[string]interface{})

	if req.Email == "" {
		validationErrors["email"] = "Email is required"
	}
	if req.Name == "" {
		validationErrors["name"] = "Name is required"
	}
	if len(req.Password) < 8 {
		validationErrors["password"] = "Password must be at least 8 characters"
	}
	if req.Age < 18 || req.Age > 120 {
		validationErrors["age"] = "Age must be between 18 and 120"
	}
	if req.Balance < 0 {
		validationErrors["balance"] = "Balance cannot be negative"
	}

	if len(validationErrors) > 0 {
		return errors.ValidationError(c, "Validation failed", validationErrors)
	}

	// Check if email already exists
	for _, user := range users {
		if user.Email == req.Email {
			return errors.ConflictError(c, "user", "Email already exists")
		}
	}

	// Create user
	newUser := &User{
		ID:       len(users) + 1,
		Email:    req.Email,
		Name:     req.Name,
		Balance:  req.Balance,
		IsActive: true,
	}
	users[newUser.ID] = newUser

	return response.Created(c, newUser)
}

// TransferMoney demonstrates business logic errors
func (h *UserHandler) TransferMoney(c echo.Context) error {
	var req struct {
		FromID int     `json:"from_id"`
		ToID   int     `json:"to_id"`
		Amount float64 `json:"amount"`
	}

	if err := c.Bind(&req); err != nil {
		return errors.BadRequest(c, "Invalid request")
	}

	// Validate amount
	if req.Amount <= 0 {
		return errors.SendError(c, errors.CodeInvalidInput, "Amount must be positive", map[string]interface{}{
			"field": "amount",
			"value": req.Amount,
		})
	}

	// Check if users exist
	fromUser, exists := users[req.FromID]
	if !exists {
		return errors.NotFoundError(c, "Sender user")
	}

	toUser, exists := users[req.ToID]
	if !exists {
		return errors.NotFoundError(c, "Recipient user")
	}

	// Check if sender is active
	if !fromUser.IsActive {
		return errors.New(errors.CodeAccountLocked, "Sender account is locked").
			WithDetail("user_id", fromUser.ID).
			Send(c, http.StatusForbidden)
	}

	// Check balance
	if fromUser.Balance < req.Amount {
		return errors.New(errors.CodeInsufficientBalance, "Insufficient balance").
			WithDetail("current_balance", fromUser.Balance).
			WithDetail("requested_amount", req.Amount).
			WithDetail("shortage", req.Amount-fromUser.Balance).
			Send(c, http.StatusPaymentRequired)
	}

	// Perform transfer
	fromUser.Balance -= req.Amount
	toUser.Balance += req.Amount

	return response.Success(c, http.StatusOK, map[string]interface{}{
		"transaction_id": "tx-" + strconv.FormatInt(int64(req.FromID*1000+req.ToID), 36),
		"from_balance":   fromUser.Balance,
		"to_balance":     toUser.Balance,
		"amount":         req.Amount,
	})
}

// ProtectedEndpoint demonstrates authentication errors
func (h *UserHandler) ProtectedEndpoint(c echo.Context) error {
	// Check auth header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return errors.UnauthorizedError(c, "Authorization header required")
	}

	// Simulate token validation
	if authHeader != "Bearer valid-token" {
		return errors.New(errors.CodeTokenInvalid, "Invalid authentication token").
			WithDetail("token_type", "Bearer").
			Send(c, http.StatusUnauthorized)
	}

	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Access granted to protected resource",
	})
}

// TriggerSystemError demonstrates system errors
func (h *UserHandler) TriggerSystemError(c echo.Context) error {
	errorType := c.QueryParam("type")

	switch errorType {
	case "timeout":
		return errors.TimeoutError(c, "database query")
	case "database":
		return errors.DatabaseError(c, "fetch user data")
	case "rate-limit":
		return errors.RateLimitError(c, 60)
	case "internal":
		// Simulate an unexpected error
		return errors.InternalServerError(c, errors.New(errors.CodeInternalServerError, "Unexpected error occurred"))
	default:
		return response.Success(c, http.StatusOK, map[string]interface{}{
			"message": "No error triggered. Use ?type=timeout|database|rate-limit|internal",
		})
	}
}

// ErrorHandlingMiddleware demonstrates how to use the error system in middleware
func ErrorHandlingMiddleware(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err != nil {
				// Check if it's already an ErrorResponse
				if errResp, ok := err.(*errors.ErrorResponse); ok {
					// Log the error
					logger.Error("Request failed",
						zap.Int("code", errResp.ErrorDetail.Code),
						zap.String("message", errResp.ErrorDetail.Message),
						zap.String("request_id", errResp.RequestID),
					)
					// Error is already formatted, just return it
					return err
				}

				// Check if it's an Echo HTTP error
				if he, ok := err.(*echo.HTTPError); ok {
					code := errors.CodeInternalServerError
					switch he.Code {
					case http.StatusBadRequest:
						code = errors.CodeInvalidInput
					case http.StatusUnauthorized:
						code = errors.CodeUnauthorized
					case http.StatusNotFound:
						code = errors.CodeResourceNotFound
					}
					return errors.SendError(c, code, he.Message.(string), nil)
				}

				// Generic error
				logger.Error("Unhandled error", zap.Error(err))
				return errors.InternalServerError(c, err)
			}
			return nil
		}
	}
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Add middleware
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(ErrorHandlingMiddleware(logger))

	// Initialize handler
	handler := &UserHandler{logger: logger}

	// Routes
	e.GET("/users/:id", handler.GetUser)
	e.POST("/users", handler.CreateUser)
	e.POST("/transfer", handler.TransferMoney)
	e.GET("/protected", handler.ProtectedEndpoint)
	e.GET("/error", handler.TriggerSystemError)

	// Start server
	logger.Info("Starting error example server on :8080")
	logger.Info("Try these endpoints:")
	logger.Info("  GET  /users/1           - Get existing user")
	logger.Info("  GET  /users/999         - Get non-existent user (404)")
	logger.Info("  POST /users             - Create user (validation errors)")
	logger.Info("  POST /transfer          - Transfer money (business errors)")
	logger.Info("  GET  /protected         - Protected endpoint (auth errors)")
	logger.Info("  GET  /error?type=xxx    - Trigger various system errors")

	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}