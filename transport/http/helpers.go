package http

import (
	"net/http"
	"strconv"
)

// ParamInt returns path parameter as int with default value
func (c *DefaultContext) ParamInt(name string, defaultValue int) int {
	param := c.Param(name)
	if param == "" {
		return defaultValue
	}
	
	value, err := strconv.Atoi(param)
	if err != nil {
		return defaultValue
	}
	
	return value
}

// QueryInt returns query parameter as int with default value
func (c *DefaultContext) QueryInt(name string, defaultValue int) int {
	query := c.QueryParam(name)
	if query == "" {
		return defaultValue
	}
	
	value, err := strconv.Atoi(query)
	if err != nil {
		return defaultValue
	}
	
	return value
}

// QueryBool returns query parameter as bool with default value
func (c *DefaultContext) QueryBool(name string, defaultValue bool) bool {
	query := c.QueryParam(name)
	if query == "" {
		return defaultValue
	}
	
	value, err := strconv.ParseBool(query)
	if err != nil {
		return defaultValue
	}
	
	return value
}

// OK sends a successful response with data (200 OK)
func (c *DefaultContext) OK(data interface{}) error {
	return c.JSON(http.StatusOK, data)
}

// Created sends a created response with data (201 Created)
func (c *DefaultContext) Created(data interface{}) error {
	return c.JSON(http.StatusCreated, data)
}

// NoContent204 sends a no content response (204 No Content)
func (c *DefaultContext) NoContent204() error {
	return c.NoContent(http.StatusNoContent)
}

// BadRequest sends a bad request response with message (400 Bad Request)
func (c *DefaultContext) BadRequest(message string) error {
	return c.JSON(http.StatusBadRequest, Map{
		"error": message,
	})
}