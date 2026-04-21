package app_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/core/app"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

type PanicHandler struct{}

func (h *PanicHandler) GET(c httpctx.Context) error {
	panic("boom")
}

type BigJSONHandler struct{}

func (h *BigJSONHandler) GET(c httpctx.Context) error {
	// Build a response body well above the compression threshold so the
	// gzip wrapper actually kicks in.
	return c.JSON(http.StatusOK, map[string]string{
		"payload": strings.Repeat("abcdefghij", 2048), // 20 KiB
	})
}

type NewAppHandlers struct {
	Panic *PanicHandler   `url:"/panic"`
	Big   *BigJSONHandler `url:"/big"`
	Err   *ErrorHandler   `url:"/err"`
}

func newAppWithHandlers(t *testing.T, cfg *app.Config) *app.App {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	a, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(&NewAppHandlers{
			Panic: &PanicHandler{},
			Big:   &BigJSONHandler{},
			Err:   &ErrorHandler{},
		}),
	)
	require.NoError(t, err)
	return a
}

func TestRecoveryMiddlewareCatchesPanic(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = false
	a := newAppWithHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code,
		"recovery middleware must translate the panic into a 500")
	assert.Contains(t, rec.Body.String(), "PANIC")
}

func TestRecoveryCanBeDisabled(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = false
	cfg.Server.CORS = false
	a := newAppWithHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	assert.Panics(t, func() {
		a.Router().ServeHTTP(rec, req)
	}, "with Recovery=false the panic must propagate")
}

func TestErrorHandlerTranslatesReturnedErrors(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = false
	a := newAppWithHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)

	// ErrorHandler converts HTTPError.StatusCode() → response status.
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Contains(t, rec.Body.String(), "I'm a teapot")
}

func TestCORSPreflightResponds(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = true
	a := newAppWithHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/big", nil)
	req.Header.Set("Origin", "https://example.test")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	// CORS runs at http.Handler scope so it can answer preflight even
	// when no OPTIONS route is registered.
	a.ServerHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodGet)
}

func TestCORSCanBeDisabled(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = false
	a := newAppWithHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/big", nil)
	req.Header.Set("Origin", "https://example.test")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	a.ServerHandler().ServeHTTP(rec, req)

	// With CORS off and no OPTIONS route registered the preflight
	// falls through to the router's default 404.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestGzipHandlerCompressesLargeResponses(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = false
	cfg.Server.GZip = true
	cfg.Server.Compression.Enabled = true
	cfg.Server.Compression.MinSize = 512
	cfg.Server.Compression.ContentTypes = []string{"application/json"}
	a := newAppWithHandlers(t, cfg)

	// Run a real HTTP server so the gzip wrapper (installed at
	// http.Server construction) is in the chain.
	srv := httptest.NewServer(testHandler(a))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/big", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	// Use a plain client that does not transparently decompress.
	client := &http.Client{
		Transport: &http.Transport{DisableCompression: true},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
	assert.Contains(t, resp.Header.Get("Vary"), "Accept-Encoding")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	zr, err := gzip.NewReader(bytes.NewReader(raw))
	require.NoError(t, err, "body must be valid gzip")
	defer zr.Close()
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Contains(t, string(body), "abcdefghij")
}

func TestGzipHandlerSkippedWithoutAcceptEncoding(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Server.CORS = false
	cfg.Server.GZip = true
	cfg.Server.Compression.Enabled = true
	cfg.Server.Compression.MinSize = 512
	cfg.Server.Compression.ContentTypes = []string{"application/json"}
	a := newAppWithHandlers(t, cfg)

	srv := httptest.NewServer(testHandler(a))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/big", nil)
	require.NoError(t, err)
	// Explicitly refuse gzip.
	req.Header.Set("Accept-Encoding", "identity")

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, resp.Header.Get("Content-Encoding"),
		"no gzip when the client did not advertise support")
}

// testHandler mirrors what App.Run installs on http.Server.Handler —
// the router possibly wrapped with the gzip handler when the config
// enables compression.
func testHandler(a *app.App) http.Handler {
	return a.ServerHandler()
}
