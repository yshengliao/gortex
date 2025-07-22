package compression

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CompressionLevel represents compression levels
type CompressionLevel int

const (
	// CompressionLevelDefault is the default compression level
	CompressionLevelDefault CompressionLevel = iota
	// CompressionLevelBestSpeed prioritizes speed over compression ratio
	CompressionLevelBestSpeed
	// CompressionLevelBestCompression prioritizes compression ratio over speed
	CompressionLevelBestCompression
	// CompressionLevelNoCompression disables compression
	CompressionLevelNoCompression
)

// Config defines compression middleware configuration
type Config struct {
	// Level sets the compression level
	Level CompressionLevel
	
	// MinSize is the minimum size in bytes before compression is applied
	MinSize int
	
	// Skipper defines a function to skip compression
	Skipper middleware.Skipper
	
	// EnableBrotli enables Brotli compression support
	EnableBrotli bool
	
	// PreferBrotli prefers Brotli over gzip when both are accepted
	PreferBrotli bool
	
	// ContentTypes defines which content types to compress
	ContentTypes []string
}

// DefaultConfig returns default compression configuration
func DefaultConfig() Config {
	return Config{
		Level:        CompressionLevelDefault,
		MinSize:      1024, // 1KB
		EnableBrotli: true,
		PreferBrotli: true,
		Skipper:      middleware.DefaultSkipper,
		ContentTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/javascript",
			"application/json",
			"application/xml",
			"application/rss+xml",
			"application/atom+xml",
			"application/x-javascript",
			"application/x-font-ttf",
			"application/vnd.ms-fontobject",
			"image/svg+xml",
			"image/x-icon",
		},
	}
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del(echo.HeaderContentLength)
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get(echo.HeaderContentType) == "" {
		w.Header().Set(echo.HeaderContentType, http.DetectContentType(b))
	}
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	w.Writer.(*gzip.Writer).Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

type brotliResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *brotliResponseWriter) WriteHeader(code int) {
	w.Header().Del(echo.HeaderContentLength)
	w.ResponseWriter.WriteHeader(code)
}

func (w *brotliResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get(echo.HeaderContentType) == "" {
		w.Header().Set(echo.HeaderContentType, http.DetectContentType(b))
	}
	return w.Writer.Write(b)
}

func (w *brotliResponseWriter) Flush() {
	w.Writer.(*brotli.Writer).Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *brotliResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// Middleware returns a compression middleware with config
func Middleware(config Config) echo.MiddlewareFunc {
	// Set defaults if not provided
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}
	
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}
			
			// Check Accept-Encoding header
			acceptEncoding := c.Request().Header.Get("Accept-Encoding")
			if acceptEncoding == "" {
				return next(c)
			}
			
			res := c.Response()
			
			// Determine which encoding to use
			var encoding string
			
			if config.EnableBrotli && strings.Contains(acceptEncoding, "br") {
				if !strings.Contains(acceptEncoding, "gzip") || config.PreferBrotli {
					encoding = "br"
				}
			}
			
			if encoding == "" && strings.Contains(acceptEncoding, "gzip") {
				encoding = "gzip"
			}
			
			if encoding == "" {
				return next(c)
			}
			
			res.Header().Add(echo.HeaderVary, echo.HeaderAcceptEncoding)
			
			switch encoding {
			case "gzip":
				res.Header().Set(echo.HeaderContentEncoding, "gzip")
				
				level := gzip.DefaultCompression
				switch config.Level {
				case CompressionLevelBestSpeed:
					level = gzip.BestSpeed
				case CompressionLevelBestCompression:
					level = gzip.BestCompression
				}
				
				gz, err := gzip.NewWriterLevel(res.Writer, level)
				if err != nil {
					return err
				}
				defer gz.Close()
				
				grw := &gzipResponseWriter{Writer: gz, ResponseWriter: res.Writer}
				res.Writer = grw
				
			case "br":
				res.Header().Set(echo.HeaderContentEncoding, "br")
				
				level := brotli.DefaultCompression
				switch config.Level {
				case CompressionLevelBestSpeed:
					level = brotli.BestSpeed
				case CompressionLevelBestCompression:
					level = brotli.BestCompression
				}
				
				br := brotli.NewWriterLevel(res.Writer, level)
				defer br.Close()
				
				brw := &brotliResponseWriter{Writer: br, ResponseWriter: res.Writer}
				res.Writer = brw
			}
			
			return next(c)
		}
	}
}

// GzipWithConfig returns a gzip compression middleware with config
func GzipWithConfig(config Config) echo.MiddlewareFunc {
	// Disable brotli for gzip-only middleware
	config.EnableBrotli = false
	return Middleware(config)
}

// Gzip returns a gzip compression middleware with default config
func Gzip() echo.MiddlewareFunc {
	config := DefaultConfig()
	config.EnableBrotli = false
	return Middleware(config)
}

// BrotliWithConfig returns a brotli compression middleware with config
func BrotliWithConfig(config Config) echo.MiddlewareFunc {
	// Force brotli only
	config.EnableBrotli = true
	config.PreferBrotli = true
	return Middleware(config)
}

// Brotli returns a brotli compression middleware with default config
func Brotli() echo.MiddlewareFunc {
	config := DefaultConfig()
	return BrotliWithConfig(config)
}