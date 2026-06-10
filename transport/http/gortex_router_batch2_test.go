package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ghttp "github.com/yshengliao/gortex/transport/http"
)

// --- Fix 2: bare-router error path must not double-write the response ---

// TestServeHTTP_HandlerWritesThenErrors verifies that when a handler writes a
// successful response and then returns an error, ServeHTTP does NOT emit a
// second status line or append an error body on top of the real response.
func TestServeHTTP_HandlerWritesThenErrors(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/", func(c ghttp.Context) error {
		if err := c.JSON(http.StatusOK, map[string]string{"ok": "yes"}); err != nil {
			return err
		}
		// Handler already committed a 200 response, then fails.
		return ghttp.NewHTTPError(http.StatusInternalServerError, "boom")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 (handler's write), got %d", rec.Code)
	}
	body := rec.Body.String()
	// The error message must NOT have been appended.
	if strings.Contains(body, "boom") {
		t.Errorf("error body was appended after the real response: %q", body)
	}
	// The real body must be intact and appear exactly once.
	if strings.Count(body, `"ok":"yes"`) != 1 {
		t.Errorf("expected exactly one real body, got %q", body)
	}
}

// TestServeHTTP_HTTPErrorWithoutWrite verifies the existing behavior: a handler
// that returns an *HTTPError without writing yields a proper JSON error.
func TestServeHTTP_HTTPErrorWithoutWrite(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/", func(c ghttp.Context) error {
		return ghttp.NewHTTPError(http.StatusNotFound, "missing")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "missing") {
		t.Errorf("expected error message in body, got %q", rec.Body.String())
	}
}

// TestServeHTTP_GenericErrorWithoutWrite verifies a generic error without a
// write yields a 500.
func TestServeHTTP_GenericErrorWithoutWrite(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/", func(c ghttp.Context) error {
		return errFake("kaboom")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "kaboom") {
		t.Errorf("expected error message in body, got %q", rec.Body.String())
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

// --- Fix 3: sibling group routers must share one lock over the shared trees ---

// TestGortexRouter_ConcurrentSiblingRegistration registers routes from many
// sibling groups concurrently while another goroutine serves requests. The
// shared-tree data is guarded by a single shared lock; the assertion is simply
// that this runs clean under -race.
func TestGortexRouter_ConcurrentSiblingRegistration(t *testing.T) {
	r := ghttp.NewGortexRouter()

	// Seed a route so the serving goroutine has a tree to read immediately.
	r.GET("/seed", func(c ghttp.Context) error { return c.String(200, "seed") })

	const groups = 16

	// Serving goroutine runs until stopped; it races readers against the
	// concurrent registrations below.
	stop := make(chan struct{})
	var serverWG sync.WaitGroup
	serverWG.Add(1)
	go func() {
		defer serverWG.Done()
		req := httptest.NewRequest("GET", "/seed", nil)
		for {
			select {
			case <-stop:
				return
			default:
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
			}
		}
	}()

	// Each goroutine creates its own sibling group off the same parent and
	// registers routes on it.
	var regWG sync.WaitGroup
	for g := 0; g < groups; g++ {
		regWG.Add(1)
		go func() {
			defer regWG.Done()
			grp := r.Group("/g")
			grp.GET("/a", func(c ghttp.Context) error { return c.String(200, "a") })
			grp.GET("/b", func(c ghttp.Context) error { return c.String(200, "b") })
		}()
	}

	// Wait for all registrations, then stop and join the serving goroutine.
	regWG.Wait()
	close(stop)
	serverWG.Wait()
}

// --- Fix 4: sibling groups must each see only their own middleware chain ---

// TestGortexRouter_SiblingGroupMiddlewareIsolation verifies that two sibling
// groups created from the same parent do not share/clobber each other's
// middleware via a shared backing array.
func TestGortexRouter_SiblingGroupMiddlewareIsolation(t *testing.T) {
	r := ghttp.NewGortexRouter()

	var mu sync.Mutex
	seen := map[string][]string{}
	record := func(route, name string) {
		mu.Lock()
		seen[route] = append(seen[route], name)
		mu.Unlock()
	}

	mw := func(name string) ghttp.MiddlewareFunc {
		return func(next ghttp.HandlerFunc) ghttp.HandlerFunc {
			return func(c ghttp.Context) error {
				record(c.Path(), name)
				return next(c)
			}
		}
	}

	// Build a parent group whose middleware slice has spare capacity so that a
	// bare append() shared between two siblings would clobber. Go grows a slice
	// past a power-of-two boundary leaving slack (e.g. the 5th append yields
	// cap 8), so nesting several single-middleware groups gives the deepest
	// parent spare capacity. Two siblings created off that parent must NOT
	// share its backing array.
	parent := r.Group("/p", mw("m1"))
	parent = parent.Group("/q", mw("m2"))
	parent = parent.Group("/r", mw("m3"))
	parent = parent.Group("/s", mw("m4"))
	parent = parent.Group("/t", mw("m5"))
	base := []string{"m1", "m2", "m3", "m4", "m5"}

	// Create BOTH siblings before registering their routes. With a shared
	// backing array, creating b clobbers the middleware a recorded; building
	// a's handler afterwards would then see b's middleware. Registering after
	// both groups exist is what exposes the aliasing at the route level.
	a := parent.Group("/a", mw("a"))
	b := parent.Group("/b", mw("b"))
	a.GET("/x", func(c ghttp.Context) error { return c.String(200, "ax") })
	b.GET("/y", func(c ghttp.Context) error { return c.String(200, "by") })

	call := func(path string) {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
	}

	prefix := "/p/q/r/s/t"
	call(prefix + "/a/x")
	call(prefix + "/b/y")

	// Each leaf must see exactly its own chain: base + its own sibling mw.
	wantA := append(append([]string{}, base...), "a")
	wantB := append(append([]string{}, base...), "b")
	if got := seen[prefix+"/a/x"]; !equalStrings(got, wantA) {
		t.Errorf("%s/a/x middleware chain = %v, want %v", prefix, got, wantA)
	}
	if got := seen[prefix+"/b/y"]; !equalStrings(got, wantB) {
		t.Errorf("%s/b/y middleware chain = %v, want %v", prefix, got, wantB)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
