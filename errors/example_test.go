package errors_test

import (
	"fmt"
	"net/http"

	"github.com/yshengliao/gortex/http/context"
	"github.com/yshengliao/gortex/errors"
	"github.com/yshengliao/gortex/http/response"
)

// Example handler showing various error response patterns
type UserHandler struct{}

// Example: Basic validation error
func (h *UserHandler) CreateUser(c context.Context) error {
	var req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	
	if err := c.Bind(&req); err != nil {
		// Invalid JSON format
		return errors.SendError(c, errors.CodeInvalidJSON, "Invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}
	
	// Field-specific validation error
	if req.Email == "" {
		return errors.ValidationFieldError(c, "email", "Email is required")
	}
	
	// Multiple validation errors
	validationErrors := make(map[string]interface{})
	if len(req.Password) < 8 {
		validationErrors["password"] = "Password must be at least 8 characters"
	}
	if len(validationErrors) > 0 {
		return errors.ValidationError(c, "Validation failed", validationErrors)
	}
	
	// Success response
	return response.Created(c, map[string]interface{}{
		"id":    "user-123",
		"email": req.Email,
	})
}

// Example: Authentication errors
func (h *UserHandler) Login(c context.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	
	if err := c.Bind(&req); err != nil {
		return errors.BadRequest(c, "Invalid request")
	}
	
	// Simulate user lookup
	user, err := h.findUserByEmail(req.Email)
	if err != nil {
		// Don't reveal if user exists or not for security
		return errors.UnauthorizedError(c, "Invalid credentials")
	}
	
	// Check if account is locked
	if user.IsLocked {
		return errors.New(errors.CodeAccountLocked, "Account has been locked due to multiple failed login attempts").
			WithDetail("locked_until", user.LockedUntil).
			Send(c, http.StatusForbidden)
	}
	
	// Verify password
	if !h.verifyPassword(user.Password, req.Password) {
		return errors.UnauthorizedError(c, "Invalid credentials")
	}
	
	// Generate token
	token, err := h.generateToken(user.ID)
	if err != nil {
		return errors.InternalServerError(c, err)
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Example: Resource not found
func (h *UserHandler) GetUser(c context.Context) error {
	userID := c.Param("id")
	
	user, err := h.findUserByID(userID)
	if err != nil {
		return errors.NotFoundError(c, "User")
	}
	
	return response.Success(c, http.StatusOK, user)
}

// Example: Business logic errors
func (h *UserHandler) TransferCredits(c context.Context) error {
	var req struct {
		FromUserID string  `json:"from_user_id"`
		ToUserID   string  `json:"to_user_id"`
		Amount     float64 `json:"amount"`
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
	
	// Check sender balance
	sender, _ := h.findUserByID(req.FromUserID)
	if sender.Balance < req.Amount {
		return errors.New(errors.CodeInsufficientBalance, "Insufficient balance for transfer").
			WithDetail("current_balance", sender.Balance).
			WithDetail("requested_amount", req.Amount).
			WithDetail("shortage", req.Amount-sender.Balance).
			Send(c, http.StatusPaymentRequired)
	}
	
	// Check daily transfer limit
	dailyTotal := h.getDailyTransferTotal(req.FromUserID)
	if dailyTotal+req.Amount > 10000 {
		return errors.New(errors.CodeQuotaExceeded, "Daily transfer limit exceeded").
			WithDetail("daily_limit", 10000).
			WithDetail("current_total", dailyTotal).
			WithDetail("requested", req.Amount).
			Send(c, http.StatusPaymentRequired)
	}
	
	// Perform transfer...
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"transaction_id": "tx-123",
		"amount":         req.Amount,
		"new_balance":    sender.Balance - req.Amount,
	})
}

// Example: Rate limiting
func (h *UserHandler) SendVerificationEmail(c context.Context) error {
	userIDValue := c.Get("user_id")
	if userIDValue == nil {
		return errors.UnauthorizedError(c, "User not authenticated")
	}
	
	userID := userIDValue.(string)
	
	// Check rate limit
	if h.isRateLimited(userID) {
		return errors.RateLimitError(c, 300) // 5 minutes
	}
	
	// Send email...
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Verification email sent",
	})
}

// Example: System errors with proper error handling
func (h *UserHandler) UpdateProfile(c context.Context) error {
	userID := c.Param("id")
	
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return errors.BadRequest(c, "Invalid request body")
	}
	
	// Database operation with timeout
	ctx := c.Request().Context()
	err := h.updateUserWithTimeout(ctx, userID, updates)
	
	if err != nil {
		switch err.Error() {
		case "context deadline exceeded":
			return errors.TimeoutError(c, "database update")
		case "connection refused":
			return errors.DatabaseError(c, "update user profile")
		default:
			// Log the actual error internally
			fmt.Printf("UpdateProfile error: %v\n", err)
			// Return generic error to client
			return errors.InternalServerError(c, err)
		}
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Profile updated successfully",
	})
}

// Example: Conflict errors
func (h *UserHandler) ChangeEmail(c context.Context) error {
	userID := c.Param("id")
	
	var req struct {
		NewEmail string `json:"new_email"`
	}
	
	if err := c.Bind(&req); err != nil {
		return errors.BadRequest(c, "Invalid request")
	}
	
	// Check if email already exists
	if h.emailExists(req.NewEmail) {
		return errors.ConflictError(c, "email", "Email address is already in use")
	}
	
	// Update email for user...
	fmt.Printf("Updating email for user %s\n", userID) // Using userID
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Email updated successfully",
	})
}

// Example: Custom error codes
func (h *UserHandler) ActivateFeature(c context.Context) error {
	userID := c.Param("id")
	feature := c.Param("feature")
	
	user, _ := h.findUserByID(userID)
	
	// Check subscription level
	if user.SubscriptionLevel < 2 && feature == "advanced-analytics" {
		// Create a custom detailed error response
		err := errors.New(errors.CodeInsufficientPermissions, 
			"This feature requires a Pro subscription").
			WithDetail("required_level", "Pro").
			WithDetail("current_level", "Basic").
			WithDetail("upgrade_url", "/subscription/upgrade").
			WithMeta(map[string]interface{}{
				"feature":    feature,
				"user_level": user.SubscriptionLevel,
			})
		
		return err.Send(c, http.StatusForbidden)
	}
	
	// Activate feature...
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"feature":  feature,
		"activated": true,
	})
}

// Helper methods (mock implementations)
func (h *UserHandler) findUserByEmail(email string) (*User, error) {
	// Mock implementation
	if email == "test@example.com" {
		return &User{ID: "user-123", Email: email}, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (h *UserHandler) findUserByID(id string) (*User, error) {
	// Mock implementation
	return &User{ID: id, Balance: 1000, SubscriptionLevel: 1}, nil
}

func (h *UserHandler) verifyPassword(hash, password string) bool {
	// Mock implementation
	return password == "correct-password"
}

func (h *UserHandler) generateToken(userID string) (string, error) {
	// Mock implementation
	return "jwt-token-here", nil
}

func (h *UserHandler) getDailyTransferTotal(userID string) float64 {
	// Mock implementation
	return 5000
}

func (h *UserHandler) isRateLimited(userID string) bool {
	// Mock implementation
	return false
}

func (h *UserHandler) updateUserWithTimeout(ctx interface{}, userID string, updates map[string]interface{}) error {
	// Mock implementation
	return nil
}

func (h *UserHandler) emailExists(email string) bool {
	// Mock implementation
	return email == "existing@example.com"
}

type User struct {
	ID                string
	Email             string
	Password          string
	Balance           float64
	IsLocked          bool
	LockedUntil       string
	SubscriptionLevel int
}

// Example: Error handling in middleware
func ErrorHandlingMiddleware() context.MiddlewareFunc {
	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(c context.Context) error {
			err := next(c)
			if err != nil {
				// Check if it's already an ErrorResponse
				if errResp, ok := err.(*errors.ErrorResponse); ok {
					// Already formatted, just send it
					return errResp.Send(c, http.StatusInternalServerError)
				}
				
				// Check if it's a Gortex HTTP error
				if he, ok := err.(*context.HTTPError); ok {
					code := errors.CodeInternalServerError
					switch he.Code {
					case http.StatusBadRequest:
						code = errors.CodeInvalidInput
					case http.StatusUnauthorized:
						code = errors.CodeUnauthorized
					case http.StatusForbidden:
						code = errors.CodeForbidden
					case http.StatusNotFound:
						code = errors.CodeResourceNotFound
					}
					
					message := fmt.Sprintf("%v", he.Message)
					return errors.SendError(c, code, message, nil)
				}
				
				// Generic error
				return errors.InternalServerError(c, err)
			}
			return nil
		}
	}
}