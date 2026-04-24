package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/core/app"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

type devRoutesTestHandler struct{}

func (h *devRoutesTestHandler) GET(c httpctx.Context) error {
	return c.String(200, "ok")
}

type devRoutesManager struct {
	Foo *devRoutesTestHandler `url:"/foo"`
}

// TestDevRoutes_RoutesEndpoint verifies that /_routes returns the actual
// registered routes, not hardcoded placeholder data.
func TestDevRoutes_RoutesEndpoint(t *testing.T) {
	a, err := app.NewApp(
		app.WithDevelopmentMode(),
		app.WithHandlers(&devRoutesManager{
			Foo: &devRoutesTestHandler{},
		}),
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/_routes", nil)
	rec := httptest.NewRecorder()
	a.ServerHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// total_routes must be a number (not a string like "TBD"), and must be > 0.
	totalRaw, ok := resp["total_routes"]
	require.True(t, ok, "response must have total_routes field")
	total, ok := totalRaw.(float64) // JSON numbers unmarshal to float64
	require.True(t, ok, "total_routes must be a number, got %T (%v)", totalRaw, totalRaw)
	assert.Greater(t, int(total), 0, "total_routes must be > 0")

	// routes array must contain at least /foo.
	routesRaw, ok := resp["routes"]
	require.True(t, ok, "response must have routes field")
	routes, ok := routesRaw.([]any)
	require.True(t, ok, "routes must be an array")
	require.NotEmpty(t, routes)

	found := false
	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if path, ok := rm["path"].(string); ok && strings.HasSuffix(path, "/foo") {
			found = true
			break
		}
	}
	assert.True(t, found, "/_routes must include the /foo route registered by the app")
}

// TestDevRoutes_MonitorEndpoint verifies that /_monitor's total_routes is a
// numeric value, not the placeholder string "TBD".
func TestDevRoutes_MonitorEndpoint(t *testing.T) {
	a, err := app.NewApp(
		app.WithDevelopmentMode(),
		app.WithHandlers(&devRoutesManager{
			Foo: &devRoutesTestHandler{},
		}),
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
	rec := httptest.NewRecorder()
	a.ServerHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	routesRaw, ok := resp["routes"]
	require.True(t, ok, "response must have routes field")
	routesMap, ok := routesRaw.(map[string]any)
	require.True(t, ok, "routes must be an object")

	totalRaw, ok := routesMap["total_routes"]
	require.True(t, ok, "routes.total_routes must exist")

	// Must NOT be the string "TBD".
	_, isStr := totalRaw.(string)
	assert.False(t, isStr, "total_routes must not be a string (was %q)", totalRaw)
	total, ok := totalRaw.(float64)
	require.True(t, ok, "total_routes must be a number, got %T", totalRaw)
	assert.Greater(t, int(total), 0, "total_routes must be > 0")
}
