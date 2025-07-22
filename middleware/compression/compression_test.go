package compression_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/middleware/compression"
)

func TestGzipCompression(t *testing.T) {
	e := echo.New()
	
	// Large content to trigger compression
	largeContent := strings.Repeat("Hello, World! This is a test content for compression. ", 100)
	
	e.Use(compression.Gzip())
	
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	// Test with gzip support
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
	
	// Decompress and verify content
	reader, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer reader.Close()
	
	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	require.NoError(t, err)
	
	assert.Equal(t, largeContent, buf.String())
}

func TestBrotliCompression(t *testing.T) {
	e := echo.New()
	
	// Large content to trigger compression
	largeContent := strings.Repeat("Hello, World! This is a test content for compression. ", 100)
	
	e.Use(compression.Brotli())
	
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	// Test with brotli support
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
	
	// Decompress and verify content
	reader := brotli.NewReader(rec.Body)
	
	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	require.NoError(t, err)
	
	assert.Equal(t, largeContent, buf.String())
}

func TestCompressionWithConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         compression.Config
		acceptEncoding string
		contentSize    int
		contentType    string
		expectEncoding string
	}{
		{
			name: "prefer_brotli_over_gzip",
			config: compression.Config{
				Level:        compression.CompressionLevelDefault,
				MinSize:      100,
				EnableBrotli: true,
				PreferBrotli: true,
				ContentTypes: []string{"text/plain"},
				Skipper:      func(c echo.Context) bool { return false },
			},
			acceptEncoding: "gzip, br",
			contentSize:    200,
			contentType:    "text/plain",
			expectEncoding: "br",
		},
		{
			name: "prefer_gzip_when_brotli_disabled",
			config: compression.Config{
				Level:        compression.CompressionLevelDefault,
				MinSize:      100,
				EnableBrotli: false,
				ContentTypes: []string{"text/plain"},
				Skipper:      func(c echo.Context) bool { return false },
			},
			acceptEncoding: "gzip, br",
			contentSize:    200,
			contentType:    "text/plain",
			expectEncoding: "gzip",
		},
		{
			name: "no_compression_without_accept_encoding",
			config: compression.Config{
				Level:        compression.CompressionLevelDefault,
				MinSize:      100,
				EnableBrotli: true,
				ContentTypes: []string{"text/plain"},
				Skipper:      func(c echo.Context) bool { return false },
			},
			acceptEncoding: "",
			contentSize:    200,
			contentType:    "text/plain",
			expectEncoding: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			
			content := strings.Repeat("a", tt.contentSize)
			
			e.Use(compression.Middleware(tt.config))
			
			e.GET("/", func(c echo.Context) error {
				c.Response().Header().Set("Content-Type", tt.contentType)
				return c.String(http.StatusOK, content)
			})
			
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()
			
			e.ServeHTTP(rec, req)
			
			assert.Equal(t, http.StatusOK, rec.Code)
			
			if tt.expectEncoding != "" {
				assert.Equal(t, tt.expectEncoding, rec.Header().Get("Content-Encoding"))
				assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"))
			}
		})
	}
}

func TestCompressionLevels(t *testing.T) {
	levels := []struct {
		name  string
		level compression.CompressionLevel
	}{
		{"default", compression.CompressionLevelDefault},
		{"best_speed", compression.CompressionLevelBestSpeed},
		{"best_compression", compression.CompressionLevelBestCompression},
	}
	
	for _, lvl := range levels {
		t.Run(lvl.name, func(t *testing.T) {
			e := echo.New()
			
			largeContent := strings.Repeat("Hello, World! ", 1000)
			
			config := compression.DefaultConfig()
			config.Level = lvl.level
			config.EnableBrotli = false // Test with gzip only
			
			e.Use(compression.Middleware(config))
			
			e.GET("/", func(c echo.Context) error {
				return c.String(http.StatusOK, largeContent)
			})
			
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			
			e.ServeHTTP(rec, req)
			
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			
			// Verify compressed size is less than original
			assert.Less(t, rec.Body.Len(), len(largeContent))
		})
	}
}

func TestSkipper(t *testing.T) {
	e := echo.New()
	
	config := compression.DefaultConfig()
	config.Skipper = func(c echo.Context) bool {
		return c.Path() == "/skip"
	}
	
	e.Use(compression.Middleware(config))
	
	largeContent := strings.Repeat("Hello, World! ", 100)
	
	e.GET("/compress", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	e.GET("/skip", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	// Test compression endpoint
	req := httptest.NewRequest(http.MethodGet, "/compress", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	
	// Test skipped endpoint
	req = httptest.NewRequest(http.MethodGet, "/skip", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
}

func BenchmarkGzipCompression(b *testing.B) {
	e := echo.New()
	
	largeContent := strings.Repeat("Hello, World! This is a test content for compression. ", 1000)
	
	e.Use(compression.Gzip())
	
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}

func BenchmarkBrotliCompression(b *testing.B) {
	e := echo.New()
	
	largeContent := strings.Repeat("Hello, World! This is a test content for compression. ", 1000)
	
	e.Use(compression.Brotli())
	
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, largeContent)
	})
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}