// Package response provides standardized HTTP response helpers
package response

import (
	"net/http"
	"time"

	"github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/pkg/errors"
)

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    any `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

// SuccessResponse represents a standardized success response
type SuccessResponse struct {
	Success   bool                   `json:"success"`
	Data      any                    `json:"data,omitempty"`
	Meta      map[string]any         `json:"meta,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
}

// Success sends a successful response
func Success(c context.Context, statusCode int, data any) error {
	return c.JSON(statusCode, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMeta sends a successful response with metadata
func SuccessWithMeta(c context.Context, statusCode int, data any, meta map[string]any) error {
	resp := &SuccessResponse{
		Success:   true,
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now().UTC(),
		RequestID: errors.GetRequestID(c),
	}
	return c.JSON(statusCode, resp)
}

// Error sends an error response (deprecated - use pkg/errors instead)
// Kept for backward compatibility
func Error(c context.Context, statusCode int, message string) error {
	return c.JSON(statusCode, StandardResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	})
}

// BadRequest sends a 400 Bad Request response (deprecated - use errors.ValidationError)
func BadRequest(c context.Context, message string) error {
	return errors.ValidationError(c, message, nil)
}

// Unauthorized sends a 401 Unauthorized response (deprecated - use errors.UnauthorizedError)
func Unauthorized(c context.Context, message string) error {
	return errors.UnauthorizedError(c, message)
}

// Forbidden sends a 403 Forbidden response (deprecated - use errors.ForbiddenError)
func Forbidden(c context.Context, message string) error {
	return errors.ForbiddenError(c, message)
}

// NotFound sends a 404 Not Found response (deprecated - use errors.NotFoundError)
func NotFound(c context.Context, message string) error {
	return errors.NotFoundError(c, message)
}

// InternalServerError sends a 500 Internal Server Error response (deprecated - use errors.InternalServerError)
func InternalServerError(c context.Context, message string) error {
	err := errors.New(errors.CodeInternalServerError, message)
	return err.Send(c, http.StatusInternalServerError)
}

// Created sends a 201 Created response
func Created(c context.Context, data any) error {
	return Success(c, http.StatusCreated, data)
}

// NoContent sends a 204 No Content response
func NoContent(c context.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Accepted sends a 202 Accepted response
func Accepted(c context.Context, data any) error {
	return Success(c, http.StatusAccepted, data)
}