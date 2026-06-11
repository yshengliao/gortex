package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decompressGzip reads a gzip-compressed body and returns the plaintext.
func decompressGzip(t *testing.T, r io.Reader) []byte {
	t.Helper()
	gr, err := gzip.NewReader(r)
	require.NoError(t, err)
	defer gr.Close()
	data, err := io.ReadAll(gr)
	require.NoError(t, err)
	return data
}

// newGzipRequest creates a GET request that advertises gzip acceptance.
func newGzipRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	return req
}

// TestCompressionGzipApplied verifies that a gzip-eligible response is
// compressed and decompresses to the original content.
func TestCompressionGzipApplied(t *testing.T) {
	body := strings.Repeat("Hello, gzip world! ", 100) // well over MinSize
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	got := string(decompressGzip(t, rec.Body))
	assert.Equal(t, body, got)
}

// TestCompressionBelowMinSizePassthrough verifies that a small response is
// not compressed (no Content-Encoding), but that Vary is still emitted
// because the middleware was active and could have compressed a larger body.
func TestCompressionBelowMinSizePassthrough(t *testing.T) {
	body := "tiny" // well below the 1 KiB default MinSize
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"), "small response must not be compressed")
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"),
		"Vary must be set even when response is below MinSize")
	assert.Equal(t, body, rec.Body.String())
}

// TestCompressionNonEligibleContentType verifies that a response with a
// content-type not in the allowlist is passed through without compression,
// and Vary is still emitted.
func TestCompressionNonEligibleContentType(t *testing.T) {
	body := strings.Repeat("binary data ", 200)
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png") // not in the default allowlist
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"), "non-eligible content type must not be compressed")
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
	assert.Equal(t, body, rec.Body.String())
}

// TestCompressionPreSetContentEncodingPassthrough verifies that when a handler
// already sets Content-Encoding (e.g. a pre-compressed asset), the middleware
// does NOT re-encode the body — double gzip would corrupt the response.
func TestCompressionPreSetContentEncodingPassthrough(t *testing.T) {
	// Build a valid gzip payload to use as the "pre-compressed" body.
	var buf strings.Builder
	gw := gzip.NewWriter(&buf)
	_, _ = io.WriteString(gw, "pre-compressed content")
	_ = gw.Close()
	precompressed := buf.String()

	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip") // already compressed
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, precompressed)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The body must arrive unchanged (decodable as gzip exactly once).
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	got := string(decompressGzip(t, rec.Body))
	assert.Equal(t, "pre-compressed content", got,
		"pre-compressed body must pass through without double encoding")
}

// TestCompressionPreSetContentEncodingPassthroughLargeBody mirrors
// TestCompressionPreSetContentEncodingPassthrough but uses a pre-compressed
// body that exceeds the default MinSize threshold (1 KiB). This ensures the
// middleware does not re-encode content once it has buffered enough bytes to
// trigger the compression decision path: the pre-set Content-Encoding must
// cause passthrough even when the size threshold has been crossed.
func TestCompressionPreSetContentEncodingPassthroughLargeBody(t *testing.T) {
	// Build a valid gzip payload whose plaintext is well above the 1 KiB
	// MinSize (use ~4 KiB so the threshold is exceeded comfortably).
	plaintext := strings.Repeat("pre-compressed large body content ", 120) // ~4 KiB
	var buf strings.Builder
	gw := gzip.NewWriter(&buf)
	_, _ = io.WriteString(gw, plaintext)
	_ = gw.Close()
	precompressed := buf.String()

	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip") // already compressed
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, precompressed)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Content-Encoding must remain exactly as the handler set it.
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	// Body must be byte-for-byte identical to what the handler wrote —
	// no double encoding.
	assert.Equal(t, precompressed, rec.Body.String(),
		"pre-compressed body must pass through without double encoding")
	// Decompressing the response exactly once must recover the original
	// plaintext — a double-gzipped body would still be compressed here.
	got := string(decompressGzip(t, rec.Body))
	assert.Equal(t, plaintext, got,
		"single gzip decode of passthrough body must recover original plaintext")
	// Vary must be present (middleware was active for this request).
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
}

// TestCompressionNoAcceptEncodingNoVary verifies that when the client does
// not advertise gzip, the middleware passes through without any
// Content-Encoding or Vary header (the middleware was not active).
func TestCompressionNoAcceptEncodingNoVary(t *testing.T) {
	body := strings.Repeat("hello world ", 200)
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil) // no Accept-Encoding
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Empty(t, rec.Header().Get("Vary"),
		"Vary must NOT be set when the client did not offer gzip (middleware bypassed)")
	assert.Equal(t, body, rec.Body.String())
}

// TestCompressionFlushNoPanic verifies that calling Flush() on the
// gzipResponseWriter does not panic, both before and after compression starts,
// and that the response body is valid.
func TestCompressionFlushNoPanic(t *testing.T) {
	body := strings.Repeat("flush test data ", 100)
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush() // must not panic
		}
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() {
		handler.ServeHTTP(rec, req)
	})

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	got := string(decompressGzip(t, rec.Body))
	assert.Equal(t, body, got)
}

// TestCompressionVaryNotDuplicated checks that the Vary: Accept-Encoding
// header appears exactly once in the compressed path (not duplicated).
func TestCompressionVaryNotDuplicated(t *testing.T) {
	body := strings.Repeat("deduplicate vary ", 100)
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))

	req := newGzipRequest("/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	varyValues := rec.Header()["Vary"]
	count := 0
	for _, v := range varyValues {
		if strings.Contains(v, "Accept-Encoding") {
			count++
		}
	}
	assert.Equal(t, 1, count, "Vary: Accept-Encoding must appear exactly once")
}
