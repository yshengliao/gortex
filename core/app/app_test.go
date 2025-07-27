package app_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

type TestHandlers struct {
	Hello *HelloHandler `url:"/hello"`
	User  *UserHandler  `url:"/user"`
}

type HelloHandler struct{}

func (h *HelloHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"message": "Hello, World!"})
}

type UserHandler struct{}

func (h *UserHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"endpoint": "list users"})
}

func (h *UserHandler) Create(c context.Context) error {
	return c.JSON(201, map[string]string{"endpoint": "create user"})
}

func TestNewApp(t *testing.T) {
	// Test with valid options
	logger, _ := zap.NewDevelopment()
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
	)

	assert.NoError(t, err)
	assert.NotNil(t, application)
	assert.NotNil(t, application.Router())
	assert.NotNil(t, application.Context())
}

func TestNewApp_WithNilConfig(t *testing.T) {
	_, err := app.NewApp(
		app.WithConfig(nil),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNewApp_WithNilLogger(t *testing.T) {
	_, err := app.NewApp(
		app.WithLogger(nil),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestDeclarativeRouting(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := &TestHandlers{
		Hello: &HelloHandler{},
		User:  &UserHandler{},
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)

	router := application.Router()

	// Test GET /hello
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Hello, World!")

	// Test GET /user
	req = httptest.NewRequest(http.MethodGet, "/user", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "list users")

	// Test POST /user/create
	req = httptest.NewRequest(http.MethodPost, "/user/create", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "create user")
}

func TestDependencyInjection(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	application, err := app.NewApp(
		app.WithLogger(logger),
	)
	require.NoError(t, err)

	ctx := application.Context()

	// Register a service
	type TestService struct {
		Name string
	}
	service := &TestService{Name: "test"}
	app.Register(ctx, service)

	// Retrieve the service
	retrieved, err := app.Get[*TestService](ctx)
	assert.NoError(t, err)
	assert.Equal(t, service.Name, retrieved.Name)

	// Test MustGet
	assert.NotPanics(t, func() {
		s := app.MustGet[*TestService](ctx)
		assert.Equal(t, service.Name, s.Name)
	})

	// Test missing service
	_, err = app.Get[*time.Timer](ctx)
	assert.Error(t, err)

	// Test MustGet panic
	assert.Panics(t, func() {
		app.MustGet[*time.Timer](ctx)
	})
}

type ErrorHandler struct{}

func (h *ErrorHandler) GET(c context.Context) error {
	return context.NewHTTPError(http.StatusTeapot, "I'm a teapot")
}

func TestCustomErrorHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	handlers := &struct {
		Error *ErrorHandler `url:"/error"`
	}{
		Error: &ErrorHandler{},
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Contains(t, rec.Body.String(), "I'm a teapot")
}
