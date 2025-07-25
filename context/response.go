package context

import (
	"bufio"
	"net"
	"net/http"
)

// compile time check
var _ ResponseWriter = (*response)(nil)

// response implements ResponseWriter interface
type response struct {
	http.ResponseWriter
	status      int
	size        int64
	written     bool
	beforeFuncs []func()
	afterFuncs  []func()
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &response{
		ResponseWriter: w,
		status:         http.StatusOK,
		beforeFuncs:    make([]func(), 0),
		afterFuncs:     make([]func(), 0),
	}
}

// WriteHeader sends an HTTP response header with the provided status code
func (r *response) WriteHeader(code int) {
	if r.written {
		return
	}
	
	// Execute before functions
	for _, fn := range r.beforeFuncs {
		fn()
	}
	
	r.status = code
	r.written = true
	r.ResponseWriter.WriteHeader(code)
}

// Write writes the data to the connection as part of an HTTP reply
func (r *response) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	
	n, err := r.ResponseWriter.Write(b)
	r.size += int64(n)
	
	// Execute after functions
	for _, fn := range r.afterFuncs {
		fn()
	}
	
	return n, err
}

// Status returns the status code
func (r *response) Status() int {
	return r.status
}

// Size returns the response size
func (r *response) Size() int64 {
	return r.size
}

// Written returns true if response was written
func (r *response) Written() bool {
	return r.written
}

// Before adds a function to be called before writing
func (r *response) Before(fn func()) {
	r.beforeFuncs = append(r.beforeFuncs, fn)
}

// After adds a function to be called after writing
func (r *response) After(fn func()) {
	r.afterFuncs = append(r.afterFuncs, fn)
}

// Hijack implements the http.Hijacker interface
func (r *response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, ErrNotFound
	}
	return hijacker.Hijack()
}

// Flush implements the http.Flusher interface
func (r *response) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push implements the http.Pusher interface
func (r *response) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := r.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return ErrNotFound
}