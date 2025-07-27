package testutil

import (
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// TestHandler is a simple handler for testing
type TestHandler struct {
	Called      bool
	ReturnError error
	Logger      *zap.Logger
}

func (h *TestHandler) GET(c httpctx.Context) error {
	h.Called = true
	if h.ReturnError != nil {
		return h.ReturnError
	}
	return c.JSON(200, map[string]string{"method": "GET"})
}

func (h *TestHandler) POST(c httpctx.Context) error {
	h.Called = true
	if h.ReturnError != nil {
		return h.ReturnError
	}
	return c.JSON(201, map[string]string{"method": "POST"})
}

// EchoHandler echoes back request data for testing
type EchoHandler struct{}

func (h *EchoHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]interface{}{
		"method": "GET",
		"path":   c.Path(),
		"params": map[string]string{}, // TODO: Fix param values
		"query":  c.QueryParams(),
	})
}

func (h *EchoHandler) POST(c httpctx.Context) error {
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid JSON"})
	}

	return c.JSON(200, map[string]interface{}{
		"method": "POST",
		"path":   c.Path(),
		"body":   body,
	})
}

// ErrorHandler always returns an error for testing error handling
type ErrorHandler struct {
	ErrorCode    int
	ErrorMessage string
}

func (h *ErrorHandler) GET(c httpctx.Context) error {
	if h.ErrorCode == 0 {
		h.ErrorCode = 500
	}
	if h.ErrorMessage == "" {
		h.ErrorMessage = "Test error"
	}
	return c.JSON(h.ErrorCode, map[string]string{"error": h.ErrorMessage})
}

// PanicHandler panics for testing panic recovery
type PanicHandler struct {
	PanicMessage string
}

func (h *PanicHandler) GET(c httpctx.Context) error {
	if h.PanicMessage == "" {
		h.PanicMessage = "Test panic"
	}
	panic(h.PanicMessage)
}
