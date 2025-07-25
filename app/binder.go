package app

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// ParameterBinder handles automatic parameter binding from HTTP requests
type ParameterBinder struct {
	// tagName is the struct tag name to use for binding hints
	tagName string
}

// NewParameterBinder creates a new parameter binder
func NewParameterBinder() *ParameterBinder {
	return &ParameterBinder{
		tagName: "bind",
	}
}

// BindMethodParams binds HTTP request parameters to method parameters
func (pb *ParameterBinder) BindMethodParams(c echo.Context, method reflect.Method) ([]reflect.Value, error) {
	methodType := method.Type
	numParams := methodType.NumIn()
	params := make([]reflect.Value, 0, numParams)

	// Skip the receiver (first parameter)
	for i := 1; i < numParams; i++ {
		paramType := methodType.In(i)
		
		// Handle echo.Context parameter
		if paramType == reflect.TypeOf((*echo.Context)(nil)).Elem() {
			params = append(params, reflect.ValueOf(c))
			continue
		}

		// Create new instance of parameter type
		var paramValue reflect.Value
		if paramType.Kind() == reflect.Ptr {
			// For pointer types, create a new instance of the element type
			paramValue = reflect.New(paramType.Elem())
		} else {
			// For non-pointer types, create a pointer to new instance
			paramValue = reflect.New(paramType)
		}
		
		// Bind based on parameter type
		if err := pb.bindParameter(c, paramValue); err != nil {
			return nil, fmt.Errorf("failed to bind parameter %d: %w", i, err)
		}

		// If parameter is not a pointer, dereference it
		if paramType.Kind() != reflect.Ptr {
			paramValue = paramValue.Elem()
		}
		
		params = append(params, paramValue)
	}

	return params, nil
}

// bindParameter binds a single parameter from the request
func (pb *ParameterBinder) bindParameter(c echo.Context, paramValue reflect.Value) error {
	paramType := paramValue.Type()
	isPtr := paramType.Kind() == reflect.Ptr
	
	// If it's a pointer, get the element type for checking
	elemType := paramType
	if isPtr {
		elemType = paramType.Elem()
	}

	switch elemType.Kind() {
	case reflect.Struct:
		return pb.bindStruct(c, paramValue)
	case reflect.String, reflect.Int, reflect.Int64, reflect.Bool:
		// Try to bind from path or query parameters
		return pb.bindPrimitive(c, paramValue)
	default:
		return fmt.Errorf("unsupported parameter type: %v", paramType)
	}
}

// bindStruct binds a struct from the request
func (pb *ParameterBinder) bindStruct(c echo.Context, structValue reflect.Value) error {
	structType := structValue.Type()
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
		structValue = structValue.Elem()
	}

	// First, try to bind from JSON body if it's a POST/PUT/PATCH request
	if c.Request().Method == "POST" || c.Request().Method == "PUT" || c.Request().Method == "PATCH" {
		if c.Request().Header.Get("Content-Type") == "application/json" {
			if err := json.NewDecoder(c.Request().Body).Decode(structValue.Addr().Interface()); err != nil && err.Error() != "EOF" {
				// If JSON parsing fails, continue to try other binding methods
			} else {
				// JSON binding successful, now bind other sources
			}
		}
	}

	// Bind individual fields based on struct tags
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)
		
		if !fieldValue.CanSet() {
			continue
		}

		// Check for binding tag
		bindTag := field.Tag.Get(pb.tagName)
		bindParts := strings.Split(bindTag, ",")
		bindName := ""
		
		if len(bindParts) > 0 && bindParts[0] != "" {
			bindName = bindParts[0]
		} else {
			// Use json tag as fallback
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				bindName = strings.Split(jsonTag, ",")[0]
			}
		}
		
		// If no tag, use field name
		if bindName == "" || bindName == "-" {
			bindName = strings.ToLower(field.Name)
		}

		// Try to bind from different sources
		if err := pb.bindField(c, fieldValue, field, bindName); err != nil {
			// Continue on error, field remains at zero value
			continue
		}
	}

	return nil
}

// bindField binds a single struct field
func (pb *ParameterBinder) bindField(c echo.Context, fieldValue reflect.Value, field reflect.StructField, name string) error {
	// Check binding source from tag
	parts := strings.Split(field.Tag.Get(pb.tagName), ",")
	source := "auto" // default to auto-detection
	if len(parts) > 1 {
		source = parts[1]
	}

	var value string
	var found bool

	switch source {
	case "path":
		value = c.Param(name)
		found = value != ""
	case "query":
		value = c.QueryParam(name)
		found = value != ""
	case "header":
		value = c.Request().Header.Get(name)
		found = value != ""
	case "form":
		value = c.FormValue(name)
		found = value != ""
	default: // "auto"
		// Try path first, then query, then form
		if value = c.Param(name); value != "" {
			found = true
		} else if value = c.QueryParam(name); value != "" {
			found = true
		} else if value = c.FormValue(name); value != "" {
			found = true
		}
	}

	if !found {
		return nil // Leave at zero value
	}

	return pb.setFieldValue(fieldValue, value)
}

// bindPrimitive binds a primitive type from path or query parameters
func (pb *ParameterBinder) bindPrimitive(c echo.Context, value reflect.Value) error {
	// For primitive types, we need to determine the parameter name
	// This is a simplified approach - in real usage, you'd need method parameter names
	
	// Try common parameter names
	paramNames := []string{"id", "ID", "value", "param"}
	
	var strValue string
	for _, name := range paramNames {
		if v := c.Param(name); v != "" {
			strValue = v
			break
		}
		if v := c.QueryParam(name); v != "" {
			strValue = v
			break
		}
	}

	if strValue == "" {
		// Try first path parameter
		for _, paramName := range c.ParamNames() {
			if v := c.Param(paramName); v != "" {
				strValue = v
				break
			}
		}
	}

	if strValue == "" {
		return nil // Leave at zero value
	}

	return pb.setFieldValue(value, strValue)
}

// setFieldValue sets the value of a field from a string
func (pb *ParameterBinder) setFieldValue(fieldValue reflect.Value, value string) error {
	fieldType := fieldValue.Type()
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
		if fieldValue.IsNil() {
			fieldValue.Set(reflect.New(fieldType))
		}
		fieldValue = fieldValue.Elem()
	}

	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fieldType == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			fieldValue.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, fieldType.Bits())
			if err != nil {
				return err
			}
			fieldValue.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, fieldType.Bits())
		if err != nil {
			return err
		}
		fieldValue.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, fieldType.Bits())
		if err != nil {
			return err
		}
		fieldValue.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		fieldValue.SetBool(boolVal)
	case reflect.Struct:
		if fieldType == reflect.TypeOf(time.Time{}) {
			// Try to parse time
			formats := []string{
				time.RFC3339,
				"2006-01-02T15:04:05",
				"2006-01-02",
			}
			var t time.Time
			var err error
			for _, format := range formats {
				t, err = time.Parse(format, value)
				if err == nil {
					fieldValue.Set(reflect.ValueOf(t))
					return nil
				}
			}
			return fmt.Errorf("unable to parse time: %s", value)
		}
		return fmt.Errorf("unsupported struct type: %v", fieldType)
	default:
		return fmt.Errorf("unsupported field type: %v", fieldType)
	}

	return nil
}