package errors

import (
	"time"

	"github.com/labstack/echo/v4"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Success      bool                   `json:"success"`
	ErrorDetail  ErrorDetail            `json:"error"`
	Timestamp    time.Time              `json:"timestamp"`
	RequestID    string                 `json:"request_id,omitempty"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
}

// ErrorDetail contains detailed error information
type ErrorDetail struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	return e.ErrorDetail.Message
}

// GetRequestID extracts request ID from Echo context
func GetRequestID(c echo.Context) string {
	// First try to get from context value (set by our middleware)
	if reqID, ok := c.Get("request_id").(string); ok && reqID != "" {
		return reqID
	}
	// Try to get from response header (set by middleware)
	if reqID := c.Response().Header().Get(echo.HeaderXRequestID); reqID != "" {
		return reqID
	}
	// Try to get from request header
	if reqID := c.Request().Header.Get(echo.HeaderXRequestID); reqID != "" {
		return reqID
	}
	return ""
}

// New creates a new error response with the given code and message
func New(code ErrorCode, message string) *ErrorResponse {
	return &ErrorResponse{
		Success: false,
		ErrorDetail: ErrorDetail{
			Code:    code.Int(),
			Message: message,
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewWithDetails creates a new error response with additional details
func NewWithDetails(code ErrorCode, message string, details map[string]interface{}) *ErrorResponse {
	return &ErrorResponse{
		Success: false,
		ErrorDetail: ErrorDetail{
			Code:    code.Int(),
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewFromCode creates a new error response using the default message for the code
func NewFromCode(code ErrorCode) *ErrorResponse {
	return New(code, code.Message())
}

// WithRequestID adds request ID to the error response
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

// WithMeta adds metadata to the error response
func (e *ErrorResponse) WithMeta(meta map[string]interface{}) *ErrorResponse {
	e.Meta = meta
	return e
}

// WithDetail adds a single detail to the error response
func (e *ErrorResponse) WithDetail(key string, value interface{}) *ErrorResponse {
	if e.ErrorDetail.Details == nil {
		e.ErrorDetail.Details = make(map[string]interface{})
	}
	e.ErrorDetail.Details[key] = value
	return e
}

// WithDetails adds multiple details to the error response
func (e *ErrorResponse) WithDetails(details map[string]interface{}) *ErrorResponse {
	e.ErrorDetail.Details = details
	return e
}

// Send sends the error response via Echo context
func (e *ErrorResponse) Send(c echo.Context, httpStatus int) error {
	// Auto-populate request ID if not set
	if e.RequestID == "" {
		e.RequestID = GetRequestID(c)
	}
	return c.JSON(httpStatus, e)
}