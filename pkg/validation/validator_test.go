package validation_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/pkg/validation"
)

func TestNewValidator(t *testing.T) {
	v := validation.NewValidator()
	assert.NotNil(t, v)
}

func TestValidation(t *testing.T) {
	v := validation.NewValidator()

	t.Run("ValidStruct", func(t *testing.T) {
		type User struct {
			Name  string `validate:"required,min=3,max=50"`
			Email string `validate:"required,email"`
			Age   int    `validate:"required,min=18,max=100"`
		}

		user := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   25,
		}

		err := v.Validate(user)
		assert.NoError(t, err)
	})

	t.Run("InvalidStruct", func(t *testing.T) {
		type User struct {
			Name  string `validate:"required,min=3,max=50"`
			Email string `validate:"required,email"`
			Age   int    `validate:"required,min=18,max=100"`
		}

		user := User{
			Name:  "Jo", // Too short
			Email: "invalid-email",
			Age:   15, // Too young
		}

		err := v.Validate(user)
		require.Error(t, err)

		ve, ok := err.(*validation.ValidationError)
		require.True(t, ok)
		assert.Len(t, ve.Errors, 3)
		assert.Contains(t, ve.Errors["name"], "at least 3 characters")
		assert.Contains(t, ve.Errors["email"], "valid email")
		assert.Contains(t, ve.Errors["age"], "at least 18")
	})
}

func TestCustomValidators(t *testing.T) {
	v := validation.NewValidator()

	t.Run("GameID", func(t *testing.T) {
		type Game struct {
			ID string `validate:"gameid"`
		}

		// Valid game IDs
		validIDs := []string{"game001", "slot123", "abc", "123456789012345678"}
		for _, id := range validIDs {
			err := v.Validate(Game{ID: id})
			assert.NoError(t, err, "GameID %s should be valid", id)
		}

		// Invalid game IDs
		invalidIDs := []string{"", "ab", "Game001", "game-001", "game_001", "123456789012345678901"}
		for _, id := range invalidIDs {
			err := v.Validate(Game{ID: id})
			assert.Error(t, err, "GameID %s should be invalid", id)
		}
	})

	t.Run("Currency", func(t *testing.T) {
		type Payment struct {
			Currency string `validate:"currency"`
		}

		// Valid currencies
		validCurrencies := []string{"USD", "EUR", "GBP", "JPY", "CNY", "TWD"}
		for _, currency := range validCurrencies {
			err := v.Validate(Payment{Currency: currency})
			assert.NoError(t, err, "Currency %s should be valid", currency)
		}

		// Invalid currencies
		invalidCurrencies := []string{"", "US", "USDD", "XYZ", "usd"}
		for _, currency := range invalidCurrencies {
			err := v.Validate(Payment{Currency: currency})
			assert.Error(t, err, "Currency %s should be invalid", currency)
		}
	})

	t.Run("Username", func(t *testing.T) {
		type User struct {
			Username string `validate:"username"`
		}

		// Valid usernames
		validUsernames := []string{"john_doe", "user123", "test-user", "JohnDoe", "user_123_test"}
		for _, username := range validUsernames {
			err := v.Validate(User{Username: username})
			assert.NoError(t, err, "Username %s should be valid", username)
		}

		// Invalid usernames
		invalidUsernames := []string{"", "ab", "_user", "user_", "-user", "user-", "user@test", "user name"}
		for _, username := range invalidUsernames {
			err := v.Validate(User{Username: username})
			assert.Error(t, err, "Username %s should be invalid", username)
		}
	})
}

func TestBindAndValidate(t *testing.T) {
	type LoginRequest struct {
		Username string `json:"username" validate:"required,min=3,max=50"`
		Password string `json:"password" validate:"required,min=6"`
	}

	handler := func(c context.Context) error {
		var req LoginRequest
		if err := validation.BindAndValidate(c, &req); err != nil {
			return err
		}
		return c.JSON(200, req)
	}

	t.Run("ValidRequest", func(t *testing.T) {
		body := `{"username":"testuser","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set(context.HeaderContentType, context.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)
		err := handler(c)
		if err != nil {
			if httpErr, ok := err.(*context.HTTPError); ok {
				rec.WriteHeader(httpErr.Code)
				if msg, ok := httpErr.Message.(string); ok {
					rec.Write([]byte(msg))
				}
			}
		}

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "testuser")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		body := `{"username":"testuser","password":}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set(context.HeaderContentType, context.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)
		err := handler(c)
		if err != nil {
			if httpErr, ok := err.(*context.HTTPError); ok {
				rec.WriteHeader(httpErr.Code)
				if msg, ok := httpErr.Message.(string); ok {
					rec.Write([]byte(msg))
				}
			}
		}

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid request format")
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		body := `{"username":"ab","password":"123"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set(context.HeaderContentType, context.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)
		err := handler(c)
		if err != nil {
			if httpErr, ok := err.(*context.HTTPError); ok {
				rec.WriteHeader(httpErr.Code)
				if msg, ok := httpErr.Message.(string); ok {
					rec.Write([]byte(msg))
				} else if errors, ok := httpErr.Message.(map[string]string); ok {
					// Handle validation errors
					body, _ := json.Marshal(errors)
					rec.Write(body)
				}
			}
		}

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "username")
		assert.Contains(t, rec.Body.String(), "password")
	})
}

func TestValidationError(t *testing.T) {
	t.Run("ErrorString", func(t *testing.T) {
		ve := &validation.ValidationError{
			Errors: map[string]string{
				"field1": "error1",
				"field2": "error2",
			},
		}

		errStr := ve.Error()
		assert.Contains(t, errStr, "field1: error1")
		assert.Contains(t, errStr, "field2: error2")
	})

	t.Run("EmptyErrors", func(t *testing.T) {
		ve := &validation.ValidationError{
			Errors: map[string]string{},
		}

		assert.Equal(t, "validation failed", ve.Error())
	})
}
