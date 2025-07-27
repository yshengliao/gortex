package http

import (
	"bufio"
	"net"
	"net/http"
)

// responseWriter wraps http.ResponseWriter to track response status and size
type responseWriter struct {
	http.ResponseWriter
	status  int
	size    int64
	written bool
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

// Status returns the HTTP status code
func (w *responseWriter) Status() int {
	return w.status
}

// Size returns the size of the response body
func (w *responseWriter) Size() int64 {
	return w.size
}

// Written returns whether the response has been written
func (w *responseWriter) Written() bool {
	return w.written
}

// WriteHeader writes the status code
func (w *responseWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

// Write writes the data to the connection
func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

// Flush implements http.Flusher
func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Before is a no-op for basic implementation
func (w *responseWriter) Before(func()) {}

// After is a no-op for basic implementation
func (w *responseWriter) After(func()) {}