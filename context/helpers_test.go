package context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParamInt(t *testing.T) {
	tests := []struct {
		name         string
		paramValue   string
		defaultValue int
		expected     int
	}{
		{"valid int", "123", 0, 123},
		{"empty param", "", 42, 42},
		{"invalid int", "abc", 10, 10},
		{"negative int", "-456", 0, -456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := NewContext(req, rec).(*DefaultContext)
			
			// Set up parameter
			c.paramNames = []string{"id"}
			c.paramValues = []string{tt.paramValue}
			
			result := c.ParamInt("id", tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		paramName    string
		defaultValue int
		expected     int
	}{
		{"valid int", "?page=5", "page", 1, 5},
		{"missing param", "?other=123", "page", 10, 10},
		{"invalid int", "?page=abc", "page", 1, 1},
		{"empty value", "?page=", "page", 7, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.queryString, nil)
			rec := httptest.NewRecorder()
			c := NewContext(req, rec).(*DefaultContext)
			
			result := c.QueryInt(tt.paramName, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryBool(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		paramName    string
		defaultValue bool
		expected     bool
	}{
		{"true value", "?active=true", "active", false, true},
		{"false value", "?active=false", "active", true, false},
		{"1 value", "?active=1", "active", false, true},
		{"0 value", "?active=0", "active", true, false},
		{"missing param", "?other=true", "active", true, true},
		{"invalid bool", "?active=yes", "active", false, false},
		{"empty value", "?active=", "active", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.queryString, nil)
			rec := httptest.NewRecorder()
			c := NewContext(req, rec).(*DefaultContext)
			
			result := c.QueryBool(tt.paramName, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseHelpers(t *testing.T) {
	t.Run("OK response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := NewContext(req, rec).(*DefaultContext)
		
		data := Map{"message": "success"}
		err := c.OK(data)
		
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"message":"success"`)
	})
	
	t.Run("Created response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		c := NewContext(req, rec).(*DefaultContext)
		
		data := Map{"id": 123}
		err := c.Created(data)
		
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), `"id":123`)
	})
	
	t.Run("NoContent204 response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		rec := httptest.NewRecorder()
		c := NewContext(req, rec).(*DefaultContext)
		
		err := c.NoContent204()
		
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})
	
	t.Run("BadRequest response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		c := NewContext(req, rec).(*DefaultContext)
		
		err := c.BadRequest("Invalid input")
		
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error":"Invalid input"`)
	})
}