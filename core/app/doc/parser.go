package doc

import (
	"fmt"
	"reflect"
	"strings"
)

// TagParser parses struct tags to extract API documentation metadata
type TagParser struct{}

// NewTagParser creates a new tag parser instance
func NewTagParser() *TagParser {
	return &TagParser{}
}

// ParseHandlerMetadata extracts metadata from handler struct tags
func (p *TagParser) ParseHandlerMetadata(handler interface{}) (*HandlerMetadata, error) {
	t := reflect.TypeOf(handler)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("handler must be a struct, got %v", t.Kind())
	}
	
	// Check for api tag on the struct
	structTag := ""
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			// Check fields inside the anonymous struct
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Struct {
				for j := 0; j < embeddedType.NumField(); j++ {
					embeddedField := embeddedType.Field(j)
					if tag := embeddedField.Tag.Get("api"); tag != "" {
						structTag = tag
						break
					}
				}
			}
			// Also check the anonymous field's tag
			if structTag == "" {
				if tag := field.Tag.Get("api"); tag != "" {
					structTag = tag
					break
				}
			}
		}
	}
	
	// If no api tag found, check the struct itself (through type name convention)
	// This requires a different approach, so let's parse from field tags
	
	metadata := &HandlerMetadata{
		Tags: []string{},
	}
	
	// Parse the api tag
	if structTag != "" {
		p.parseAPITag(structTag, metadata)
	}
	
	return metadata, nil
}

// parseAPITag parses the api struct tag
func (p *TagParser) parseAPITag(tag string, metadata *HandlerMetadata) {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "group=") {
			metadata.Group = strings.TrimPrefix(part, "group=")
		} else if strings.HasPrefix(part, "version=") {
			metadata.Version = strings.TrimPrefix(part, "version=")
		} else if strings.HasPrefix(part, "desc=") {
			metadata.Description = strings.TrimPrefix(part, "desc=")
		} else if strings.HasPrefix(part, "tags=") {
			tagList := strings.TrimPrefix(part, "tags=")
			metadata.Tags = strings.Split(tagList, "|")
		} else if strings.HasPrefix(part, "basePath=") {
			metadata.BasePath = strings.TrimPrefix(part, "basePath=")
		}
	}
}

// ParseRouteInfo extracts route information from handler methods
func (p *TagParser) ParseRouteInfo(handlerType reflect.Type, method reflect.Method, basePath string, baseMetadata *HandlerMetadata) *RouteInfo {
	routeInfo := &RouteInfo{
		Handler:  fmt.Sprintf("%s.%s", handlerType.Name(), method.Name),
		Metadata: make(map[string]interface{}),
	}
	
	// Inherit metadata from handler
	if baseMetadata != nil {
		routeInfo.Tags = append(routeInfo.Tags, baseMetadata.Tags...)
		if baseMetadata.Group != "" {
			routeInfo.Metadata["group"] = baseMetadata.Group
		}
		if baseMetadata.Version != "" {
			routeInfo.Metadata["version"] = baseMetadata.Version
		}
	}
	
	// Parse method-specific api tag if present
	if method.Type.NumIn() > 0 {
		// Check for api tag on the method (this would require method tags which Go doesn't support directly)
		// Instead, we can use naming conventions or comments
		methodName := method.Name
		
		// Extract HTTP method from method name
		httpMethod := p.extractHTTPMethod(methodName)
		if httpMethod != "" {
			routeInfo.Method = httpMethod
		}
		
		// Generate path from method name
		routePath := p.generateRoutePath(methodName, basePath)
		routeInfo.Path = routePath
		
		// Set description based on method name
		routeInfo.Description = p.generateDescription(handlerType.Name(), methodName)
	}
	
	return routeInfo
}

// extractHTTPMethod extracts HTTP method from handler method name
func (p *TagParser) extractHTTPMethod(methodName string) string {
	// Standard HTTP method names
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	
	for _, method := range httpMethods {
		if methodName == method {
			return method
		}
	}
	
	// Check for RESTful naming conventions
	if strings.HasPrefix(methodName, "List") || strings.HasPrefix(methodName, "Get") {
		return "GET"
	}
	if strings.HasPrefix(methodName, "Create") || strings.HasPrefix(methodName, "Add") {
		return "POST"
	}
	if strings.HasPrefix(methodName, "Update") || strings.HasPrefix(methodName, "Edit") {
		return "PUT"
	}
	if strings.HasPrefix(methodName, "Delete") || strings.HasPrefix(methodName, "Remove") {
		return "DELETE"
	}
	
	// Default to POST for custom methods
	return "POST"
}

// generateRoutePath generates route path from method name
func (p *TagParser) generateRoutePath(methodName string, basePath string) string {
	// For standard HTTP methods, use base path
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range httpMethods {
		if methodName == method {
			return basePath
		}
	}
	
	// For custom methods, append to base path
	// Convert CamelCase to kebab-case
	routeName := camelToKebab(methodName)
	
	if basePath == "/" {
		return "/" + routeName
	}
	return basePath + "/" + routeName
}

// generateDescription generates a description from handler and method names
func (p *TagParser) generateDescription(handlerName, methodName string) string {
	// Remove common suffixes
	handlerName = strings.TrimSuffix(handlerName, "Handler")
	handlerName = strings.TrimSuffix(handlerName, "Controller")
	
	// Generate human-readable description
	action := methodNameToAction(methodName)
	resource := camelToWords(handlerName)
	
	return fmt.Sprintf("%s %s", action, resource)
}

// Helper functions

// camelToKebab converts CamelCase to kebab-case
func camelToKebab(s string) string {
	var result []rune
	runes := []rune(s)
	
	for i, r := range runes {
		if i > 0 && 'A' <= r && r <= 'Z' {
			// Check if this is the start of a new word
			// Add hyphen if:
			// 1. Previous char is lowercase (e.g., "userID" -> "user-id")
			// 2. Previous char is uppercase but next char is lowercase (e.g., "HTTPServer" -> "http-server")
			prevIsLower := 'a' <= runes[i-1] && runes[i-1] <= 'z'
			nextIsLower := i+1 < len(runes) && 'a' <= runes[i+1] && runes[i+1] <= 'z'
			prevIsUpper := 'A' <= runes[i-1] && runes[i-1] <= 'Z'
			
			if prevIsLower || (prevIsUpper && nextIsLower) {
				result = append(result, '-')
			}
		}
		result = append(result, []rune(strings.ToLower(string(r)))...)
	}
	return string(result)
}

// camelToWords converts CamelCase to space-separated words
func camelToWords(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result = append(result, ' ')
		}
		if i == 0 {
			result = append(result, []rune(strings.ToUpper(string(r)))...)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// methodNameToAction converts method name to action description
func methodNameToAction(methodName string) string {
	switch methodName {
	case "GET", "List":
		return "List"
	case "Get":
		return "Get"
	case "POST", "Create":
		return "Create"
	case "PUT", "Update":
		return "Update"
	case "DELETE", "Delete":
		return "Delete"
	case "PATCH":
		return "Partially update"
	default:
		return camelToWords(methodName)
	}
}