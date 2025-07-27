// Package validation provides request validation using go-playground/validator
package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/yshengliao/gortex/transport/http"
)

// Validator wraps go-playground/validator
type Validator struct {
	validator *validator.Validate
}

// NewValidator creates a new validator instance with custom rules
func NewValidator() *Validator {
	v := validator.New()
	
	// Register custom validators
	RegisterCustomValidators(v)
	
	return &Validator{
		validator: v,
	}
}

// Validate validates a struct
func (v *Validator) Validate(i any) error {
	if err := v.validator.Struct(i); err != nil {
		return NewValidationError(err)
	}
	return nil
}

// getValidationMessage returns a custom error message for validation errors
func getValidationMessage(field, tag, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, param)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "gameid":
		return fmt.Sprintf("%s must be 3-20 lowercase alphanumeric characters", field)
	case "currency":
		return fmt.Sprintf("%s must be a valid currency code", field)
	case "username":
		return fmt.Sprintf("%s must be 3-30 characters, alphanumeric with _ or -", field)
	default:
		return fmt.Sprintf("%s failed %s validation", field, tag)
	}
}

// RegisterCustomValidators registers all custom validation rules
func RegisterCustomValidators(v *validator.Validate) {
	// Game ID validator
	v.RegisterValidation("gameid", func(fl validator.FieldLevel) bool {
		gameID := fl.Field().String()
		if len(gameID) < 3 || len(gameID) > 20 {
			return false
		}
		for _, r := range gameID {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				return false
			}
		}
		return true
	})

	// Currency validator
	v.RegisterValidation("currency", func(fl validator.FieldLevel) bool {
		currency := fl.Field().String()
		validCurrencies := []string{"USD", "EUR", "GBP", "JPY", "CNY", "TWD"}
		for _, valid := range validCurrencies {
			if currency == valid {
				return true
			}
		}
		return false
	})

	// Username validator
	v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		username := fl.Field().String()
		if len(username) < 3 || len(username) > 30 {
			return false
		}
		for i, r := range username {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			   (r >= '0' && r <= '9') || r == '_' || r == '-') {
				return false
			}
			if (r == '_' || r == '-') && (i == 0 || i == len(username)-1) {
				return false
			}
		}
		return true
	})
}

// ValidationError represents a validation error
type ValidationError struct {
	Errors map[string]string `json:"errors"`
}

// NewValidationError creates a validation error from validator errors
func NewValidationError(err error) *ValidationError {
	ve := &ValidationError{
		Errors: make(map[string]string),
	}
	
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := strings.ToLower(e.Field())
			tag := e.Tag()
			
			// Custom error messages
			ve.Errors[field] = getValidationMessage(field, tag, e.Param())
		}
	}
	
	return ve
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "validation failed"
	}
	
	var errors []string
	for field, msg := range ve.Errors {
		errors = append(errors, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(errors, "; ")
}

// BindAndValidate binds and validates request data
func BindAndValidate(c context.Context, i any) error {
	if err := c.Bind(i); err != nil {
		return context.NewHTTPError(400, "Invalid request format")
	}
	
	// Create a validator instance
	v := NewValidator()
	if err := v.Validate(i); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			return context.NewHTTPError(400, ve.Errors)
		}
		return context.NewHTTPError(400, err.Error())
	}
	
	return nil
}