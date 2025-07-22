package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/middleware/static"
	"go.uber.org/zap"
)

func TestStaticFilesExample(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	createDemoFiles(tempDir)

	// Initialize logger
	logger, _ := zap.NewDevelopment()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	cfg.Logger.Level = "debug"

	// Create handlers
	handlers := &HandlersManager{}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)

	// Add static middleware
	e := application.Echo()
	e.Use(static.Static(tempDir))

	// Test serving index.html
	t.Run("ServeIndexHTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, rec.Body.String(), "Welcome to Gortex")
	})

	// Test serving CSS file
	t.Run("ServeCSSFile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
		assert.Contains(t, rec.Body.String(), "font-family: Arial")
	})

	// Test serving JS file
	t.Run("ServeJSFile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/js/app.js", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "javascript")
		assert.Contains(t, rec.Body.String(), "console.log")
	})

	// Test 404 for non-existent file
	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/notfound.txt", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestStaticFilesWithAdvancedConfig(t *testing.T) {
	tempDir := t.TempDir()
	createDemoFiles(tempDir)

	logger, _ := zap.NewDevelopment()
	cfg := &app.Config{}
	handlers := &HandlersManager{}

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)

	e := application.Echo()

	// Test with advanced configuration
	e.Use(static.StaticWithConfig(static.Config{
		Root:         tempDir,
		Browse:       true,
		EnableETag:   true,
		EnableCache:  true,
		CacheMaxAge:  3600,
		EnableGzip:   true,
		EnableBrotli: true,
	}))

	// Test ETag support
	t.Run("ETagSupport", func(t *testing.T) {
		// First request
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		etag := rec.Header().Get("ETag")
		assert.NotEmpty(t, etag)

		// Second request with If-None-Match
		req = httptest.NewRequest(http.MethodGet, "/index.html", nil)
		req.Header.Set("If-None-Match", etag)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotModified, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	// Test cache headers
	t.Run("CacheHeaders", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=3600")
		assert.NotEmpty(t, rec.Header().Get("Last-Modified"))
	})

	// Test pre-compressed file serving
	t.Run("PreCompressedGzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, "GZIP_COMPRESSED_CSS", rec.Body.String())
	})

	// Test directory browsing
	t.Run("DirectoryBrowsing", func(t *testing.T) {
		// Create a subdirectory without index.html
		subdir := filepath.Join(tempDir, "subdir")
		os.MkdirAll(subdir, 0755)
		os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("test"), 0644)

		req := httptest.NewRequest(http.MethodGet, "/subdir/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, rec.Body.String(), "Index of /subdir/")
		assert.Contains(t, rec.Body.String(), "file.txt")
	})
}

func TestHTML5Mode(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create only index.html for SPA
	indexContent := `<!DOCTYPE html>
<html>
<head><title>SPA</title></head>
<body><div id="app"></div></body>
</html>`
	os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexContent), 0644)

	logger, _ := zap.NewDevelopment()
	cfg := &app.Config{}
	handlers := &HandlersManager{}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)

	e := application.Echo()
	e.Use(static.StaticWithConfig(static.Config{
		Root:  tempDir,
		HTML5: true,
	}))

	// Test SPA routes
	routes := []string{"/", "/about", "/users/123", "/deep/nested/route"}
	
	for _, route := range routes {
		t.Run("Route_"+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), `<div id="app"></div>`)
		})
	}
}

func BenchmarkStaticFileServing(b *testing.B) {
	tempDir := b.TempDir()
	createDemoFiles(tempDir)

	logger, _ := zap.NewProduction()
	cfg := &app.Config{}
	handlers := &HandlersManager{}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)

	e := application.Echo()
	e.Use(static.Static(tempDir))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}

func BenchmarkStaticFileWithETag(b *testing.B) {
	tempDir := b.TempDir()
	createDemoFiles(tempDir)

	logger, _ := zap.NewProduction()
	cfg := &app.Config{}
	handlers := &HandlersManager{}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)

	e := application.Echo()
	e.Use(static.StaticWithConfig(static.Config{
		Root:       tempDir,
		EnableETag: true,
	}))

	// Get ETag
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	etag := rec.Header().Get("ETag")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}