package middleware

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// CompressionConfig configures the gzip compression wrapper. The wrapper
// buffers writes until enough data has accumulated to apply the content
// filter and decide whether to stream compressed output.
type CompressionConfig struct {
	// Level is the gzip compression level (see gzip.DefaultCompression
	// etc.). Zero is treated as gzip.DefaultCompression.
	Level int
	// MinSize is the minimum response body size in bytes before
	// compression kicks in. Responses smaller than MinSize are written
	// through uncompressed. Zero disables the threshold (compress
	// everything).
	MinSize int
	// ContentTypes, when non-empty, restricts compression to responses
	// whose Content-Type matches one of the listed prefixes
	// (case-insensitive, with any "; charset=..." suffix stripped).
	ContentTypes []string
}

// DefaultCompressionConfig returns safe defaults: gzip default level, 1
// KiB threshold and a broad allowlist of text-like content types.
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Level:   gzip.DefaultCompression,
		MinSize: 1024,
		ContentTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/javascript",
			"application/json",
			"application/xml",
			"image/svg+xml",
		},
	}
}

// GzipHandler wraps next with a response compressor that activates when
// the client advertises Accept-Encoding: gzip and the response matches
// the configured content-type allowlist and size threshold.
func GzipHandler(next http.Handler) http.Handler {
	return GzipHandlerWithConfig(DefaultCompressionConfig(), next)
}

// GzipHandlerWithConfig wraps next using the supplied configuration.
func GzipHandlerWithConfig(config *CompressionConfig, next http.Handler) http.Handler {
	if config == nil {
		config = DefaultCompressionConfig()
	}
	level := config.Level
	if level == 0 {
		level = gzip.DefaultCompression
	}
	pool := &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, level)
			return w
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !clientAcceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}

		gz := pool.Get().(*gzip.Writer)
		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			gz:             gz,
			minSize:        config.MinSize,
			contentTypes:   config.ContentTypes,
			status:         http.StatusOK,
		}
		defer func() {
			gzw.close()
			gz.Reset(io.Discard)
			pool.Put(gz)
		}()

		next.ServeHTTP(gzw, r)
	})
}

func clientAcceptsGzip(header string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		token := strings.TrimSpace(part)
		if idx := strings.IndexByte(token, ';'); idx >= 0 {
			token = token[:idx]
		}
		if strings.EqualFold(token, "gzip") {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz           *gzip.Writer
	minSize      int
	contentTypes []string

	buf         []byte
	wroteHeader bool
	status      int
	compressing bool
	passthrough bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	// Defer the real WriteHeader until Write() so we can decide whether
	// to compress based on Content-Type and accumulated size.
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.passthrough {
		return w.ResponseWriter.Write(p)
	}
	if w.compressing {
		return w.gz.Write(p)
	}

	w.buf = append(w.buf, p...)
	if len(w.buf) < w.minSize {
		return len(p), nil
	}

	if !w.shouldCompress() {
		w.passthrough = true
		w.ResponseWriter.WriteHeader(w.status)
		if _, err := w.ResponseWriter.Write(w.buf); err != nil {
			w.buf = nil
			return 0, err
		}
		w.buf = nil
		return len(p), nil
	}

	w.startCompressed()
	if _, err := w.gz.Write(w.buf); err != nil {
		w.buf = nil
		return 0, err
	}
	w.buf = nil
	return len(p), nil
}

func (w *gzipResponseWriter) shouldCompress() bool {
	if len(w.contentTypes) == 0 {
		return true
	}
	ct := w.Header().Get("Content-Type")
	if ct == "" {
		return false
	}
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	for _, allowed := range w.contentTypes {
		if strings.HasPrefix(ct, strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

func (w *gzipResponseWriter) startCompressed() {
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	h.Add("Vary", "Accept-Encoding")
	w.gz.Reset(w.ResponseWriter)
	w.ResponseWriter.WriteHeader(w.status)
	w.compressing = true
}

func (w *gzipResponseWriter) close() {
	if !w.wroteHeader {
		return
	}
	if w.compressing {
		_ = w.gz.Close()
		return
	}
	if w.passthrough {
		return
	}
	// Buffered body shorter than MinSize: flush as-is without
	// Content-Encoding.
	w.ResponseWriter.WriteHeader(w.status)
	if len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
}

func (w *gzipResponseWriter) Flush() {
	if w.compressing {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack is forwarded when the underlying writer supports it. Hijacking
// is incompatible with active compression, so it is refused once we've
// begun streaming compressed output.
func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.compressing {
		return nil, nil, errors.New("gzip response writer: cannot hijack after compression started")
	}
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
