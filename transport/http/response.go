// Package http provides standardized HTTP response helpers
package http

import (
	"net/http"
	"time"

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
func Success(c Context, statusCode int, data any) error {
	return c.JSON(statusCode, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMeta sends a successful response with metadata
func SuccessWithMeta(c Context, statusCode int, data any, meta map[string]any) error {
	resp := &SuccessResponse{
		Success:   true,
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now().UTC(),
		RequestID: errors.GetRequestID(c),
	}
	return c.JSON(statusCode, resp)
}


// Created sends a 201 Created response
func Created(c Context, data any) error {
	return Success(c, http.StatusCreated, data)
}

// NoContent sends a 204 No Content response
func NoContent(c Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Accepted sends a 202 Accepted response
func Accepted(c Context, data any) error {
	return Success(c, http.StatusAccepted, data)
}