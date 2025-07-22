package errors

import (
	"testing"
)

func TestErrorCode_Message(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected string
	}{
		// Validation errors
		{"ValidationFailed", CodeValidationFailed, "Validation failed"},
		{"InvalidInput", CodeInvalidInput, "Invalid input provided"},
		{"MissingRequiredField", CodeMissingRequiredField, "Required field is missing"},
		
		// Auth errors
		{"Unauthorized", CodeUnauthorized, "Unauthorized access"},
		{"TokenExpired", CodeTokenExpired, "Token has expired"},
		{"Forbidden", CodeForbidden, "Access forbidden"},
		
		// System errors
		{"InternalServerError", CodeInternalServerError, "Internal server error"},
		{"DatabaseError", CodeDatabaseError, "Database error occurred"},
		{"RateLimitExceeded", CodeRateLimitExceeded, "Rate limit exceeded"},
		
		// Business errors
		{"ResourceNotFound", CodeResourceNotFound, "Resource not found"},
		{"Conflict", CodeConflict, "Resource conflict"},
		{"InsufficientBalance", CodeInsufficientBalance, "Insufficient balance"},
		
		// Unknown code
		{"Unknown", ErrorCode(9999), "Unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.Message(); got != tt.expected {
				t.Errorf("ErrorCode.Message() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCode_Int(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		{"ValidationFailed", CodeValidationFailed, 1000},
		{"Unauthorized", CodeUnauthorized, 2000},
		{"InternalServerError", CodeInternalServerError, 3000},
		{"BusinessLogicError", CodeBusinessLogicError, 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.Int(); got != tt.expected {
				t.Errorf("ErrorCode.Int() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected string
	}{
		{"ValidationFailed", CodeValidationFailed, "Validation failed"},
		{"Unauthorized", CodeUnauthorized, "Unauthorized access"},
		{"Unknown", ErrorCode(9999), "Unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("ErrorCode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCodeCategories(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		category string
		minCode  int
		maxCode  int
	}{
		{"Validation", CodeValidationFailed, "Validation", 1000, 1999},
		{"Auth", CodeUnauthorized, "Auth", 2000, 2999},
		{"System", CodeInternalServerError, "System", 3000, 3999},
		{"Business", CodeBusinessLogicError, "Business", 4000, 4999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codeInt := tt.code.Int()
			if codeInt < tt.minCode || codeInt > tt.maxCode {
				t.Errorf("%s code %d is not in expected range [%d, %d]", 
					tt.category, codeInt, tt.minCode, tt.maxCode)
			}
		})
	}
}