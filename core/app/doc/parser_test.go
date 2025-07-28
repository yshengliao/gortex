package doc

import (
	"reflect"
	"testing"
)

// Test handler for parsing
type TestHandler struct {
	Embedded
}

type Embedded struct {
	_ struct{} `api:"group=Test,version=v1,desc=Test API,tags=test|example"`
}

func (h *TestHandler) GET() {}
func (h *TestHandler) CreateUser() {}
func (h *TestHandler) UpdateProfile() {}

func TestParseHandlerMetadata(t *testing.T) {
	parser := NewTagParser()
	handler := &TestHandler{}

	metadata, err := parser.ParseHandlerMetadata(handler)
	if err != nil {
		t.Fatalf("ParseHandlerMetadata() error = %v", err)
	}

	if metadata.Group != "Test" {
		t.Errorf("Expected group 'Test', got '%s'", metadata.Group)
	}

	if metadata.Version != "v1" {
		t.Errorf("Expected version 'v1', got '%s'", metadata.Version)
	}

	if metadata.Description != "Test API" {
		t.Errorf("Expected description 'Test API', got '%s'", metadata.Description)
	}

	expectedTags := []string{"test", "example"}
	if !reflect.DeepEqual(metadata.Tags, expectedTags) {
		t.Errorf("Expected tags %v, got %v", expectedTags, metadata.Tags)
	}
}

func TestExtractHTTPMethod(t *testing.T) {
	parser := NewTagParser()

	tests := []struct {
		methodName string
		want       string
	}{
		{"GET", "GET"},
		{"POST", "POST"},
		{"PUT", "PUT"},
		{"DELETE", "DELETE"},
		{"ListUsers", "GET"},
		{"GetUser", "GET"},
		{"CreateUser", "POST"},
		{"UpdateUser", "PUT"},
		{"DeleteUser", "DELETE"},
		{"CustomMethod", "POST"},
	}

	for _, tt := range tests {
		t.Run(tt.methodName, func(t *testing.T) {
			if got := parser.extractHTTPMethod(tt.methodName); got != tt.want {
				t.Errorf("extractHTTPMethod(%s) = %v, want %v", tt.methodName, got, tt.want)
			}
		})
	}
}

func TestGenerateRoutePath(t *testing.T) {
	parser := NewTagParser()

	tests := []struct {
		methodName string
		basePath   string
		want       string
	}{
		{"GET", "/users", "/users"},
		{"POST", "/users", "/users"},
		{"CreateUser", "/users", "/users/create-user"},
		{"UpdateProfile", "/users", "/users/update-profile"},
		{"Search", "/", "/search"},
	}

	for _, tt := range tests {
		t.Run(tt.methodName, func(t *testing.T) {
			if got := parser.generateRoutePath(tt.methodName, tt.basePath); got != tt.want {
				t.Errorf("generateRoutePath(%s, %s) = %v, want %v", tt.methodName, tt.basePath, got, tt.want)
			}
		})
	}
}

func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CreateUser", "create-user"},
		{"UpdateUserProfile", "update-user-profile"},
		{"GET", "get"},
		{"HTTPSConnection", "https-connection"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := camelToKebab(tt.input); got != tt.want {
				t.Errorf("camelToKebab(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}