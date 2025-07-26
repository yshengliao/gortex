package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/context"
)

// Test structures for parameter binding
type SimpleParams struct {
	ID   int    `json:"id" bind:"id,path"`
	Name string `json:"name" bind:"name,query"`
}

type ComplexParams struct {
	UserID      int       `json:"user_id" bind:"user_id,path"`
	GameID      string    `json:"game_id" bind:"game_id,path"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Timestamp   time.Time `json:"timestamp"`
	IsActive    bool      `json:"is_active" bind:"active,query"`
	PageSize    int       `json:"page_size" bind:"page_size,query"`
	AuthToken   string    `json:"-" bind:"Authorization,header"`
	Description string    `json:"description"`
}

type AutoBindParams struct {
	ID          int    `json:"id"`    // Should auto-bind from path
	Search      string `json:"search"` // Should auto-bind from query
	Name        string `json:"name"`   // Should auto-bind from JSON body
	NotBound    string `json:"-"`      // Should not be bound
}

// Test handler for binding tests
type TestBindingHandler struct {
	binder *ParameterBinder
}

func (h *TestBindingHandler) SimpleMethod(c context.Context, params *SimpleParams) error {
	return c.JSON(200, params)
}

func (h *TestBindingHandler) ComplexMethod(c context.Context, params *ComplexParams) error {
	return c.JSON(200, params)
}

func (h *TestBindingHandler) PrimitiveMethod(c context.Context, id int) error {
	return c.JSON(200, map[string]int{"id": id})
}

func (h *TestBindingHandler) MultipleParams(c context.Context, id int, params *SimpleParams) error {
	return c.JSON(200, map[string]any{
		"id":     id,
		"params": params,
	})
}

func TestParameterBinderSimple(t *testing.T) {
	binder := NewParameterBinder()
	handler := &TestBindingHandler{binder: binder}

	// Test simple parameter binding
	req := httptest.NewRequest(http.MethodGet, "/users/123?name=test", nil)
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("123")

	handlerType := reflect.TypeOf(handler)
	method, _ := handlerType.MethodByName("SimpleMethod")
	params, err := binder.BindMethodParams(ctx, method)
	require.NoError(t, err)
	require.Len(t, params, 2) // context.Context + *SimpleParams

	// Check bound values
	simpleParams := params[1].Interface().(*SimpleParams)
	assert.Equal(t, 123, simpleParams.ID)
	assert.Equal(t, "test", simpleParams.Name)
}

func TestParameterBinderComplex(t *testing.T) {
	
	binder := NewParameterBinder()
	handler := &TestBindingHandler{binder: binder}

	// Create request body
	body := map[string]any{
		"amount":      100.50,
		"currency":    "USD",
		"timestamp":   "2023-01-01T00:00:00Z",
		"description": "Test transaction",
	}
	bodyBytes, _ := json.Marshal(body)

	// Test complex parameter binding
	req := httptest.NewRequest(http.MethodPost, "/users/456/games/ABC?active=true&page_size=20", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	ctx.SetParamNames("user_id", "game_id")
	ctx.SetParamValues("456", "ABC")

	handlerType := reflect.TypeOf(handler)
	method, _ := handlerType.MethodByName("ComplexMethod")
	params, err := binder.BindMethodParams(ctx, method)
	require.NoError(t, err)
	require.Len(t, params, 2) // context.Context + *ComplexParams

	// Check bound values
	complexParams := params[1].Interface().(*ComplexParams)
	assert.Equal(t, 456, complexParams.UserID)
	assert.Equal(t, "ABC", complexParams.GameID)
	assert.Equal(t, 100.50, complexParams.Amount)
	assert.Equal(t, "USD", complexParams.Currency)
	assert.Equal(t, true, complexParams.IsActive)
	assert.Equal(t, 20, complexParams.PageSize)
	assert.Equal(t, "Bearer token123", complexParams.AuthToken)
	assert.Equal(t, "Test transaction", complexParams.Description)
	assert.Equal(t, "2023-01-01T00:00:00Z", complexParams.Timestamp.Format(time.RFC3339))
}

func TestParameterBinderPrimitive(t *testing.T) {
	
	binder := NewParameterBinder()
	handler := &TestBindingHandler{binder: binder}

	// Test primitive parameter binding
	req := httptest.NewRequest(http.MethodGet, "/items/789", nil)
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("789")

	handlerType := reflect.TypeOf(handler)
	method, _ := handlerType.MethodByName("PrimitiveMethod")
	params, err := binder.BindMethodParams(ctx, method)
	require.NoError(t, err)
	require.Len(t, params, 2) // context.Context + int

	// Check bound value
	id := params[1].Interface().(int)
	assert.Equal(t, 789, id)
}

func TestParameterBinderAutoDetection(t *testing.T) {
	
	binder := NewParameterBinder()

	// Create request with mixed parameters
	body := map[string]any{
		"name": "John Doe",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/users/999?search=keyword", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("999")

	// Create a method-like structure for testing
	params := &AutoBindParams{}
	paramValue := reflect.ValueOf(params)
	
	err := binder.bindParameter(ctx, paramValue)
	require.NoError(t, err)

	// Check auto-detected bindings
	assert.Equal(t, 999, params.ID)         // From path
	assert.Equal(t, "keyword", params.Search) // From query
	assert.Equal(t, "John Doe", params.Name)  // From JSON body
	assert.Equal(t, "", params.NotBound)      // Should remain empty
}

func TestParameterBinderEdgeCases(t *testing.T) {
	
	binder := NewParameterBinder()

	t.Run("empty request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		params := &SimpleParams{}
		paramValue := reflect.ValueOf(params)
		
		err := binder.bindParameter(ctx, paramValue)
		require.NoError(t, err)
		
		// Should have zero values
		assert.Equal(t, 0, params.ID)
		assert.Equal(t, "", params.Name)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		params := &SimpleParams{}
		paramValue := reflect.ValueOf(params)
		
		err := binder.bindParameter(ctx, paramValue)
		require.NoError(t, err) // Should not fail, just skip JSON binding
	})

	t.Run("type conversion errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/abc?page_size=xyz", nil)
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)
		ctx.SetParamNames("user_id")
		ctx.SetParamValues("abc")

		params := &ComplexParams{}
		paramValue := reflect.ValueOf(params)
		
		err := binder.bindParameter(ctx, paramValue)
		require.NoError(t, err)
		
		// Invalid conversions should result in zero values
		assert.Equal(t, 0, params.UserID)
		assert.Equal(t, 0, params.PageSize)
	})
}

func TestParameterBinderTimeFormats(t *testing.T) {
	
	binder := NewParameterBinder()

	type TimeParams struct {
		CreatedAt time.Time `json:"created_at" bind:"created_at,query"`
		UpdatedAt time.Time `json:"updated_at" bind:"updated_at,query"`
		DeletedAt time.Time `json:"deleted_at" bind:"deleted_at,query"`
	}

	testCases := []struct {
		name     string
		query    string
		expected []string // expected time formats
	}{
		{
			name:     "RFC3339",
			query:    "?created_at=2023-01-01T12:00:00Z",
			expected: []string{"2023-01-01T12:00:00Z"},
		},
		{
			name:     "DateTime without timezone",
			query:    "?updated_at=2023-01-01T12:00:00",
			expected: []string{"2023-01-01T12:00:00Z"},
		},
		{
			name:     "Date only",
			query:    "?deleted_at=2023-01-01",
			expected: []string{"2023-01-01T00:00:00Z"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tc.query, nil)
			rec := httptest.NewRecorder()
			ctx := context.NewContext(req, rec)

			params := &TimeParams{}
			paramValue := reflect.ValueOf(params)
			
			err := binder.bindParameter(ctx, paramValue)
			require.NoError(t, err)
			
			// At least one time field should be set
			hasTime := !params.CreatedAt.IsZero() || !params.UpdatedAt.IsZero() || !params.DeletedAt.IsZero()
			assert.True(t, hasTime, "At least one time field should be parsed")
		})
	}
}

func TestParameterBinderFieldTypes(t *testing.T) {
	
	binder := NewParameterBinder()

	type AllTypes struct {
		String   string        `bind:"string,query"`
		Int      int           `bind:"int,query"`
		Int8     int8          `bind:"int8,query"`
		Int16    int16         `bind:"int16,query"`
		Int32    int32         `bind:"int32,query"`
		Int64    int64         `bind:"int64,query"`
		Uint     uint          `bind:"uint,query"`
		Uint8    uint8         `bind:"uint8,query"`
		Uint16   uint16        `bind:"uint16,query"`
		Uint32   uint32        `bind:"uint32,query"`
		Uint64   uint64        `bind:"uint64,query"`
		Float32  float32       `bind:"float32,query"`
		Float64  float64       `bind:"float64,query"`
		Bool     bool          `bind:"bool,query"`
		Duration time.Duration `bind:"duration,query"`
	}

	query := "?string=test&int=10&int8=8&int16=16&int32=32&int64=64" +
		"&uint=10&uint8=8&uint16=16&uint32=32&uint64=64" +
		"&float32=3.14&float64=3.14159&bool=true&duration=5m"

	req := httptest.NewRequest(http.MethodGet, "/"+query, nil)
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)

	params := &AllTypes{}
	paramValue := reflect.ValueOf(params)
	
	err := binder.bindParameter(ctx, paramValue)
	require.NoError(t, err)

	assert.Equal(t, "test", params.String)
	assert.Equal(t, 10, params.Int)
	assert.Equal(t, int8(8), params.Int8)
	assert.Equal(t, int16(16), params.Int16)
	assert.Equal(t, int32(32), params.Int32)
	assert.Equal(t, int64(64), params.Int64)
	assert.Equal(t, uint(10), params.Uint)
	assert.Equal(t, uint8(8), params.Uint8)
	assert.Equal(t, uint16(16), params.Uint16)
	assert.Equal(t, uint32(32), params.Uint32)
	assert.Equal(t, uint64(64), params.Uint64)
	assert.Equal(t, float32(3.14), params.Float32)
	assert.Equal(t, 3.14159, params.Float64)
	assert.Equal(t, true, params.Bool)
	assert.Equal(t, 5*time.Minute, params.Duration)
}