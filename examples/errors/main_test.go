package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/errors"
	"go.uber.org/zap"
)

func setupTestEcho() *echo.Echo {
	e := echo.New()
	logger, _ := zap.NewDevelopment()
	handler := &UserHandler{logger: logger}

	// Routes
	e.GET("/users/:id", handler.GetUser)
	e.POST("/users", handler.CreateUser)
	e.POST("/transfer", handler.TransferMoney)
	e.GET("/protected", handler.ProtectedEndpoint)
	e.GET("/error", handler.TriggerSystemError)

	return e
}

func TestGetUser(t *testing.T) {
	e := setupTestEcho()

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		checkError     bool
		errorCode      int
	}{
		{
			name:           "Get existing user",
			userID:         "1",
			expectedStatus: http.StatusOK,
			checkError:     false,
		},
		{
			name:           "Get non-existent user",
			userID:         "999",
			expectedStatus: http.StatusNotFound,
			checkError:     true,
			errorCode:      errors.CodeResourceNotFound.Int(),
		},
		{
			name:           "Invalid user ID",
			userID:         "abc",
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
			errorCode:      errors.CodeInvalidInput.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users/"+tt.userID, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.checkError {
				var errResp errors.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if errResp.ErrorDetail.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, errResp.ErrorDetail.Code)
				}
			}
		})
	}
}

func TestCreateUser(t *testing.T) {
	e := setupTestEcho()

	tests := []struct {
		name           string
		payload        interface{}
		expectedStatus int
		checkError     bool
		errorCode      int
	}{
		{
			name: "Valid user creation",
			payload: map[string]interface{}{
				"email":    "new@example.com",
				"name":     "New User",
				"password": "password123",
				"age":      25,
				"balance":  100,
			},
			expectedStatus: http.StatusCreated,
			checkError:     false,
		},
		{
			name: "Missing required fields",
			payload: map[string]interface{}{
				"email": "",
				"name":  "",
			},
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
			errorCode:      errors.CodeValidationFailed.Int(),
		},
		{
			name: "Duplicate email",
			payload: map[string]interface{}{
				"email":    "admin@example.com",
				"name":     "Duplicate User",
				"password": "password123",
				"age":      25,
				"balance":  100,
			},
			expectedStatus: http.StatusConflict,
			checkError:     true,
			errorCode:      errors.CodeConflict.Int(),
		},
		{
			name:           "Invalid JSON",
			payload:        "invalid json",
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
			errorCode:      errors.CodeInvalidJSON.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if str, ok := tt.payload.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.payload)
			}

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.checkError {
				var errResp errors.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if errResp.ErrorDetail.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, errResp.ErrorDetail.Code)
				}
			}
		})
	}
}

func TestTransferMoney(t *testing.T) {
	e := setupTestEcho()

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		checkError     bool
		errorCode      int
	}{
		{
			name: "Valid transfer",
			payload: map[string]interface{}{
				"from_id": 1,
				"to_id":   2,
				"amount":  100,
			},
			expectedStatus: http.StatusOK,
			checkError:     false,
		},
		{
			name: "Insufficient balance",
			payload: map[string]interface{}{
				"from_id": 1,
				"to_id":   2,
				"amount":  10000,
			},
			expectedStatus: http.StatusPaymentRequired,
			checkError:     true,
			errorCode:      errors.CodeInsufficientBalance.Int(),
		},
		{
			name: "Inactive sender",
			payload: map[string]interface{}{
				"from_id": 3,
				"to_id":   1,
				"amount":  10,
			},
			expectedStatus: http.StatusForbidden,
			checkError:     true,
			errorCode:      errors.CodeAccountLocked.Int(),
		},
		{
			name: "Invalid amount",
			payload: map[string]interface{}{
				"from_id": 1,
				"to_id":   2,
				"amount":  -100,
			},
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
			errorCode:      errors.CodeInvalidInput.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.checkError {
				var errResp errors.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if errResp.ErrorDetail.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, errResp.ErrorDetail.Code)
				}
			}
		})
	}
}

func TestProtectedEndpoint(t *testing.T) {
	e := setupTestEcho()

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		checkError     bool
		errorCode      int
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer valid-token",
			expectedStatus: http.StatusOK,
			checkError:     false,
		},
		{
			name:           "Missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkError:     true,
			errorCode:      errors.CodeUnauthorized.Int(),
		},
		{
			name:           "Invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			checkError:     true,
			errorCode:      errors.CodeTokenInvalid.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.checkError {
				var errResp errors.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if errResp.ErrorDetail.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, errResp.ErrorDetail.Code)
				}
			}
		})
	}
}

func TestTriggerSystemError(t *testing.T) {
	e := setupTestEcho()

	tests := []struct {
		name           string
		errorType      string
		expectedStatus int
		errorCode      int
	}{
		{
			name:           "Timeout error",
			errorType:      "timeout",
			expectedStatus: http.StatusRequestTimeout,
			errorCode:      errors.CodeTimeout.Int(),
		},
		{
			name:           "Database error",
			errorType:      "database",
			expectedStatus: http.StatusInternalServerError,
			errorCode:      errors.CodeDatabaseError.Int(),
		},
		{
			name:           "Rate limit error",
			errorType:      "rate-limit",
			expectedStatus: http.StatusTooManyRequests,
			errorCode:      errors.CodeRateLimitExceeded.Int(),
		},
		{
			name:           "Internal error",
			errorType:      "internal",
			expectedStatus: http.StatusInternalServerError,
			errorCode:      errors.CodeInternalServerError.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/error?type="+tt.errorType, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var errResp errors.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("Failed to unmarshal error response: %v", err)
			}
			if errResp.ErrorDetail.Code != tt.errorCode {
				t.Errorf("Expected error code %d, got %d", tt.errorCode, errResp.ErrorDetail.Code)
			}
		})
	}
}

func BenchmarkErrorResponse(b *testing.B) {
	e := setupTestEcho()

	b.Run("NotFoundError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, "/users/999", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})

	b.Run("ValidationError", func(b *testing.B) {
		payload := map[string]interface{}{
			"email": "",
			"name":  "",
		}
		body, _ := json.Marshal(payload)

		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}