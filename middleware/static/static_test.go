package static_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/middleware/static"
)

func TestStaticFileServer(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create test files
	testFiles := map[string]string{
		"index.html":       "<html><body>Home</body></html>",
		"about.html":       "<html><body>About</body></html>",
		"css/style.css":    "body { margin: 0; }",
		"js/script.js":     "console.log('Hello');",
		"data.json":        `{"message": "Hello"}`,
		"image.png":        "PNG_DATA",
	}
	
	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}
	
	// Create pre-compressed files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "data.json.gz"), []byte("GZIP_COMPRESSED_DATA"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "data.json.br"), []byte("BROTLI_COMPRESSED_DATA"), 0644))
	
	e := echo.New()
	e.Use(static.StaticWithConfig(static.Config{
		Root:         tempDir,
		EnableCache:  true,
		EnableETag:   true,
		EnableGzip:   true,
		EnableBrotli: true,
	}))
	
	// Test serving regular file
	t.Run("ServeFile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, testFiles["index.html"], rec.Body.String())
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
		assert.NotEmpty(t, rec.Header().Get("ETag"))
		assert.NotEmpty(t, rec.Header().Get("Last-Modified"))
		assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=")
	})
	
	// Test serving nested file
	t.Run("ServeNestedFile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, testFiles["css/style.css"], rec.Body.String())
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
	})
	
	// Test directory index
	t.Run("DirectoryIndex", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, testFiles["index.html"], rec.Body.String())
	})
	
	// Test ETag
	t.Run("ETag", func(t *testing.T) {
		// First request
		req := httptest.NewRequest(http.MethodGet, "/about.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		etag := rec.Header().Get("ETag")
		assert.NotEmpty(t, etag)
		
		// Second request with If-None-Match
		req = httptest.NewRequest(http.MethodGet, "/about.html", nil)
		req.Header.Set("If-None-Match", etag)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotModified, rec.Code)
		assert.Empty(t, rec.Body.String())
	})
	
	// Test Last-Modified
	t.Run("LastModified", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/js/script.js", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		lastModified := rec.Header().Get("Last-Modified")
		assert.NotEmpty(t, lastModified)
		
		// Request with If-Modified-Since (future date)
		req = httptest.NewRequest(http.MethodGet, "/js/script.js", nil)
		req.Header.Set("If-Modified-Since", time.Now().Add(1*time.Hour).UTC().Format(http.TimeFormat))
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotModified, rec.Code)
	})
	
	// Test pre-compressed files
	t.Run("PreCompressedGzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/data.json", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, "GZIP_COMPRESSED_DATA", rec.Body.String())
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	})
	
	t.Run("PreCompressedBrotli", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/data.json", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, "BROTLI_COMPRESSED_DATA", rec.Body.String())
	})
	
	// Test 404
	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/notfound.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestRangeRequests(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test file with known content
	content := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))
	
	e := echo.New()
	e.Use(static.Static(tempDir))
	
	tests := []struct {
		name          string
		rangeHeader   string
		expectedCode  int
		expectedBody  string
		expectedRange string
	}{
		{
			name:          "full_range",
			rangeHeader:   "bytes=0-35",
			expectedCode:  http.StatusPartialContent,
			expectedBody:  content,
			expectedRange: "bytes 0-35/36",
		},
		{
			name:          "partial_range",
			rangeHeader:   "bytes=10-19",
			expectedCode:  http.StatusPartialContent,
			expectedBody:  "ABCDEFGHIJ",
			expectedRange: "bytes 10-19/36",
		},
		{
			name:          "suffix_range",
			rangeHeader:   "bytes=-10",
			expectedCode:  http.StatusPartialContent,
			expectedBody:  "QRSTUVWXYZ", // Last 10 bytes
			expectedRange: "bytes 26-35/36",
		},
		{
			name:          "open_ended_range",
			rangeHeader:   "bytes=30-",
			expectedCode:  http.StatusPartialContent,
			expectedBody:  "UVWXYZ",
			expectedRange: "bytes 30-35/36",
		},
		{
			name:          "invalid_range",
			rangeHeader:   "bytes=50-60",
			expectedCode:  http.StatusRequestedRangeNotSatisfiable,
			expectedBody:  "",
			expectedRange: "bytes */36",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedCode, rec.Code)
			
			if tt.expectedCode == http.StatusPartialContent || tt.expectedCode == http.StatusRequestedRangeNotSatisfiable {
				assert.Equal(t, tt.expectedRange, rec.Header().Get("Content-Range"))
			}
			
			if tt.expectedBody != "" {
				body, _ := io.ReadAll(rec.Body)
				assert.Equal(t, tt.expectedBody, string(body))
			}
		})
	}
}

func TestDirectoryBrowsing(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "subdir/file3.txt"), []byte("content3"), 0644))
	
	e := echo.New()
	e.Use(static.StaticWithConfig(static.Config{
		Root:   tempDir,
		Browse: true,
	}))
	
	// Test directory listing
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	
	body := rec.Body.String()
	assert.Contains(t, body, "Index of /")
	assert.Contains(t, body, "file1.txt")
	assert.Contains(t, body, "file2.txt")
	assert.Contains(t, body, "subdir/")
	
	// Test subdirectory listing
	req = httptest.NewRequest(http.MethodGet, "/subdir/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	body = rec.Body.String()
	assert.Contains(t, body, "Index of /subdir/")
	assert.Contains(t, body, "../")
	assert.Contains(t, body, "file3.txt")
}

func TestHTML5Mode(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create index.html
	indexContent := "<html><body>SPA</body></html>"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexContent), 0644))
	
	// Create some other files
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "api"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "api/data.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "style.css"), []byte("body{}"), 0644))
	
	e := echo.New()
	e.Use(static.StaticWithConfig(static.Config{
		Root:  tempDir,
		HTML5: true,
	}))
	
	// Routes without extension should fallback to index.html
	routes := []string{"/", "/about", "/users/123", "/deep/nested/route"}
	
	for _, route := range routes {
		t.Run("Route_"+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, indexContent, rec.Body.String())
		})
	}
	
	// Files with extensions should be served normally
	t.Run("CSS_File", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "body{}", rec.Body.String())
	})
	
	// API routes should 404 if not found
	t.Run("API_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notfound.json", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestSkipper(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0644))
	
	e := echo.New()
	e.Use(static.StaticWithConfig(static.Config{
		Root: tempDir,
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Path(), "/api")
		},
	}))
	
	// API handler
	e.GET("/api/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "API response")
	})
	
	// Static file should be served
	req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "content", rec.Body.String())
	
	// API route should skip static middleware
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "API response", rec.Body.String())
}

func BenchmarkStaticFileServing(b *testing.B) {
	tempDir := b.TempDir()
	
	// Create test file
	content := strings.Repeat("Hello World!\n", 1000)
	require.NoError(b, os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(content), 0644))
	
	e := echo.New()
	e.Use(static.Static(tempDir))
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}

func BenchmarkStaticFileWithETag(b *testing.B) {
	tempDir := b.TempDir()
	
	content := strings.Repeat("Hello World!\n", 1000)
	require.NoError(b, os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(content), 0644))
	
	e := echo.New()
	e.Use(static.StaticWithConfig(static.Config{
		Root:       tempDir,
		EnableETag: true,
	}))
	
	// Get ETag
	req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	etag := rec.Header().Get("ETag")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}