// Package errors provides a standardized error response system for the Gortex framework
package errors

// ErrorCode represents a standardized error code
type ErrorCode int

// Error code categories:
// 1xxx - Validation errors
// 2xxx - Authentication/Authorization errors
// 3xxx - System errors
// 4xxx - Business logic errors
const (
	// Validation errors (1xxx)
	CodeValidationFailed      ErrorCode = 1000
	CodeInvalidInput          ErrorCode = 1001
	CodeMissingRequiredField  ErrorCode = 1002
	CodeInvalidFormat         ErrorCode = 1003
	CodeValueOutOfRange       ErrorCode = 1004
	CodeDuplicateValue        ErrorCode = 1005
	CodeInvalidLength         ErrorCode = 1006
	CodeInvalidType           ErrorCode = 1007
	CodeInvalidJSON           ErrorCode = 1008
	CodeInvalidQueryParam     ErrorCode = 1009

	// Authentication/Authorization errors (2xxx)
	CodeUnauthorized          ErrorCode = 2000
	CodeInvalidCredentials    ErrorCode = 2001
	CodeTokenExpired          ErrorCode = 2002
	CodeTokenInvalid          ErrorCode = 2003
	CodeTokenMissing          ErrorCode = 2004
	CodeForbidden             ErrorCode = 2005
	CodeInsufficientPermissions ErrorCode = 2006
	CodeAccountLocked         ErrorCode = 2007
	CodeAccountNotFound       ErrorCode = 2008
	CodeSessionExpired        ErrorCode = 2009

	// System errors (3xxx)
	CodeInternalServerError   ErrorCode = 3000
	CodeDatabaseError         ErrorCode = 3001
	CodeServiceUnavailable    ErrorCode = 3002
	CodeTimeout               ErrorCode = 3003
	CodeRateLimitExceeded     ErrorCode = 3004
	CodeResourceExhausted     ErrorCode = 3005
	CodeNotImplemented        ErrorCode = 3006
	CodeBadGateway            ErrorCode = 3007
	CodeCircuitBreakerOpen    ErrorCode = 3008
	CodeConfigurationError    ErrorCode = 3009

	// Business logic errors (4xxx)
	CodeBusinessLogicError    ErrorCode = 4000
	CodeResourceNotFound      ErrorCode = 4001
	CodeResourceAlreadyExists ErrorCode = 4002
	CodeInvalidOperation      ErrorCode = 4003
	CodePreconditionFailed    ErrorCode = 4004
	CodeConflict              ErrorCode = 4005
	CodeInsufficientBalance   ErrorCode = 4006
	CodeQuotaExceeded         ErrorCode = 4007
	CodeInvalidState          ErrorCode = 4008
	CodeDependencyFailed      ErrorCode = 4009
	
	// Additional error codes
	CodeInvalidToken          ErrorCode = 2010
	CodeAccountSuspended      ErrorCode = 2011
	CodeNetworkError          ErrorCode = 3010
	CodeUnknownError          ErrorCode = 3011
	CodeMarshalError          ErrorCode = 3012
	CodeUnmarshalError        ErrorCode = 3013
	CodeFileSystemError       ErrorCode = 3014
	CodeThirdPartyServiceError ErrorCode = 3015
)

// errorMessages maps error codes to default messages
var errorMessages = map[ErrorCode]string{
	// Validation errors
	CodeValidationFailed:      "Validation failed",
	CodeInvalidInput:          "Invalid input provided",
	CodeMissingRequiredField:  "Required field is missing",
	CodeInvalidFormat:         "Invalid format",
	CodeValueOutOfRange:       "Value is out of acceptable range",
	CodeDuplicateValue:        "Duplicate value not allowed",
	CodeInvalidLength:         "Invalid length",
	CodeInvalidType:           "Invalid type",
	CodeInvalidJSON:           "Invalid JSON format",
	CodeInvalidQueryParam:     "Invalid query parameter",

	// Authentication/Authorization errors
	CodeUnauthorized:           "Unauthorized access",
	CodeInvalidCredentials:     "Invalid credentials",
	CodeTokenExpired:           "Token has expired",
	CodeTokenInvalid:           "Invalid token",
	CodeTokenMissing:           "Token is missing",
	CodeForbidden:              "Access forbidden",
	CodeInsufficientPermissions: "Insufficient permissions",
	CodeAccountLocked:          "Account is locked",
	CodeAccountNotFound:        "Account not found",
	CodeSessionExpired:         "Session has expired",

	// System errors
	CodeInternalServerError:    "Internal server error",
	CodeDatabaseError:          "Database error occurred",
	CodeServiceUnavailable:     "Service temporarily unavailable",
	CodeTimeout:                "Request timeout",
	CodeRateLimitExceeded:      "Rate limit exceeded",
	CodeResourceExhausted:      "Resource exhausted",
	CodeNotImplemented:         "Feature not implemented",
	CodeBadGateway:             "Bad gateway",
	CodeCircuitBreakerOpen:     "Circuit breaker is open",
	CodeConfigurationError:     "Configuration error",

	// Business logic errors
	CodeBusinessLogicError:     "Business logic error",
	CodeResourceNotFound:       "Resource not found",
	CodeResourceAlreadyExists:  "Resource already exists",
	CodeInvalidOperation:       "Invalid operation",
	CodePreconditionFailed:     "Precondition failed",
	CodeConflict:               "Resource conflict",
	CodeInsufficientBalance:    "Insufficient balance",
	CodeQuotaExceeded:          "Quota exceeded",
	CodeInvalidState:           "Invalid state",
	CodeDependencyFailed:       "Dependency failed",
	
	// Additional error messages
	CodeInvalidToken:           "Invalid token",
	CodeAccountSuspended:       "Account suspended",
	CodeNetworkError:           "Network error",
	CodeUnknownError:           "Unknown error",
	CodeMarshalError:           "Data marshaling error",
	CodeUnmarshalError:         "Data unmarshaling error",
	CodeFileSystemError:        "File system error",
	CodeThirdPartyServiceError: "Third party service error",
}

// Message returns the default message for an error code
func (e ErrorCode) Message() string {
	if msg, ok := errorMessages[e]; ok {
		return msg
	}
	return "Unknown error"
}

// Int returns the error code as an integer
func (e ErrorCode) Int() int {
	return int(e)
}

// String returns the error code as a string
func (e ErrorCode) String() string {
	if msg, ok := errorMessages[e]; ok {
		return msg
	}
	return "Unknown error"
}

// GetHTTPStatus returns the HTTP status code for an error code
func GetHTTPStatus(code ErrorCode) int {
	// Define the mapping from error codes to HTTP status codes
	switch code {
	// Validation errors -> 400
	case CodeValidationFailed, CodeInvalidInput, CodeMissingRequiredField,
		CodeInvalidFormat, CodeValueOutOfRange, CodeDuplicateValue,
		CodeInvalidLength, CodeInvalidType:
		return 400
		
	// Auth errors -> 401/403
	case CodeUnauthorized, CodeTokenExpired, CodeInvalidToken:
		return 401
	case CodeForbidden, CodeInsufficientPermissions, CodeAccountSuspended:
		return 403
		
	// Not found -> 404
	case CodeResourceNotFound:
		return 404
		
	// Conflict -> 409
	case CodeResourceAlreadyExists, CodeConflict:
		return 409
		
	// Precondition failed -> 412
	case CodePreconditionFailed:
		return 412
		
	// Rate limit -> 429
	case CodeRateLimitExceeded:
		return 429
		
	// Server errors -> 500/503
	case CodeInternalServerError, CodeDatabaseError, CodeNetworkError,
		CodeUnknownError, CodeMarshalError, CodeUnmarshalError,
		CodeFileSystemError:
		return 500
	case CodeTimeout:
		return 504
	case CodeThirdPartyServiceError, CodeDependencyFailed:
		return 503
		
	// Business logic errors -> depends on context
	case CodeBusinessLogicError, CodeInvalidOperation, CodeInvalidState,
		CodeQuotaExceeded:
		return 400
	case CodeInsufficientBalance:
		return 402 // Payment Required
		
	default:
		return 500
	}
}