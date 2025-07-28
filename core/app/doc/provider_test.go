package doc

import (
	"net/http"
	"testing"
)

// MockDocProvider is a mock implementation of DocProvider for testing
type MockDocProvider struct {
	GenerateCalled bool
	Routes         []RouteInfo
	JSONData       []byte
}

func (m *MockDocProvider) Generate(routes []RouteInfo) ([]byte, error) {
	m.GenerateCalled = true
	m.Routes = routes
	return m.JSONData, nil
}

func (m *MockDocProvider) ContentType() string {
	return "application/json"
}

func (m *MockDocProvider) UIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Mock UI"))
	})
}

func (m *MockDocProvider) Endpoints() map[string]http.Handler {
	return map[string]http.Handler{
		"/docs":      m.UIHandler(),
		"/docs.json": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", m.ContentType())
			w.Write(m.JSONData)
		}),
	}
}

func TestMockDocProvider(t *testing.T) {
	provider := &MockDocProvider{
		JSONData: []byte(`{"test": "data"}`),
	}

	// Test Generate
	routes := []RouteInfo{
		{
			Method:      "GET",
			Path:        "/test",
			Description: "Test route",
		},
	}

	data, err := provider.Generate(routes)
	if err != nil {
		t.Errorf("Generate() error = %v", err)
	}

	if !provider.GenerateCalled {
		t.Error("Generate() was not called")
	}

	if string(data) != string(provider.JSONData) {
		t.Errorf("Generate() returned %s, want %s", data, provider.JSONData)
	}

	// Test ContentType
	if ct := provider.ContentType(); ct != "application/json" {
		t.Errorf("ContentType() = %v, want application/json", ct)
	}

	// Test Endpoints
	endpoints := provider.Endpoints()
	if len(endpoints) != 2 {
		t.Errorf("Endpoints() returned %d endpoints, want 2", len(endpoints))
	}
}