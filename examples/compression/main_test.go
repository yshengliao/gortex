package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"go.uber.org/zap"
)

func TestCompressionExample(t *testing.T) {
	// Initialize logger
	logger, _ := zap.NewDevelopment()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":0" // Random port
	cfg.Logger.Level = "debug"
	cfg.Server.Compression.Enabled = true
	cfg.Server.Compression.Level = "default"
	cfg.Server.Compression.MinSize = 100
	cfg.Server.Compression.EnableBrotli = true
	cfg.Server.Compression.PreferBrotli = true

	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandlers{
			Logger: logger,
		},
		Files: &FileHandlers{
			Logger: logger,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)

	e := application.Echo()

	// Test gzip compression
	t.Run("GzipCompression", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/large", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		// Decompress and verify it's valid JSON
		reader, err := gzip.NewReader(rec.Body)
		require.NoError(t, err)
		defer reader.Close()

		var buf bytes.Buffer
		_, err = io.Copy(&buf, reader)
		require.NoError(t, err)

		// Should be large content
		assert.Greater(t, buf.Len(), 10000)
	})

	// Test brotli compression
	t.Run("BrotliCompression", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/large", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))

		// Decompress and verify
		reader := brotli.NewReader(rec.Body)
		var buf bytes.Buffer
		_, err := io.Copy(&buf, reader)
		require.NoError(t, err)

		// Should be large content
		assert.Greater(t, buf.Len(), 10000)
	})

	// Test no compression when not requested
	t.Run("NoCompression", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/large", nil)
		// No Accept-Encoding header
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
	})

	// Test different content types
	t.Run("ContentTypes", func(t *testing.T) {
		tests := []struct {
			path     string
			method   string
			encoding string
		}{
			{"/api/text", http.MethodPost, "gzip"},
			{"/files/css", http.MethodPost, "br"},
			{"/files/javascript", http.MethodPost, "gzip"},
			{"/files/xml", http.MethodPost, "br"},
		}

		for _, tt := range tests {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Accept-Encoding", tt.encoding)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.encoding, rec.Header().Get("Content-Encoding"))
		}
	})
}

func BenchmarkLargeJSONCompression(b *testing.B) {
	logger, _ := zap.NewProduction()

	cfg := &app.Config{}
	cfg.Server.Compression.Enabled = true
	cfg.Server.Compression.EnableBrotli = true

	handlers := &HandlersManager{
		API: &APIHandlers{Logger: logger},
	}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)

	e := application.Echo()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/large", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}