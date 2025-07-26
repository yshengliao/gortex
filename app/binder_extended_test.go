package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yshengliao/gortex/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Test structures for extended binding features
type UserService struct {
	Name string
}

type AuthRequest struct {
	Username string `json:"username" validate:"required,min=3"`
	Password string `json:"password" validate:"required,min=6"`
}

type ProfileRequest struct {
	UserID   string `json:"user_id" bind:"user_id,jwt"`
	Username string `json:"username" bind:"username,jwt"`
	Email    string `json:"email" bind:"email,jwt"`
	Role     string `json:"role" bind:"role,context"`
}

type ServiceRequest struct {
	Name string `json:"name" validate:"required"`
}

// Test handler with extended features
type ExtendedHandler struct {
	Logger *zap.Logger
}

// Method with validation
func (h *ExtendedHandler) Login(c context.Context, req *AuthRequest) error {
	return c.JSON(200, map[string]string{
		"username": req.Username,
		"message":  "Login successful",
	})
}

// Method with JWT claims binding
func (h *ExtendedHandler) Profile(c context.Context, req *ProfileRequest) error {
	return c.JSON(200, req)
}

// Method with DI service injection
func (h *ExtendedHandler) ServiceMethod(c context.Context, svc *UserService, req *ServiceRequest) error {
	return c.JSON(200, map[string]string{
		"service_name": svc.Name,
		"request_name": req.Name,
	})
}

func TestParameterBinderValidation(t *testing.T) {
	
	binder := NewParameterBinder()
	handler := &ExtendedHandler{}

	t.Run("validation success", func(t *testing.T) {
		body := map[string]any{
			"username": "testuser",
			"password": "password123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		handlerType := reflect.TypeOf(handler)
		method, _ := handlerType.MethodByName("Login")
		params, err := binder.BindMethodParams(ctx, method)
		require.NoError(t, err)
		
		authReq := params[1].Interface().(*AuthRequest)
		assert.Equal(t, "testuser", authReq.Username)
		assert.Equal(t, "password123", authReq.Password)
	})

	t.Run("validation failure - missing required field", func(t *testing.T) {
		body := map[string]any{
			"username": "te", // too short
			"password": "pass", // too short
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		handlerType := reflect.TypeOf(handler)
		method, _ := handlerType.MethodByName("Login")
		_, err := binder.BindMethodParams(ctx, method)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestParameterBinderJWTClaims(t *testing.T) {
	
	binder := NewParameterBinder()
	handler := &ExtendedHandler{}

	// Create JWT token with claims
	claims := jwt.MapClaims{
		"user_id":  "123",
		"username": "john_doe",
		"email":    "john@example.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	
	// Set JWT token in context (as JWT middleware would do)
	ctx.Set("user", token)
	
	// Set additional context value
	ctx.Set("role", "admin")

	handlerType := reflect.TypeOf(handler)
	method, _ := handlerType.MethodByName("Profile")
	params, err := binder.BindMethodParams(ctx, method)
	require.NoError(t, err)

	profile := params[1].Interface().(*ProfileRequest)
	assert.Equal(t, "123", profile.UserID)
	assert.Equal(t, "john_doe", profile.Username)
	assert.Equal(t, "john@example.com", profile.Email)
	assert.Equal(t, "admin", profile.Role)
}

func TestParameterBinderDI(t *testing.T) {
	
	
	// Create DI context and register service
	diContext := NewContext()
	userService := &UserService{Name: "UserServiceImpl"}
	Register(diContext, userService)
	
	binder := NewParameterBinderWithContext(diContext)
	handler := &ExtendedHandler{}

	body := map[string]any{
		"name": "test request",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/service", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)

	handlerType := reflect.TypeOf(handler)
	method, _ := handlerType.MethodByName("ServiceMethod")
	params, err := binder.BindMethodParams(ctx, method)
	require.NoError(t, err)
	require.Len(t, params, 3) // context.Context + *UserService + *ServiceRequest

	// Check injected service
	svc := params[1].Interface().(*UserService)
	assert.Equal(t, "UserServiceImpl", svc.Name)

	// Check bound request
	svcReq := params[2].Interface().(*ServiceRequest)
	assert.Equal(t, "test request", svcReq.Name)
}

func TestParameterBinderAllSources(t *testing.T) {
	
	
	type CompleteRequest struct {
		// Different binding sources
		PathID      string `bind:"id,path"`
		QuerySearch string `bind:"q,query"`
		HeaderAuth  string `bind:"Authorization,header"`
		FormName    string `bind:"name,form"`
		JWTUserID   string `bind:"sub,jwt"`
		CtxRole     string `bind:"role,context"`
		// Auto detection
		AutoField   string `json:"auto_field"`
		// Validation
		Email       string `json:"email" validate:"required,email"`
	}

	// Create JWT token
	claims := jwt.MapClaims{
		"sub": "user-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create form data
	formData := "name=FormValue&auto_field=AutoValue"
	
	req := httptest.NewRequest(http.MethodPost, "/test/456?q=search-term", 
		bytes.NewReader([]byte(formData)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer token123")
	
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("456")
	ctx.Set("user", token)
	ctx.Set("role", "moderator")

	// Add JSON body for email field
	jsonBody := map[string]string{
		"email": "test@example.com",
	}
	jsonBytes, _ := json.Marshal(jsonBody)
	
	// For this test, we'll use JSON body instead of form
	req = httptest.NewRequest(http.MethodPost, "/test/456?q=search-term", 
		bytes.NewReader(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	ctx = context.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("456")
	ctx.Set("user", token)
	ctx.Set("role", "moderator")

	binder := NewParameterBinder()
	
	// Create a request and bind
	reqObj := &CompleteRequest{}
	paramValue := reflect.ValueOf(reqObj)
	
	err := binder.bindParameter(ctx, paramValue)
	require.NoError(t, err)

	// Verify all bindings
	assert.Equal(t, "456", reqObj.PathID)
	assert.Equal(t, "search-term", reqObj.QuerySearch)
	assert.Equal(t, "Bearer token123", reqObj.HeaderAuth)
	assert.Equal(t, "user-123", reqObj.JWTUserID)
	assert.Equal(t, "moderator", reqObj.CtxRole)
	assert.Equal(t, "test@example.com", reqObj.Email)
}

func TestParameterBinderEdgeCasesExtended(t *testing.T) {
	

	t.Run("DI service not found", func(t *testing.T) {
		diContext := NewContext()
		binder := NewParameterBinderWithContext(diContext)
		
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		_ = context.NewContext(req, rec)

		// Try to bind a service that's not registered
		paramType := reflect.TypeOf((*UserService)(nil))
		service, err := binder.getFromDI(paramType)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.False(t, service.IsValid())
	})

	t.Run("JWT claims not present", func(t *testing.T) {
		binder := NewParameterBinder()
		
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		claims := binder.getJWTClaims(ctx)
		assert.Nil(t, claims)
	})

	t.Run("validation with nil validator", func(t *testing.T) {
		binder := &ParameterBinder{
			tagName:   "bind",
			validator: nil, // No validator
		}
		
		body := map[string]any{
			"username": "", // Would fail validation if validator was present
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := context.NewContext(req, rec)

		authReq := &AuthRequest{}
		paramValue := reflect.ValueOf(authReq)
		
		// Should not error even with invalid data when validator is nil
		err := binder.bindParameter(ctx, paramValue)
		require.NoError(t, err)
	})
}