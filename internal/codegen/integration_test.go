package codegen

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	pkgerrors "github.com/yshengliao/gortex/pkg/errors"
)

// Example business errors
var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrPaymentFailed      = errors.New("payment processing failed")
)

// Example service
type OrderService struct{}

func (s *OrderService) GetOrder(id string) (*Order, error) {
	if id == "404" {
		return nil, ErrOrderNotFound
	}
	return &Order{ID: id, Status: "confirmed"}, nil
}

func (s *OrderService) CreateOrder(req *CreateOrderRequest) (*Order, error) {
	if req.Quantity > 100 {
		return nil, ErrInsufficientStock
	}
	if req.ProductID == "payment-fail" {
		return nil, ErrPaymentFailed
	}
	return &Order{ID: "123", Status: "pending"}, nil
}

type Order struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type CreateOrderRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

func TestErrorRegistryIntegration(t *testing.T) {
	// Setup error mappings
	pkgerrors.Register(ErrOrderNotFound, pkgerrors.CodeResourceNotFound, http.StatusNotFound, "Order not found")
	pkgerrors.Register(ErrInsufficientStock, pkgerrors.CodeInvalidOperation, http.StatusBadRequest, "Not enough stock available")
	pkgerrors.Register(ErrPaymentFailed, pkgerrors.CodeDependencyFailed, http.StatusBadGateway, "Payment service error")

	// Create Echo instance
	e := echo.New()

	// Simulate a generated handler
	service := &OrderService{}

	// GET /orders/:id handler
	e.GET("/orders/:id", func(c echo.Context) error {
		id := c.Param("id")
		
		result, err := service.GetOrder(id)
		if err != nil {
			// This is what the generated code would do
			httpStatus, errResp := pkgerrors.HandleBusinessError(err)
			if errResp != nil {
				return errResp.Send(c, httpStatus)
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    result,
		})
	})

	// POST /orders handler
	e.POST("/orders", func(c echo.Context) error {
		var req CreateOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		
		result, err := service.CreateOrder(&req)
		if err != nil {
			// This is what the generated code would do
			httpStatus, errResp := pkgerrors.HandleBusinessError(err)
			if errResp != nil {
				return errResp.Send(c, httpStatus)
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    result,
		})
	})

	t.Run("GET success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orders/123", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":true`)
		assert.Contains(t, rec.Body.String(), `"id":"123"`)
	})

	t.Run("GET with registered error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orders/404", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":false`)
		assert.Contains(t, rec.Body.String(), `"code":4001`)
		assert.Contains(t, rec.Body.String(), `"message":"Order not found"`)
	})

	t.Run("POST with insufficient stock error", func(t *testing.T) {
		body := strings.NewReader(`{"product_id":"test","quantity":200}`)
		req := httptest.NewRequest(http.MethodPost, "/orders", body)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":4003`)
		assert.Contains(t, rec.Body.String(), `"message":"Not enough stock available"`)
	})

	t.Run("POST with payment error", func(t *testing.T) {
		body := strings.NewReader(`{"product_id":"payment-fail","quantity":1}`)
		req := httptest.NewRequest(http.MethodPost, "/orders", body)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadGateway, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":4009`)
		assert.Contains(t, rec.Body.String(), `"message":"Payment service error"`)
	})
}