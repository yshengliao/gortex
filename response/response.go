// Package response provides standardized HTTP response helpers
package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

// Success sends a successful response
func Success(c echo.Context, statusCode int, data interface{}) error {
	return c.JSON(statusCode, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// Error sends an error response
func Error(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, StandardResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	})
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c echo.Context, message string) error {
	return Error(c, http.StatusBadRequest, message)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c echo.Context, message string) error {
	return Error(c, http.StatusUnauthorized, message)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c echo.Context, message string) error {
	return Error(c, http.StatusForbidden, message)
}

// NotFound sends a 404 Not Found response
func NotFound(c echo.Context, message string) error {
	return Error(c, http.StatusNotFound, message)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c echo.Context, message string) error {
	return Error(c, http.StatusInternalServerError, message)
}