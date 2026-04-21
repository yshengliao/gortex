package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appcontext "github.com/yshengliao/gortex/core/context"
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// --- utils.go ----------------------------------------------------------

func TestCamelToKebab(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"Hello":         "hello",
		"HelloWorld":    "hello-world",
		"ABC":           "a-b-c",
		"getUserByID":   "get-user-by-i-d",
		"doStuff":       "do-stuff",
		"HTTPServer":    "h-t-t-p-server",
	}
	for input, want := range cases {
		assert.Equal(t, want, camelToKebab(input), "input=%q", input)
	}
}

func TestMethodNameToPath(t *testing.T) {
	assert.Equal(t, "/list-users", methodNameToPath("ListUsers"))
	assert.Equal(t, "/", methodNameToPath(""))
	assert.Equal(t, "/get", methodNameToPath("Get"))
}

func TestExtractMiddlewareNames(t *testing.T) {
	noop := func(next middleware.HandlerFunc) middleware.HandlerFunc { return next }
	got := extractMiddlewareNames([]middleware.MiddlewareFunc{noop, nil, noop})
	// nil middleware is skipped; the two non-nil entries produce
	// generic placeholder names derived from their slice index.
	assert.Equal(t, []string{"middleware_0", "middleware_2"}, got)
	assert.Empty(t, extractMiddlewareNames(nil))
}

func TestContainsHelper(t *testing.T) {
	assert.True(t, contains([]string{"a", "b", "c"}, "b"))
	assert.False(t, contains([]string{"a", "b"}, "z"))
	assert.False(t, contains(nil, "x"))
}

// isValidGortexHandler only accepts the (ctx) error signature.
type validTarget struct{}

func (validTarget) GET(c httpctx.Context) error  { return nil }
func (validTarget) Wrong(s string) error         { return nil }
func (validTarget) NoReturn(c httpctx.Context)   {}
func (validTarget) TwoReturn(c httpctx.Context) (int, error) {
	return 0, nil
}

func TestIsValidGortexHandler(t *testing.T) {
	t_ := reflect.TypeOf(validTarget{})

	ok, _ := t_.MethodByName("GET")
	assert.True(t, isValidGortexHandler(ok))

	wrong, _ := t_.MethodByName("Wrong")
	assert.False(t, isValidGortexHandler(wrong), "non-Context arg must be rejected")

	noret, _ := t_.MethodByName("NoReturn")
	assert.False(t, isValidGortexHandler(noret), "missing error return must be rejected")

	tworet, _ := t_.MethodByName("TwoReturn")
	assert.False(t, isValidGortexHandler(tworet), "two returns must be rejected")
}

// --- route_cache.go ----------------------------------------------------

type cacheTestHandler struct{}

func (cacheTestHandler) GET(c httpctx.Context) error            { return nil }
func (cacheTestHandler) POST(c httpctx.Context) error           { return nil }
func (cacheTestHandler) CreateWidget(c httpctx.Context) error   { return nil }
func (cacheTestHandler) UpdateWidget(c httpctx.Context) error   { return nil }
func (cacheTestHandler) DeleteWidget(c httpctx.Context) error   { return nil }
func (cacheTestHandler) ListWidgets(c httpctx.Context) error    { return nil }
func (cacheTestHandler) notExported(c httpctx.Context) error    { return nil } //nolint:unused

func TestHandlerCacheBuildsStandardAndCustomMethods(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)

	ty := reflect.TypeOf(cacheTestHandler{})
	methods := handlerCache.GetHandlerMethods(ty)

	require.Contains(t, methods, "GET")
	assert.Equal(t, "GET", methods["GET"].HTTPMethod)
	require.Contains(t, methods, "POST")
	assert.Equal(t, "POST", methods["POST"].HTTPMethod)

	// Custom methods get their HTTP verb inferred from the method-name prefix.
	require.Contains(t, methods, "CreateWidget")
	assert.Equal(t, "POST", methods["CreateWidget"].HTTPMethod)
	require.Contains(t, methods, "UpdateWidget")
	assert.Equal(t, "PUT", methods["UpdateWidget"].HTTPMethod)
	require.Contains(t, methods, "DeleteWidget")
	assert.Equal(t, "DELETE", methods["DeleteWidget"].HTTPMethod)

	// Unknown prefix falls back to GET.
	require.Contains(t, methods, "ListWidgets")
	assert.Equal(t, "GET", methods["ListWidgets"].HTTPMethod)
	assert.Equal(t, "/list-widgets", methods["ListWidgets"].Path)
}

func TestHandlerCacheReturnsCachedCopy(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)

	ty := reflect.TypeOf(cacheTestHandler{})
	first := handlerCache.GetHandlerMethods(ty)
	second := handlerCache.GetHandlerMethods(ty)

	// Both calls return the same map — the second hits the cache.
	assert.Equal(t, len(first), len(second))
	// Go maps compare by reference; if the cache hit returns the same
	// instance reflect.ValueOf gives equal pointers.
	assert.Equal(t,
		reflect.ValueOf(first).Pointer(),
		reflect.ValueOf(second).Pointer(),
		"second lookup should hit the cache, not rebuild")
}

func TestRouteCacheGetSet(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)

	routes := []RouteInfo{{Method: "GET", Path: "/x"}}
	routeCache.SetRoutes("key", routes)

	got, ok := routeCache.GetRoutes("key")
	require.True(t, ok)
	assert.Equal(t, routes, got)

	_, ok = routeCache.GetRoutes("missing")
	assert.False(t, ok)

	ClearCache()
	_, ok = routeCache.GetRoutes("key")
	assert.False(t, ok, "ClearCache must drop stored routes")
}

// --- route_registration.go: parseMiddleware & parseRateLimit -----------

func TestParseMiddlewareBuiltins(t *testing.T) {
	ctx := appcontext.NewContext()

	mws := parseMiddleware("requestid,recover", ctx)
	require.Len(t, mws, 2, "requestid + recover both resolve via the builtin switch")

	// Builtin recover wraps the handler and converts panics into 500s.
	recover := mws[1]
	wrapped := recover(func(c httpctx.Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	routerCtx := httpctx.NewDefaultContext(req, rec)
	assert.NotPanics(t, func() { _ = wrapped(routerCtx) })
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestParseMiddlewareEmptyAndUnknown(t *testing.T) {
	ctx := appcontext.NewContext()
	// Empty tag & bare commas resolve to zero middleware.
	assert.Empty(t, parseMiddleware("", ctx))
	assert.Empty(t, parseMiddleware(",,", ctx))
	// Unknown name is silently dropped (rbac without config just logs).
	assert.Empty(t, parseMiddleware("does-not-exist", ctx))
}

func TestParseMiddlewareRegistry(t *testing.T) {
	ctx := appcontext.NewContext()

	seen := false
	registry := map[string]middleware.MiddlewareFunc{
		"custom": func(next middleware.HandlerFunc) middleware.HandlerFunc {
			return func(c httpctx.Context) error {
				seen = true
				return next(c)
			}
		},
	}
	appcontext.Register(ctx, registry)

	mws := parseMiddleware("custom", ctx)
	require.Len(t, mws, 1)

	// Invoke the middleware to prove it's the one we registered.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	routerCtx := httpctx.NewDefaultContext(req, rec)
	_ = mws[0](func(c httpctx.Context) error { return nil })(routerCtx)
	assert.True(t, seen, "registry-provided middleware must be invoked")
}

func TestParseRateLimitSeconds(t *testing.T) {
	ctx := appcontext.NewContext()
	mw := parseRateLimit("5/sec", ctx)
	require.NotNil(t, mw)

	// Fire five requests from a distinct remote — all should pass; the
	// sixth in the same second should get 429.
	handler := mw(func(c httpctx.Context) error {
		return c.NoContent(http.StatusOK)
	})

	var last int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/rl", nil)
		req.RemoteAddr = "203.0.113.10:12345" // outside the local skip list
		rec := httptest.NewRecorder()
		c := httpctx.NewDefaultContext(req, rec)
		_ = handler(c)
		last = rec.Code
	}
	assert.Equal(t, http.StatusTooManyRequests, last)
}

func TestParseRateLimitMinutesAndHours(t *testing.T) {
	ctx := appcontext.NewContext()

	// "/min" and "/hour" compute a per-second burst of at least 1.
	assert.NotNil(t, parseRateLimit("100/min", ctx))
	assert.NotNil(t, parseRateLimit("100/hour", ctx))
}

func TestParseRateLimitRejectsBadInput(t *testing.T) {
	ctx := appcontext.NewContext()
	// No slash, non-numeric count, unknown unit all surface as nil.
	assert.Nil(t, parseRateLimit("100", ctx))
	assert.Nil(t, parseRateLimit("abc/sec", ctx))
	assert.Nil(t, parseRateLimit("10/fortnight", ctx))
}

// --- route_registration.go: isHandlerGroup -----------------------------

type hgLeaf struct{}

func (hgLeaf) GET(c httpctx.Context) error { return nil }

type hgGroup struct {
	Child *hgLeaf `url:"/child"`
}

func TestIsHandlerGroup(t *testing.T) {
	// Leaf handler with only method receivers is not a group.
	assert.False(t, isHandlerGroup(&hgLeaf{}))

	// Group with a URL-tagged child pointer is a group.
	assert.True(t, isHandlerGroup(&hgGroup{Child: &hgLeaf{}}))

	// Non-pointer argument is rejected outright.
	assert.False(t, isHandlerGroup(hgGroup{}))
	assert.False(t, isHandlerGroup("not a struct"))
}

// --- route_registration.go: getAvailableTypes stub --------------------

func TestGetAvailableTypesStub(t *testing.T) {
	ctx := appcontext.NewContext()
	// Implementation is a TODO; just make sure it returns a non-nil
	// (possibly empty) slice without panicking.
	got := getAvailableTypes(ctx)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// --- app.go: WithRuntimeMode, WithRoutesLogger, Config, compressionEnabled

func TestWithRuntimeModeAndRoutesLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	a, err := NewApp(
		WithLogger(logger),
		WithRuntimeMode(ModeGortex),
		WithRoutesLogger(),
	)
	require.NoError(t, err)
	assert.Equal(t, ModeGortex, a.runtimeMode)
	assert.True(t, a.enableRoutesLog)
}

func TestAppCompressionEnabledLegacyAndModern(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// No config → compression disabled.
	a, err := NewApp(WithLogger(logger))
	require.NoError(t, err)
	assert.False(t, a.compressionEnabled())

	// Legacy flag.
	cfg := &Config{}
	cfg.Server.GZip = true
	a, err = NewApp(WithConfig(cfg), WithLogger(logger))
	require.NoError(t, err)
	assert.True(t, a.compressionEnabled())

	// Modern flag.
	cfg2 := &Config{}
	cfg2.Server.Compression.Enabled = true
	a, err = NewApp(WithConfig(cfg2), WithLogger(logger))
	require.NoError(t, err)
	assert.True(t, a.compressionEnabled())
}

// --- app.go: Run + Shutdown end-to-end ---------------------------------

func TestAppRunAndShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{}
	cfg.Server.Address = "127.0.0.1:0" // let the OS pick a port for Listen

	a, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
		WithShutdownTimeout(2*time.Second),
	)
	require.NoError(t, err)

	// Listen manually so we know which port was chosen and can wait for
	// the server to come up without timing-based polling.
	ln, err := net.Listen("tcp", cfg.Server.Address)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	a.server = &http.Server{Handler: a.serverHandler()}
	serveErr := make(chan error, 1)
	go func() { serveErr <- a.server.Serve(ln) }()

	// Now Shutdown should gracefully stop the server.
	err = a.Shutdown(t.Context())
	require.NoError(t, err)

	// Serve returns http.ErrServerClosed on clean shutdown.
	select {
	case e := <-serveErr:
		assert.True(t, strings.Contains(e.Error(), "closed"), "got %v", e)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after Shutdown")
	}
}

func TestAppShutdownWithoutServerIsNoop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	a, err := NewApp(WithLogger(logger))
	require.NoError(t, err)
	// No Run() called — Shutdown must still succeed without panicking.
	assert.NoError(t, a.Shutdown(t.Context()))
}
